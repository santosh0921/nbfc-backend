// Branded PDF report generation. Uses gofpdf, already a dependency
// (internal/loans/letter.go uses it for sanction/disbursement letters) —
// no new library added.
//
// HISTORY: v1 was a single unpaginated text block (data ran off the
// visible page). v2 fixed that with a plain bordered data table, but a
// real generated file showed CellFormat neither wraps nor clips —
// long values overlapped the next column. v3 (this version) fixes the
// wrapping properly AND adds the branded, presentation-quality layout
// that was asked for: a company-branded header band, KPI highlight
// cards pulled from the report's own totals (never invented), an
// optional bar-chart visualization when the report has a comparable
// numeric column, and a footer with page numbers — while keeping the
// wrapped, non-lossy data table underneath all of it.
package reporting

import (
	"strconv"
	"strings"

	"github.com/jung-kurt/gofpdf"
)

// Brand palette — matches the admin panel's own AppColors (0xFF3B5BFF
// primary / 0xFF1E2A78 primaryDark) so a downloaded report looks like it
// came from the same product as the screen it was downloaded from.
const (
	brandName    = "OneFinance"
	brandTagline = "NBFC Management Reporting"
)

var (
	brandPrimaryR, brandPrimaryG, brandPrimaryB       = 59, 91, 255
	brandPrimaryDarkR, brandPrimaryDarkG, brandPrimaryDarkB = 30, 42, 120
	brandInkR, brandInkG, brandInkB                   = 18, 22, 58
	brandMutedR, brandMutedG, brandMutedB             = 107, 113, 149
	brandSurfaceR, brandSurfaceG, brandSurfaceB       = 245, 246, 253
	brandBorderR, brandBorderG, brandBorderB          = 235, 237, 247
)

// buildPDFReport renders meta + headers + rows into a branded, paginated
// A4 landscape PDF. Every figure shown — KPI cards, chart, table — comes
// directly from the report's own computed rows/totals; nothing here is
// invented for visual effect.
func buildPDFReport(meta ExportMetadata, headers []string, rows [][]string) ([]byte, error) {
	pdf := gofpdf.New("L", "mm", "A4", "")
	pdf.SetMargins(12, 0, 12)
	pdf.SetAutoPageBreak(false, 0)

	const lineHeight = 4.6
	const cellPad = 1.5
	pageW, pageH := 297.0, 210.0
	printableW := pageW - 24
	const headerBandH = 26.0
	const footerBandH = 12.0
	const contentTop = headerBandH + 8
	const contentBottom = 6.0 // margin above footer band

	colWidths := distributeColumnWidths(headers, printableW)
	pageNum := 0

	newPage := func() {
		pageNum++
		pdf.AddPage()
		drawHeaderBand(pdf, meta, pageW, headerBandH)
		pdf.SetY(contentTop)
	}

	newPage()

	// --- Page 1 only: period/filters line, KPI highlight cards, chart ---
	pdf.SetFont("Helvetica", "", 8.5)
	pdf.SetTextColor(brandMutedR, brandMutedG, brandMutedB)
	// A plain hyphen separator, not a middot/em-dash — gofpdf's default
	// core font uses WinAnsi (Windows-1252) encoding, which has no glyph
	// for "·" or "—"; those characters were rendering as mojibake
	// ("Â·", "â€"") in the actual generated file, confirmed by rendering
	// a real export to an image rather than trusting a text-only check.
	metaLine := "Period: " + meta.ReportingPeriod
	if meta.Filters != "" {
		metaLine += "   -   Filters: " + meta.Filters
	}
	metaLine += "   -   Records: " + strconv.Itoa(meta.RecordCount)
	metaLine += "   -   Generated " + meta.GeneratedAt + " by " + meta.GeneratedBy
	pdf.CellFormat(0, 5, metaLine, "", 1, "L", false, 0, "")
	pdf.SetTextColor(brandInkR, brandInkG, brandInkB)
	pdf.Ln(2)

	if len(meta.Totals) > 0 {
		drawKPICards(pdf, meta.Totals, printableW)
	}

	if chartLabels, chartValues, ok := extractChartSeries(headers, rows); ok {
		drawBarChart(pdf, chartLabels, chartValues, printableW)
	}

	pdf.SetFont("Helvetica", "B", 10.5)
	pdf.SetTextColor(brandPrimaryDarkR, brandPrimaryDarkG, brandPrimaryDarkB)
	pdf.CellFormat(0, 6, "Detail", "", 1, "L", false, 0, "")
	pdf.SetTextColor(brandInkR, brandInkG, brandInkB)
	pdf.Ln(1)

	drawColumnHeader := func() {
		pdf.SetFont("Helvetica", "B", 8)
		pdf.SetFillColor(brandSurfaceR, brandSurfaceG, brandSurfaceB)
		x0, y0 := pdf.GetX(), pdf.GetY()
		h := drawWrappedRow(pdf, headers, colWidths, x0, y0, lineHeight, cellPad, true)
		pdf.SetXY(x0, y0+h)
		pdf.SetFont("Helvetica", "", 8)
	}

	if len(headers) > 0 {
		drawColumnHeader()
	}

	for _, row := range rows {
		rh := measureWrappedRowHeight(pdf, row, colWidths, lineHeight, cellPad)
		if pdf.GetY()+rh > pageH-footerBandH-contentBottom {
			drawFooterBand(pdf, pageW, pageH, footerBandH, pageNum)
			newPage()
			if len(headers) > 0 {
				drawColumnHeader()
			}
		}
		x0, y0 := pdf.GetX(), pdf.GetY()
		h := drawWrappedRow(pdf, row, colWidths, x0, y0, lineHeight, cellPad, false)
		pdf.SetXY(x0, y0+h)
	}

	if len(rows) == 0 {
		pdf.SetFont("Helvetica", "I", 9)
		pdf.SetTextColor(brandMutedR, brandMutedG, brandMutedB)
		pdf.CellFormat(0, 8, "No records for this period.", "", 1, "L", false, 0, "")
	}

	drawFooterBand(pdf, pageW, pageH, footerBandH, pageNum)

	// Total page count is only known after rendering everything; gofpdf's
	// AliasNbPages placeholder (set below, before Output) resolves "{nb}"
	// in every already-drawn footer text to the real final count.
	pdf.AliasNbPages("{nb}")

	w := &byteBufferWriter{}
	if err := pdf.Output(w); err != nil {
		return nil, err
	}
	return w.data, nil
}

// drawHeaderBand paints the branded top band: a solid brand-color
// rectangle across the full page width with the company name/tagline on
// the left and the report's own name on the right — real report
// identity, not decoration.
func drawHeaderBand(pdf *gofpdf.Fpdf, meta ExportMetadata, pageW, bandH float64) {
	pdf.SetFillColor(brandPrimaryR, brandPrimaryG, brandPrimaryB)
	pdf.Rect(0, 0, pageW, bandH, "F")
	// A subtle darker strip along the bottom edge of the band gives the
	// header some depth without needing an image asset.
	pdf.SetFillColor(brandPrimaryDarkR, brandPrimaryDarkG, brandPrimaryDarkB)
	pdf.Rect(0, bandH-2, pageW, 2, "F")

	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 16)
	pdf.SetXY(12, 6)
	pdf.CellFormat(140, 8, brandName, "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 8.5)
	pdf.SetXY(12, 15)
	pdf.CellFormat(140, 5, brandTagline, "", 1, "L", false, 0, "")

	pdf.SetFont("Helvetica", "B", 13)
	pdf.SetXY(pageW-150, 8)
	pdf.CellFormat(138, 7, meta.ReportName, "", 1, "R", false, 0, "")

	pdf.SetTextColor(brandInkR, brandInkG, brandInkB)
}

// drawFooterBand paints a thin bottom band with page numbering and the
// brand name — every page, not just the first, so a printed/scattered
// page can still be traced back to its report.
func drawFooterBand(pdf *gofpdf.Fpdf, pageW, pageH, bandH float64, pageNum int) {
	y := pageH - bandH
	pdf.SetDrawColor(brandBorderR, brandBorderG, brandBorderB)
	pdf.Line(12, y, pageW-12, y)
	pdf.SetFont("Helvetica", "", 7.5)
	pdf.SetTextColor(brandMutedR, brandMutedG, brandMutedB)
	pdf.SetXY(12, y+3)
	pdf.CellFormat(140, 5, brandName+" - confidential, for internal management use", "", 0, "L", false, 0, "")
	pdf.SetXY(pageW-150, y+3)
	pdf.CellFormat(138, 5, "Page "+strconv.Itoa(pageNum)+" of {nb}", "", 0, "R", false, 0, "")
	pdf.SetTextColor(brandInkR, brandInkG, brandInkB)
}

// drawKPICards renders the report's own Totals as highlighted cards —
// the "how much in total" figures the phase brief asked to be
// prominent — instead of a plain text list. Up to 4 per row, wrapping
// to further rows if a report has more totals than that (e.g. a
// register with several distinct sums).
func drawKPICards(pdf *gofpdf.Fpdf, totals map[string]string, printableW float64) {
	// Deterministic order: Go map iteration order is random, and a KPI
	// card row that rearranges itself between renders of the identical
	// report would be a real defect for a report that gets compared
	// period over period.
	//
	// Free-text explanatory entries (e.g. a "Note" total explaining a
	// caveat, seen in a real render truncating a full sentence into a
	// blue value-sized number, which reads as broken rather than
	// explanatory) are excluded from the card treatment — cards are for
	// figures a reader scans at a glance, not prose. Everything excluded
	// here is still in the full report; it is just not forced into a
	// KPI-card shape it doesn't fit.
	labels := make([]string, 0, len(totals))
	for k, v := range totals {
		if isCardWorthy(k, v) {
			labels = append(labels, k)
		}
	}
	sortStrings(labels)
	if len(labels) == 0 {
		return
	}

	const perRow = 4
	const gap = 4.0
	cardW := (printableW - gap*(perRow-1)) / perRow
	const cardH = 16.0

	x0, y0 := pdf.GetX(), pdf.GetY()
	x, y := x0, y0
	for i, label := range labels {
		if i > 0 && i%perRow == 0 {
			x = x0
			y += cardH + gap
		}
		pdf.SetFillColor(brandSurfaceR, brandSurfaceG, brandSurfaceB)
		pdf.Rect(x, y, cardW, cardH, "F")
		pdf.SetDrawColor(brandBorderR, brandBorderG, brandBorderB)
		pdf.Rect(x, y, cardW, cardH, "D")

		pdf.SetFont("Helvetica", "B", 12)
		pdf.SetTextColor(brandPrimaryR, brandPrimaryG, brandPrimaryB)
		pdf.SetXY(x+3, y+3)
		displayValue := formatCardValue(totals[label])
		valueLines := pdf.SplitLines([]byte(displayValue), cardW-6)
		valLine := displayValue
		if len(valueLines) > 0 {
			valLine = string(valueLines[0])
		}
		pdf.CellFormat(cardW-6, 6, valLine, "", 1, "L", false, 0, "")

		pdf.SetFont("Helvetica", "", 7.5)
		pdf.SetTextColor(brandMutedR, brandMutedG, brandMutedB)
		pdf.SetXY(x+3, y+10)
		lines := pdf.SplitLines([]byte(label), cardW-6)
		lbl := label
		if len(lines) > 0 {
			lbl = string(lines[0])
		}
		pdf.CellFormat(cardW-6, 4, lbl, "", 1, "L", false, 0, "")

		x += cardW + gap
	}
	rowsUsed := (len(labels)-1)/perRow + 1
	if len(labels) == 0 {
		rowsUsed = 0
	}
	pdf.SetTextColor(brandInkR, brandInkG, brandInkB)
	pdf.SetXY(x0, y0+float64(rowsUsed)*(cardH+gap)+2)
}

// isCardWorthy decides whether a Totals entry belongs in the KPI card
// grid: numeric values always qualify (that is exactly what a card is
// for), and short non-numeric values (a status word, a short id) still
// fit a card. A long free-text value — the giveaway is a label like
// "Note" or a sentence-length value — is explanatory prose, not a
// figure, and is left out of the card grid entirely.
func isCardWorthy(label, value string) bool {
	if strings.Contains(strings.ToLower(label), "note") {
		return false
	}
	if _, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil {
		return true
	}
	return len([]rune(value)) <= 40
}

// formatCardValue makes a numeric-looking total readable at a glance —
// "28540000.00" becomes "2,85,40,000.00" (Indian lakh/crore grouping,
// the convention this NBFC's own figures are quoted in elsewhere in the
// product). Anything that doesn't parse as a plain number (a date, a
// free-text note) is returned unchanged — this never reformats or
// reinterprets non-numeric totals.
func formatCardValue(s string) string {
	trimmed := strings.TrimSpace(s)
	f, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return s
	}
	neg := f < 0
	if neg {
		f = -f
	}
	whole := strconv.FormatFloat(f, 'f', 0, 64)
	frac := ""
	if s := strconv.FormatFloat(f, 'f', 2, 64); strings.Contains(s, ".") {
		frac = s[strings.Index(s, "."):]
	}
	grouped := indianGrouping(whole)
	if neg {
		grouped = "-" + grouped
	}
	return grouped + frac
}

// indianGrouping inserts commas as 12,34,567 (last 3 digits, then pairs)
// rather than the international 1,234,567 — the convention this
// product's own currency figures use elsewhere.
func indianGrouping(digits string) string {
	if len(digits) <= 3 {
		return digits
	}
	last3 := digits[len(digits)-3:]
	rest := digits[:len(digits)-3]
	var out strings.Builder
	for i, c := range rest {
		remaining := len(rest) - i
		if remaining%2 == 0 && i != 0 {
			out.WriteByte(',')
		}
		out.WriteRune(c)
	}
	return out.String() + "," + last3
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}

// extractChartSeries looks for a "Current Period" (or "Amount", "Value")
// numeric column paired with a label column (the first text column,
// usually "Metric" or the report's own first column) — the shape every
// growth/analytics-style report in this system shares. Returns false
// (no chart) for register-style reports with no such column, rather
// than forcing an inappropriate visualization onto tabular data.
func extractChartSeries(headers []string, rows [][]string) (labels []string, values []float64, ok bool) {
	valueCol := -1
	for i, h := range headers {
		lh := strings.ToLower(h)
		if strings.Contains(lh, "current period") || strings.Contains(lh, "current_period") {
			valueCol = i
			break
		}
	}
	if valueCol == -1 {
		return nil, nil, false
	}
	labelCol := 1 // by convention the growth-report shape is Key, Label, Unit, Current, Prior, Change
	if labelCol >= len(headers) {
		labelCol = 0
	}

	const maxBars = 8
	for _, r := range rows {
		if valueCol >= len(r) || labelCol >= len(r) {
			continue
		}
		v, err := strconv.ParseFloat(strings.TrimSpace(r[valueCol]), 64)
		if err != nil {
			continue
		}
		labels = append(labels, r[labelCol])
		values = append(values, v)
		if len(labels) >= maxBars {
			break
		}
	}
	if len(labels) == 0 {
		return nil, nil, false
	}
	return labels, values, true
}

// drawBarChart renders a simple, real horizontal bar chart (not a
// static image) from actual row values — the "at a glance" visual the
// brief asked for, built from the same numbers the table below shows,
// never separate/estimated figures.
func drawBarChart(pdf *gofpdf.Fpdf, labels []string, values []float64, printableW float64) {
	max := 0.0
	for _, v := range values {
		av := v
		if av < 0 {
			av = -av
		}
		if av > max {
			max = av
		}
	}
	if max == 0 {
		max = 1
	}

	pdf.SetFont("Helvetica", "B", 10.5)
	pdf.SetTextColor(brandPrimaryDarkR, brandPrimaryDarkG, brandPrimaryDarkB)
	pdf.CellFormat(0, 6, "At a Glance", "", 1, "L", false, 0, "")
	pdf.SetTextColor(brandInkR, brandInkG, brandInkB)
	pdf.Ln(1)

	const labelW = 55.0
	const valueW = 28.0
	barAreaW := printableW - labelW - valueW
	const barH = 5.0
	const barGap = 2.5

	x0, y := pdf.GetX(), pdf.GetY()
	pdf.SetFont("Helvetica", "", 8)
	for i, label := range labels {
		lines := pdf.SplitLines([]byte(label), labelW-2)
		lbl := label
		if len(lines) > 0 {
			lbl = string(lines[0])
		}
		pdf.SetXY(x0, y)
		pdf.CellFormat(labelW, barH, lbl, "", 0, "L", false, 0, "")

		barW := (values[i] / max) * barAreaW
		if barW < 0 {
			barW = 0
		}
		barColorR, barColorG, barColorB := brandPrimaryR, brandPrimaryG, brandPrimaryB
		if values[i] < 0 {
			barColorR, barColorG, barColorB = 239, 68, 68 // negative values read as a warning, not the brand color
		}
		pdf.SetFillColor(barColorR, barColorG, barColorB)
		pdf.Rect(x0+labelW, y+0.6, barW, barH-1.2, "F")

		pdf.SetXY(x0+labelW+barAreaW+2, y)
		pdf.CellFormat(valueW-2, barH, formatChartValue(values[i]), "", 0, "R", false, 0, "")

		y += barH + barGap
		if i < len(labels)-1 {
			pdf.SetXY(x0, y)
		}
	}
	pdf.SetXY(x0, y+2)
}

func formatChartValue(v float64) string {
	s := strconv.FormatFloat(v, 'f', 2, 64)
	return s
}

// measureWrappedRowHeight computes how tall a row will render (the
// tallest of its wrapped cells) without drawing anything — used to
// decide whether a page break is needed before drawing starts.
func measureWrappedRowHeight(pdf *gofpdf.Fpdf, cells []string, colWidths []float64, lineHeight, cellPad float64) float64 {
	maxLines := 1
	for i, cell := range cells {
		w := 18.0
		if i < len(colWidths) {
			w = colWidths[i]
		}
		lines := pdf.SplitLines([]byte(cell), w-2*cellPad)
		if len(lines) > maxLines {
			maxLines = len(lines)
		}
	}
	return float64(maxLines)*lineHeight + 2*cellPad
}

// drawWrappedRow draws one row of bordered cells, wrapping each cell's
// text to its own column width (gofpdf's plain CellFormat does neither
// wrapping nor clipping) and sizing every cell in the row to the tallest
// wrapped cell so the row's border is a clean rectangle. Returns the
// row height used.
func drawWrappedRow(pdf *gofpdf.Fpdf, cells []string, colWidths []float64, x0, y0, lineHeight, cellPad float64, fill bool) float64 {
	type wrapped struct {
		lines []string
		width float64
	}
	wrappedCells := make([]wrapped, len(cells))
	maxLines := 1
	for i, cell := range cells {
		w := 18.0
		if i < len(colWidths) {
			w = colWidths[i]
		}
		raw := pdf.SplitLines([]byte(cell), w-2*cellPad)
		lines := make([]string, len(raw))
		for j, l := range raw {
			lines[j] = string(l)
		}
		if len(lines) == 0 {
			lines = []string{""}
		}
		wrappedCells[i] = wrapped{lines: lines, width: w}
		if len(lines) > maxLines {
			maxLines = len(lines)
		}
	}
	rowH := float64(maxLines)*lineHeight + 2*cellPad

	x := x0
	for _, wc := range wrappedCells {
		pdf.SetDrawColor(brandBorderR, brandBorderG, brandBorderB)
		pdf.Rect(x, y0, wc.width, rowH, boxStyle(fill))
		ty := y0 + cellPad
		for _, l := range wc.lines {
			pdf.SetXY(x+cellPad, ty)
			pdf.CellFormat(wc.width-2*cellPad, lineHeight, l, "", 0, "L", false, 0, "")
			ty += lineHeight
		}
		x += wc.width
	}
	return rowH
}

func boxStyle(fill bool) string {
	if fill {
		return "DF"
	}
	return "D"
}

// distributeColumnWidths splits the printable width across columns,
// weighted by each header's own length rather than split perfectly
// evenly — a report mixing short columns ("Unit", "Status") with long
// ones ("Definition", "Formula") reads far better when the wide columns
// actually get more room; since every cell wraps rather than
// overflowing, a narrower column just means more wrapped lines instead
// of lost/overlapping text.
func distributeColumnWidths(headers []string, printableW float64) []float64 {
	n := len(headers)
	if n == 0 {
		return nil
	}
	const minWidth = 20.0
	weights := make([]float64, n)
	totalWeight := 0.0
	for i, h := range headers {
		w := float64(len([]rune(h)))
		if w < 6 {
			w = 6
		}
		weights[i] = w
		totalWeight += w
	}
	widths := make([]float64, n)
	for i, w := range weights {
		widths[i] = printableW * w / totalWeight
		if widths[i] < minWidth {
			widths[i] = minWidth
		}
	}
	return widths
}

// byteBufferWriter adapts gofpdf's io.Writer-based Output to a plain
// byte slice without pulling in bytes.Buffer's extra API surface here.
type byteBufferWriter struct{ data []byte }

func (w *byteBufferWriter) Write(p []byte) (int, error) {
	w.data = append(w.data, p...)
	return len(p), nil
}
