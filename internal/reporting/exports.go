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
			data, err = generatePDFPlaceholder(headers, rows)
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

// generatePDFPlaceholder emits a minimal, valid, text-only PDF — a real
// formatted-report PDF renderer is out of scope for this phase's
// generic export scaffolding (see the Phase 6 report's gaps); this
// produces an actually-openable PDF rather than mislabeling a CSV.
func generatePDFPlaceholder(headers []string, rows [][]string) ([]byte, error) {
	var content bytes.Buffer
	content.WriteString("Report Export\n")
	if len(headers) > 0 {
		content.WriteString(joinRow(headers) + "\n")
	}
	for _, r := range rows {
		content.WriteString(joinRow(r) + "\n")
	}
	text := content.String()

	var pdf bytes.Buffer
	pdf.WriteString("%PDF-1.4\n")
	pdf.WriteString("1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj\n")
	pdf.WriteString("2 0 obj<</Type/Pages/Kids[3 0 R]/Count 1>>endobj\n")
	pdf.WriteString("3 0 obj<</Type/Page/Parent 2 0 R/Resources<</Font<</F1 5 0 R>>>>/MediaBox[0 0 612 792]/Contents 4 0 R>>endobj\n")
	streamContent := "BT /F1 10 Tf 20 750 Td (" + pdfEscape(text) + ") Tj ET"
	pdf.WriteString("4 0 obj<</Length " + itoaInt(len(streamContent)) + ">>stream\n" + streamContent + "\nendstream endobj\n")
	pdf.WriteString("5 0 obj<</Type/Font/Subtype/Type1/BaseFont/Helvetica>>endobj\n")
	pdf.WriteString("trailer<</Root 1 0 R>>\n%%EOF")
	return pdf.Bytes(), nil
}

func joinRow(fields []string) string {
	out := ""
	for i, f := range fields {
		if i > 0 {
			out += ", "
		}
		out += f
	}
	return out
}

func pdfEscape(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '(' || c == ')' || c == '\\' {
			out = append(out, '\\')
		}
		if c == '\n' {
			continue
		}
		out = append(out, c)
	}
	return string(out)
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
