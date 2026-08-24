// CSV/Excel formula injection defense (Phase 14 item L).
//
// Finding: cell values reaching generateCSV and BuildXLSX came straight
// from report builders — some of it customer/vendor/free-text data
// (expense descriptions, KYC-adjacent fields, admin notes) — with no
// check for a leading formula trigger character. A value like
// `=HYPERLINK("http://attacker/",\"click\")` or `=cmd|'/c calc'!A1`
// stored as, say, an expense description and then exported would be
// evaluated as a live formula by Excel/Sheets/LibreOffice on open,
// regardless of file format (CSV has no cell-type information at all,
// and some spreadsheet applications re-evaluate inline-string XLSX cells
// leading with these characters too — this sanitizes both formats rather
// than relying on the XLSX writer's inlineStr type alone).
//
// Every header, row cell, and totals value passed to an export now goes
// through sanitizeCell before being written, at the single choke point
// (RequestExportWithMetadata) rather than in every report builder — so a
// future report type cannot forget it.
package reporting

// formulaTriggers are the characters OWASP's CSV-injection guidance
// names as capable of starting a live formula in a spreadsheet
// application: =, +, -, @, and the tab/CR control characters some
// parsers also treat as cell-leading whitespace before a trigger.
var formulaTriggers = []byte{'=', '+', '-', '@', '\t', '\r'}

// sanitizeCell neutralizes a value that would otherwise be interpreted as
// a formula by prefixing it with a single quote — the standard,
// spreadsheet-application-recognized "treat this as literal text" marker
// — while leaving every other value byte-for-byte unchanged.
func sanitizeCell(v string) string {
	if v == "" {
		return v
	}
	for _, t := range formulaTriggers {
		if v[0] == t {
			return "'" + v
		}
	}
	return v
}

func sanitizeRow(row []string) []string {
	out := make([]string, len(row))
	for i, v := range row {
		out[i] = sanitizeCell(v)
	}
	return out
}

func sanitizeRows(rows [][]string) [][]string {
	out := make([][]string, len(rows))
	for i, r := range rows {
		out[i] = sanitizeRow(r)
	}
	return out
}

// SanitizeCellForTest exposes sanitizeCell to tests in other packages —
// same pattern as BuildRegisterReportForTest in analytics_registers.go.
func SanitizeCellForTest(v string) string { return sanitizeCell(v) }

func sanitizeTotals(totals map[string]string) map[string]string {
	if totals == nil {
		return nil
	}
	out := make(map[string]string, len(totals))
	for k, v := range totals {
		// Keys are developer-authored labels, never user/customer data —
		// only values need sanitizing.
		out[k] = sanitizeCell(v)
	}
	return out
}
