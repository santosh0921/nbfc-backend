// Exports (item L). Every export is logged (who/when/what/filters) BEFORE
// generation starts, runs asynchronously so a large report never blocks
// the request, and is stored via internal/storage's encrypted provider
// (Phase 3) — never a public URL.
package reporting

import (
	"bytes"
	"encoding/csv"
	"time"

	"github.com/google/uuid"

	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/observability"
	"github.com/santosh0921/nbfc-backend/internal/storage"
)

// RequestExport (item L) — logs the export request immediately (audit
// trail exists even if generation later fails), then generates
// asynchronously. rows/headers are the already-computed report content —
// this function's job is the security/audit/storage scaffolding around
// export, not computing report content itself (that's MIS/regulatory
// report generation, elsewhere in this package).
func RequestExport(reportType, format, parametersJSON string, exportedByAdminID uint, exportedByName string, headers []string, rows [][]string) (*models.ReportExport, error) {
	return RequestExportWithMetadata(reportType, format, parametersJSON, exportedByAdminID, exportedByName, headers, rows, ExportMetadata{
		ReportName: reportType, GeneratedBy: exportedByName, RecordCount: len(rows),
	})
}

// RequestExportWithMetadata is RequestExport plus the full item-S
// metadata block (reporting period, filters, totals) — callers that have
// this context (admin report-builder handlers) should prefer this over
// the plain RequestExport, which fills in only the minimum.
func RequestExportWithMetadata(reportType, format, parametersJSON string, exportedByAdminID uint, exportedByName string, headers []string, rows [][]string, meta ExportMetadata) (*models.ReportExport, error) {
	// Row-count cap (Phase 16 item AK) — refused BEFORE the export row
	// is created, so an over-limit request produces a clear error rather
	// than a queued job that fails later for reasons the requester has
	// to go looking for.
	if err := ValidateExportRows(len(rows)); err != nil {
		observability.Inc(observability.MetricExportRejected)
		return nil, err
	}

	// Formula-injection defense (item L) applied once, here, rather than
	// in every report builder — see csv_sanitize.go.
	headers = sanitizeRow(headers)
	rows = sanitizeRows(rows)
	meta.Totals = sanitizeTotals(meta.Totals)

	export := models.ReportExport{
		ReportType: reportType, Format: format, Parameters: parametersJSON,
		ExportedByAdminID: exportedByAdminID, ExportedByName: exportedByName,
		Status: models.ExportStatusQueued, RequestedAt: time.Now(),
	}
	if err := database.DB.Create(&export).Error; err != nil {
		return nil, err
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorID: exportedByAdminID, ActorRef: exportedByName,
		Action: "report.export.requested", TargetType: "report_export", TargetID: itoa(export.ID),
		Result: audit.ResultSuccess, Details: reportType + " (" + format + ")",
	})

	if meta.GeneratedAt == "" {
		meta.GeneratedAt = time.Now().Format(time.RFC3339)
	}
	go generateExportAsync(export.ID, format, headers, rows, meta)

	return &export, nil
}

func generateExportAsync(exportID uint, format string, headers []string, rows [][]string, meta ExportMetadata) {
	// Concurrency gate (item AK): bound how many exports may render at
	// once, so N simultaneous large exports cannot collectively exhaust
	// memory or hold every pooled database connection.
	if err := acquireExportSlot(); err != nil {
		observability.Inc(observability.MetricExportRejected)
		failExport(exportID, err.Error())
		return
	}
	defer releaseExportSlot()

	database.DB.Model(&models.ReportExport{}).Where("id = ?", exportID).Update("status", models.ExportStatusRunning)

	// Execution-time cap (item AK). Rendering is CPU-bound and not
	// context-aware, so it runs on its own goroutine and the deadline is
	// enforced by whichever finishes first. The render goroutine is not
	// forcibly killed — Go provides no safe way to do that — so this
	// bounds how long the EXPORT stays pending from the requester's
	// point of view, and frees the concurrency slot; the abandoned
	// render still completes and is discarded. That limitation is
	// recorded in the Phase 16 report rather than papered over.
	type renderResult struct {
		data []byte
		err  error
	}
	done := make(chan renderResult, 1)
	go func() {
		var data []byte
		var err error
		switch format {
		case models.ExportFormatExcel:
			data, err = BuildXLSX(meta, headers, rows)
		case models.ExportFormatCSV:
			data, err = generateCSV(headers, rows)
		case models.ExportFormatPDF:
			data, err = buildPDFReport(meta, headers, rows)
		default:
			data, err = generateCSV(headers, rows)
		}
		done <- renderResult{data, err}
	}()

	limits := CurrentExportLimits()
	var data []byte
	var err error
	select {
	case r := <-done:
		data, err = r.data, r.err
	case <-time.After(limits.MaxExecution):
		observability.Inc(observability.MetricExportFailures)
		failExport(exportID, ErrExportTimedOut.Error())
		return
	}

	if err == nil {
		// File-size cap (item AK) — checked before the artifact is
		// persisted, so an oversized file never reaches storage.
		err = ValidateExportSize(len(data))
	}
	if err != nil {
		database.DB.Model(&models.ReportExport{}).Where("id = ?", exportID).
			Updates(map[string]interface{}{"status": models.ExportStatusFailed, "failure_reason": err.Error()})
		return
	}

	now := time.Now()
	provider := storage.Active()
	if provider == nil {
		database.DB.Model(&models.ReportExport{}).Where("id = ?", exportID).
			Updates(map[string]interface{}{"status": models.ExportStatusFailed, "failure_reason": "no storage provider configured — cannot securely persist the export"})
		return
	}
	ref, err := provider.Store(uuid.Nil, uuid.New(), data)
	if err != nil {
		database.DB.Model(&models.ReportExport{}).Where("id = ?", exportID).
			Updates(map[string]interface{}{"status": models.ExportStatusFailed, "failure_reason": err.Error()})
		return
	}

	database.DB.Model(&models.ReportExport{}).Where("id = ?", exportID).Updates(map[string]interface{}{
		"status": models.ExportStatusReady, "storage_reference": ref, "completed_at": now,
	})
}

func generateCSV(headers []string, rows [][]string) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if len(headers) > 0 {
		if err := w.Write(headers); err != nil {
			return nil, err
		}
	}
	for _, r := range rows {
		if err := w.Write(r); err != nil {
			return nil, err
		}
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}

// failExport records a terminal failure reason on an export row. Single
// helper so every failure path writes the same shape.
func failExport(exportID uint, reason string) {
	database.DB.Model(&models.ReportExport{}).Where("id = ?", exportID).
		Updates(map[string]interface{}{
			"status":         models.ExportStatusFailed,
			"failure_reason": reason,
		})
}
