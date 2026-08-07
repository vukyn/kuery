// Package excel is the shared XLSX wrapper over github.com/xuri/excelize/v2 —
// one lib for every spreadsheet touchpoint across the platform services
// (report export, error reports, fixed-template downloads, upload parsing).
//
//   - READ  — parse an uploaded .xlsx (validate-first structural read · row apply).
//   - WRITE — stream-generate an .xlsx into memory (per-row report · fixed-template
//     download · report export). Streaming keeps large files (thousands of rows)
//     off the heap-all-at-once path.
//
// Output is bytes / io.Writer — never a local-disk SaveAs — so a caller streams
// straight to an HTTP response or object storage (R2). The Workbook type is the
// non-streaming, multi-sheet counterpart for richer documents (multiple sheets,
// dropdown data-validation, native tables) that the StreamWriter path can't build.
//
// NOT safe for concurrent use on a single Book/Writer/Workbook. Construct per
// operation.
package excel

import (
	"bytes"
	"fmt"
	"io"

	"github.com/xuri/excelize/v2"
)

// Decompression guards (security · zip-bomb / billion-laughs DoS). excelize's
// defaults are 16 GB total + 16 MB per-worksheet-XML — a tiny crafted .xlsx (zip,
// highly compressible XML) can inflate toward those ceilings → heap OOM. A caller
// byte-cap only bounds the COMPRESSED upload; decompression happens INSIDE
// OpenReader, so the real guard must live here (the single ingestion chokepoint).
const (
	maxUnzipBytes    = 256 << 20 // 256 MiB total decompressed
	maxUnzipXMLBytes = 64 << 20  // 64 MiB per worksheet XML
)

// =================================================================
// READ
// =================================================================

// Book is a read handle over a parsed workbook. Close it when done to release
// the excelize temp buffers.
type Book struct {
	f *excelize.File
}

// Open parses an .xlsx from raw bytes (an uploaded file held in memory). Returns
// an error for a corrupt / non-OOXML payload — the caller surfaces it as a
// user-facing "cannot parse" fault, not an internal error.
func Open(data []byte) (*Book, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data), excelize.Options{
		UnzipSizeLimit:    maxUnzipBytes,
		UnzipXMLSizeLimit: maxUnzipXMLBytes,
	})
	if err != nil {
		return nil, fmt.Errorf("excel: open: %w", err)
	}
	return &Book{f: f}, nil
}

// SheetNames returns the workbook's sheet names in tab order.
func (b *Book) SheetNames() []string { return b.f.GetSheetList() }

// Rows returns every row of a sheet as string cells (header row included, row 0).
// Trailing empty cells are excelize-trimmed per row.
func (b *Book) Rows(sheet string) ([][]string, error) {
	rows, err := b.f.GetRows(sheet)
	if err != nil {
		return nil, fmt.Errorf("excel: rows %q: %w", sheet, err)
	}
	return rows, nil
}

// FirstSheetRows returns the rows of the first sheet — the convention for
// single-sheet import templates. Returns an error when the workbook has no sheet.
func (b *Book) FirstSheetRows() ([][]string, error) {
	names := b.f.GetSheetList()
	if len(names) == 0 {
		return nil, fmt.Errorf("excel: workbook has no sheet")
	}
	return b.Rows(names[0])
}

// Close releases the underlying file buffers.
func (b *Book) Close() error { return b.f.Close() }

// =================================================================
// WRITE (streaming · in-memory → HTTP/R2)
// =================================================================

// Writer stream-builds a single-sheet .xlsx into memory. Rows are appended in
// order via SetHeader (row 1) then WriteRow; Bytes() flushes + returns the file.
type Writer struct {
	f       *excelize.File
	sw      *excelize.StreamWriter
	sheet   string
	nextRow int // 1-based next row to write
}

// NewWriter starts a streaming workbook with a single sheet (default "Sheet1"
// renamed to sheetName when non-empty).
func NewWriter(sheetName string) (*Writer, error) {
	f := excelize.NewFile()
	sheet := "Sheet1"
	if sheetName != "" {
		if err := f.SetSheetName("Sheet1", sheetName); err != nil {
			_ = f.Close()
			return nil, fmt.Errorf("excel: name sheet: %w", err)
		}
		sheet = sheetName
	}
	sw, err := f.NewStreamWriter(sheet)
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("excel: stream writer: %w", err)
	}
	return &Writer{f: f, sw: sw, sheet: sheet, nextRow: 1}, nil
}

// SetHeader writes cols as a bold header on row 1. Must be called before any
// WriteRow (it consumes row 1). A nil/empty cols is a no-op.
func (w *Writer) SetHeader(cols []string) error {
	if len(cols) == 0 {
		return nil
	}
	styleID, err := w.f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return fmt.Errorf("excel: header style: %w", err)
	}
	cells := make([]any, len(cols))
	for i, c := range cols {
		cells[i] = excelize.Cell{StyleID: styleID, Value: neutralizeFormula(c)}
	}
	cell, _ := excelize.CoordinatesToCellName(1, w.nextRow)
	if err := w.sw.SetRow(cell, cells); err != nil {
		return fmt.Errorf("excel: header row: %w", err)
	}
	w.nextRow++
	return nil
}

// WriteRow appends one data row (values written left-to-right from column A).
// String values are formula-neutralized (security · a report reflecting
// user-controlled data — names, notes — where a cell starting =/+/-/@ would
// execute when the tenant opens the file in Excel/Sheets). Non-string values
// pass through.
func (w *Writer) WriteRow(values []any) error {
	safe := make([]any, len(values))
	for i, v := range values {
		if s, ok := v.(string); ok {
			safe[i] = neutralizeFormula(s)
		} else {
			safe[i] = v
		}
	}
	cell, _ := excelize.CoordinatesToCellName(1, w.nextRow)
	if err := w.sw.SetRow(cell, safe); err != nil {
		return fmt.Errorf("excel: write row %d: %w", w.nextRow, err)
	}
	w.nextRow++
	return nil
}

// neutralizeFormula prefixes a single quote to a string whose first character is
// a spreadsheet formula/command trigger (= + - @ TAB CR), so Excel/Sheets renders
// it as literal text instead of executing it (CSV/formula-injection · OWASP).
func neutralizeFormula(s string) string {
	if s == "" {
		return s
	}
	switch s[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + s
	default:
		return s
	}
}

// Bytes flushes the stream and returns the finished .xlsx as bytes (ready for an
// HTTP body or R2 PutObject). The Writer must not be used after Bytes.
func (w *Writer) Bytes() ([]byte, error) {
	if err := w.sw.Flush(); err != nil {
		return nil, fmt.Errorf("excel: flush: %w", err)
	}
	buf, err := w.f.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("excel: serialize: %w", err)
	}
	return buf.Bytes(), nil
}

// WriteTo flushes + streams the finished .xlsx to dst (io.Writer path for large
// files that should not be fully buffered by the caller).
func (w *Writer) WriteTo(dst io.Writer) (int64, error) {
	if err := w.sw.Flush(); err != nil {
		return 0, fmt.Errorf("excel: flush: %w", err)
	}
	n, err := w.f.WriteTo(dst)
	if err != nil {
		return n, fmt.Errorf("excel: write-to: %w", err)
	}
	return n, nil
}

// Close releases the underlying file. Safe to call after Bytes/WriteTo.
func (w *Writer) Close() error { return w.f.Close() }

// =================================================================
// WORKBOOK (multi-sheet · in-memory → HTTP/R2 · normal-mode)
// =================================================================
//
// Workbook is the NON-streaming counterpart of Writer, for richer documents that
// need MULTIPLE sheets (e.g. a summary sheet + a detail sheet), per-column
// dropdown data-validation, or native Excel tables — none of which the
// StreamWriter path supports (StreamWriter only exposes SetRow/AddTable/MergeCell;
// data validation is a *File-level feature via AddDataValidation, and a second
// sheet needs the normal SetSheetRow model).
//
// Such documents are typically small (summary + a few hundred rows), so the
// normal SetSheetRow API is fine — streaming is only justified for the large
// single-sheet path (Writer). Keeping the two types separate leaves the streaming
// Writer untouched.

// Workbook stream-builds a multi-sheet .xlsx via the normal excelize File API.
// NOT safe for concurrent use; construct per operation.
type Workbook struct {
	f        *excelize.File
	nextRow  map[string]int // 1-based next row per sheet
	hdrStyle int            // lazily-created bold header style (0 = uncreated)
}

// NewWorkbook starts a multi-sheet workbook with a single sheet (default
// "Sheet1" renamed to firstSheet when non-empty). Add more via AddSheet.
func NewWorkbook(firstSheet string) (*Workbook, error) {
	f := excelize.NewFile()
	sheet := "Sheet1"
	if firstSheet != "" {
		if err := f.SetSheetName("Sheet1", firstSheet); err != nil {
			_ = f.Close()
			return nil, fmt.Errorf("excel: name sheet: %w", err)
		}
		sheet = firstSheet
	}
	return &Workbook{f: f, nextRow: map[string]int{sheet: 1}}, nil
}

// AddSheet appends a new empty sheet. Returns an error if the name collides.
func (b *Workbook) AddSheet(name string) error {
	if name == "" {
		return fmt.Errorf("excel: add sheet: empty name")
	}
	if _, exists := b.nextRow[name]; exists {
		return fmt.Errorf("excel: add sheet: duplicate %q", name)
	}
	if _, err := b.f.NewSheet(name); err != nil {
		return fmt.Errorf("excel: add sheet %q: %w", name, err)
	}
	b.nextRow[name] = 1
	return nil
}

// SetHeader writes cols as a bold header on the sheet's next row (row 1 when the
// sheet is fresh). A nil/empty cols is a no-op.
func (b *Workbook) SetHeader(sheet string, cols []string) error {
	if len(cols) == 0 {
		return nil
	}
	if err := b.writeRow(sheet, toAnySafe(cols)); err != nil {
		return err
	}
	if b.hdrStyle == 0 {
		id, err := b.f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
		if err != nil {
			return fmt.Errorf("excel: header style: %w", err)
		}
		b.hdrStyle = id
	}
	row := b.nextRow[sheet] - 1 // the row just written
	first, _ := excelize.CoordinatesToCellName(1, row)
	last, _ := excelize.CoordinatesToCellName(len(cols), row)
	if err := b.f.SetCellStyle(sheet, first, last, b.hdrStyle); err != nil {
		return fmt.Errorf("excel: header style apply: %w", err)
	}
	return nil
}

// WriteRow appends one data row to the sheet (values left-to-right from column A).
// String values are formula-neutralized (mirrors Writer.WriteRow · OWASP).
func (b *Workbook) WriteRow(sheet string, values []any) error {
	safe := make([]any, len(values))
	for i, v := range values {
		if s, ok := v.(string); ok {
			safe[i] = neutralizeFormula(s)
		} else {
			safe[i] = v
		}
	}
	return b.writeRow(sheet, safe)
}

// AddTable formats cellRange (e.g. "A1:H4") on the sheet as a native Excel table —
// banded rows + autofilter, and auto-expands as the user types below the last row.
// name must be unique + letter-led. An empty range is a no-op. cellRange MUST
// include the header row + ≥1 data row (excelize rejects a header-only table).
func (b *Workbook) AddTable(sheet, cellRange, name string) error {
	if cellRange == "" {
		return nil
	}
	if _, ok := b.nextRow[sheet]; !ok {
		return fmt.Errorf("excel: add table: unknown sheet %q", sheet)
	}
	stripes := true
	if err := b.f.AddTable(sheet, &excelize.Table{
		Range:          cellRange,
		Name:           name,
		StyleName:      "TableStyleMedium2",
		ShowRowStripes: &stripes,
	}); err != nil {
		return fmt.Errorf("excel: add table on %q: %w", sheet, err)
	}
	return nil
}

// FreezeTopRow freezes the sheet's first row so the header stays visible while the
// user scrolls. No-op on an unknown sheet.
func (b *Workbook) FreezeTopRow(sheet string) error {
	if _, ok := b.nextRow[sheet]; !ok {
		return fmt.Errorf("excel: freeze: unknown sheet %q", sheet)
	}
	if err := b.f.SetPanes(sheet, &excelize.Panes{
		Freeze:      true,
		YSplit:      1,
		TopLeftCell: "A2",
		ActivePane:  "bottomLeft",
	}); err != nil {
		return fmt.Errorf("excel: freeze top row on %q: %w", sheet, err)
	}
	return nil
}

// SetAutoFilter enables the filter/sort dropdowns on the header range (e.g.
// "A1:H1"). An empty range is a no-op.
func (b *Workbook) SetAutoFilter(sheet, cellRange string) error {
	if cellRange == "" {
		return nil
	}
	if _, ok := b.nextRow[sheet]; !ok {
		return fmt.Errorf("excel: autofilter: unknown sheet %q", sheet)
	}
	if err := b.f.AutoFilter(sheet, cellRange, []excelize.AutoFilterOptions{}); err != nil {
		return fmt.Errorf("excel: autofilter on %q: %w", sheet, err)
	}
	return nil
}

// StyleHeaderCells applies a bold + solid-fill style with the given fill + font
// colours (argb, e.g. fill "003366" / font "FFFFFF") to the 1-based header column
// indices on row 1 — used to colour-code a report header with a legible title.
// fontARGB "" keeps the default (black) font. Each call creates one shared style.
// Out-of-range/empty indices are skipped.
func (b *Workbook) StyleHeaderCells(sheet string, colIdxs []int, fillARGB, fontARGB string) error {
	if len(colIdxs) == 0 {
		return nil
	}
	if _, ok := b.nextRow[sheet]; !ok {
		return fmt.Errorf("excel: style header: unknown sheet %q", sheet)
	}
	font := &excelize.Font{Bold: true}
	if fontARGB != "" {
		font.Color = fontARGB
	}
	styleID, err := b.f.NewStyle(&excelize.Style{
		Font: font,
		Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{fillARGB}},
	})
	if err != nil {
		return fmt.Errorf("excel: header style: %w", err)
	}
	for _, idx := range colIdxs {
		if idx < 1 {
			continue
		}
		cell, cerr := excelize.CoordinatesToCellName(idx, 1)
		if cerr != nil {
			return fmt.Errorf("excel: header style cell %d: %w", idx, cerr)
		}
		if err := b.f.SetCellStyle(sheet, cell, cell, styleID); err != nil {
			return fmt.Errorf("excel: header style apply %s: %w", cell, err)
		}
	}
	return nil
}

// LastRow returns the 1-based index of the row most recently written to the sheet,
// or 0 when nothing has been written yet (and for an unknown sheet).
//
// Exists so a caller can style the row it just wrote without re-deriving the number
// from a loop index — that derivation has to account for the header row, which is
// exactly the off-by-one worth not repeating per report.
func (b *Workbook) LastRow(sheet string) int {
	next, ok := b.nextRow[sheet]
	if !ok || next <= 1 {
		return 0
	}
	return next - 1
}

// StyleRow applies a solid fill and/or font colour across columns 1..columns of a
// 1-based row — e.g. tinting a voided record so it reads as void at a glance
// instead of only in a status column.
//
// fillARGB "" leaves the background alone; fontARGB "" leaves the font colour alone
// (pass both to do both). Deliberately no bold: bold is the header's signature, and
// a highlighted data row that is also bold competes with it.
//
// Called once per highlighted row, so it creates a style per call — deliberately NOT
// cached, because excelize already dedupes identical styles (measured: 50 identical
// NewStyle calls all return id 1). A cache here would be code that cannot be observed
// working, which is worse than none.
//
// A row < 1 or columns < 1 is a no-op.
func (b *Workbook) StyleRow(sheet string, row, columns int, fillARGB, fontARGB string) error {
	if row < 1 || columns < 1 {
		return nil
	}
	if _, ok := b.nextRow[sheet]; !ok {
		return fmt.Errorf("excel: style row: unknown sheet %q", sheet)
	}
	if fillARGB == "" && fontARGB == "" {
		return nil
	}

	style := &excelize.Style{}
	if fillARGB != "" {
		style.Fill = excelize.Fill{
			Type:    "pattern",
			Pattern: 1,
			Color:   []string{fillARGB},
		}
	}
	if fontARGB != "" {
		style.Font = &excelize.Font{Color: fontARGB}
	}
	styleID, styleErr := b.f.NewStyle(style)
	if styleErr != nil {
		return fmt.Errorf("excel: row style: %w", styleErr)
	}

	first, err := excelize.CoordinatesToCellName(1, row)
	if err != nil {
		return fmt.Errorf("excel: style row %d: %w", row, err)
	}
	last, err := excelize.CoordinatesToCellName(columns, row)
	if err != nil {
		return fmt.Errorf("excel: style row %d: %w", row, err)
	}
	if err := b.f.SetCellStyle(sheet, first, last, styleID); err != nil {
		return fmt.Errorf("excel: style row %d apply: %w", row, err)
	}
	return nil
}

// SetColWidth sets the width of the columns between startCol and endCol (1-based,
// inclusive) on the sheet — report ergonomics so long labels aren't clipped. No-op
// on an unknown sheet or a non-positive width.
func (b *Workbook) SetColWidth(sheet string, startCol, endCol int, width float64) error {
	if width <= 0 {
		return nil
	}
	if _, ok := b.nextRow[sheet]; !ok {
		return fmt.Errorf("excel: col width: unknown sheet %q", sheet)
	}
	start, err := excelize.ColumnNumberToName(startCol)
	if err != nil {
		return fmt.Errorf("excel: col width start %d: %w", startCol, err)
	}
	end, err := excelize.ColumnNumberToName(endCol)
	if err != nil {
		return fmt.Errorf("excel: col width end %d: %w", endCol, err)
	}
	if err := b.f.SetColWidth(sheet, start, end, width); err != nil {
		return fmt.Errorf("excel: set col width on %q: %w", sheet, err)
	}
	return nil
}

// writeRow is the shared append body (header + data). values are written verbatim
// (the caller has already neutralized strings where required).
func (b *Workbook) writeRow(sheet string, values []any) error {
	row, ok := b.nextRow[sheet]
	if !ok {
		return fmt.Errorf("excel: write row: unknown sheet %q", sheet)
	}
	cell, _ := excelize.CoordinatesToCellName(1, row)
	if err := b.f.SetSheetRow(sheet, cell, &values); err != nil {
		return fmt.Errorf("excel: write row %d on %q: %w", row, sheet, err)
	}
	b.nextRow[sheet] = row + 1
	return nil
}

// SetDataValidation attaches a dropdown (list) validation to a single column over
// [firstRow, lastRow] on the sheet — e.g. an enum column. A nil/empty allowed set
// is a no-op. Excel caps an inline drop-list at 255 joined characters; a longer
// set returns an error from excelize.
func (b *Workbook) SetDataValidation(sheet, colLetter string, firstRow, lastRow int, allowed []string) error {
	if len(allowed) == 0 {
		return nil
	}
	dv := excelize.NewDataValidation(true)
	dv.SetSqref(fmt.Sprintf("%s%d:%s%d", colLetter, firstRow, colLetter, lastRow))
	if err := dv.SetDropList(allowed); err != nil {
		return fmt.Errorf("excel: drop-list %s: %w", colLetter, err)
	}
	if err := b.f.AddDataValidation(sheet, dv); err != nil {
		return fmt.Errorf("excel: data validation %s: %w", colLetter, err)
	}
	return nil
}

// Bytes serializes the finished .xlsx (ready for an HTTP body or R2 PutObject).
func (b *Workbook) Bytes() ([]byte, error) {
	buf, err := b.f.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("excel: serialize: %w", err)
	}
	return buf.Bytes(), nil
}

// Close releases the underlying file.
func (b *Workbook) Close() error { return b.f.Close() }

// ColumnLetter returns the spreadsheet column name for a 1-based column index
// (1→"A", 27→"AA"). Confines the excelize dependency to this lib so callers don't
// import excelize just to compute a column letter.
func ColumnLetter(col1 int) (string, error) {
	name, err := excelize.ColumnNumberToName(col1)
	if err != nil {
		return "", fmt.Errorf("excel: column letter %d: %w", col1, err)
	}
	return name, nil
}

// toAnySafe converts strings to []any with formula-neutralization (header helper).
func toAnySafe(cols []string) []any {
	out := make([]any, len(cols))
	for i, c := range cols {
		out[i] = neutralizeFormula(c)
	}
	return out
}
