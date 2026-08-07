package excel_test

import (
	"bytes"
	"testing"

	"github.com/xuri/excelize/v2"

	"github.com/vukyn/kuery/excel"
)

// roundTrip writes a header + 2 data rows, then re-opens the bytes and reads
// them back — exercises both the write (StreamWriter) and read (OpenReader) paths.
func TestWriteThenRead_RoundTrip(t *testing.T) {
	t.Parallel()

	w, err := excel.NewWriter("Items")
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	defer w.Close()

	if err := w.SetHeader([]string{"code", "name", "price"}); err != nil {
		t.Fatalf("SetHeader: %v", err)
	}
	if err := w.WriteRow([]any{"C-1", "Widget", 100}); err != nil {
		t.Fatalf("WriteRow 1: %v", err)
	}
	if err := w.WriteRow([]any{"C-2", "Gadget", 250}); err != nil {
		t.Fatalf("WriteRow 2: %v", err)
	}

	data, err := w.Bytes()
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("empty xlsx bytes")
	}

	book, err := excel.Open(data)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer book.Close()

	if names := book.SheetNames(); len(names) != 1 || names[0] != "Items" {
		t.Fatalf("sheet names = %v, want [Items]", names)
	}

	rows, err := book.FirstSheetRows()
	if err != nil {
		t.Fatalf("FirstSheetRows: %v", err)
	}
	if len(rows) != 3 { // header + 2 data
		t.Fatalf("rows = %d, want 3", len(rows))
	}
	if got := rows[0]; len(got) != 3 || got[0] != "code" || got[2] != "price" {
		t.Fatalf("header row = %v", got)
	}
	if got := rows[1]; got[0] != "C-1" || got[1] != "Widget" || got[2] != "100" {
		t.Fatalf("data row 1 = %v", got)
	}
	if got := rows[2]; got[0] != "C-2" || got[2] != "250" {
		t.Fatalf("data row 2 = %v", got)
	}
}

// TestWriteRow_FormulaNeutralized: a data cell starting with a formula trigger
// is stored as literal text ('-prefixed) so it can't execute on open.
func TestWriteRow_FormulaNeutralized(t *testing.T) {
	t.Parallel()
	w, err := excel.NewWriter("E")
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	defer w.Close()
	_ = w.SetHeader([]string{"col"})
	if err := w.WriteRow([]any{"=HYPERLINK(\"http://evil\")"}); err != nil {
		t.Fatalf("WriteRow: %v", err)
	}
	data, err := w.Bytes()
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}
	book, err := excel.Open(data)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer book.Close()
	rows, _ := book.FirstSheetRows()
	// Neutralized: stored as literal text with a leading apostrophe (Excel's
	// text-marker · invisible in the Excel UI) — NOT a live formula.
	if len(rows) != 2 || rows[1][0] != "'=HYPERLINK(\"http://evil\")" {
		t.Fatalf("expected apostrophe-neutralized text cell, got %v", rows)
	}
}

func TestOpen_CorruptBytes(t *testing.T) {
	t.Parallel()
	if _, err := excel.Open([]byte("not an xlsx")); err == nil {
		t.Fatal("expected error opening non-xlsx bytes")
	}
}

func TestWriteTo_Streams(t *testing.T) {
	t.Parallel()
	w, err := excel.NewWriter("")
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	defer w.Close()
	_ = w.SetHeader([]string{"a", "b"})
	_ = w.WriteRow([]any{"1", "2"})

	var buf bytes.Buffer
	// excelize's File.WriteTo may report n=0 even on success; the written bytes
	// are the reliable signal.
	if _, err := w.WriteTo(&buf); err != nil || buf.Len() == 0 {
		t.Fatalf("WriteTo: err=%v len=%d", err, buf.Len())
	}
}

// TestWorkbook_MultiSheet writes two sheets and reads both back — exercises the
// non-streaming Workbook path plus header styling / table / freeze plumbing.
func TestWorkbook_MultiSheet(t *testing.T) {
	t.Parallel()
	wb, err := excel.NewWorkbook("Summary")
	if err != nil {
		t.Fatalf("NewWorkbook: %v", err)
	}
	defer wb.Close()

	if err := wb.SetHeader("Summary", []string{"metric", "value"}); err != nil {
		t.Fatalf("SetHeader summary: %v", err)
	}
	if err := wb.WriteRow("Summary", []any{"total_cost", 1500000}); err != nil {
		t.Fatalf("WriteRow summary: %v", err)
	}
	if err := wb.StyleHeaderCells("Summary", []int{1, 2}, "003366", "FFFFFF"); err != nil {
		t.Fatalf("StyleHeaderCells: %v", err)
	}
	if err := wb.FreezeTopRow("Summary"); err != nil {
		t.Fatalf("FreezeTopRow: %v", err)
	}
	if err := wb.SetColWidth("Summary", 1, 2, 24); err != nil {
		t.Fatalf("SetColWidth: %v", err)
	}

	if err := wb.AddSheet("Detail"); err != nil {
		t.Fatalf("AddSheet: %v", err)
	}
	if err := wb.SetHeader("Detail", []string{"item", "qty", "amount"}); err != nil {
		t.Fatalf("SetHeader detail: %v", err)
	}
	if err := wb.WriteRow("Detail", []any{"Phân bón", 3, 450000}); err != nil {
		t.Fatalf("WriteRow detail: %v", err)
	}
	if err := wb.AddTable("Detail", "A1:C2", "DetailTbl"); err != nil {
		t.Fatalf("AddTable: %v", err)
	}

	data, err := wb.Bytes()
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}
	book, err := excel.Open(data)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer book.Close()

	if names := book.SheetNames(); len(names) != 2 || names[0] != "Summary" || names[1] != "Detail" {
		t.Fatalf("sheet names = %v, want [Summary Detail]", names)
	}
	detail, err := book.Rows("Detail")
	if err != nil {
		t.Fatalf("Rows Detail: %v", err)
	}
	if len(detail) != 2 || detail[1][0] != "Phân bón" || detail[1][2] != "450000" {
		t.Fatalf("detail rows = %v", detail)
	}
}

func TestAddSheet_DuplicateRejected(t *testing.T) {
	t.Parallel()
	wb, err := excel.NewWorkbook("S")
	if err != nil {
		t.Fatalf("NewWorkbook: %v", err)
	}
	defer wb.Close()
	if err := wb.AddSheet("S"); err == nil {
		t.Fatal("expected duplicate-sheet error")
	}
}

func TestColumnLetter(t *testing.T) {
	t.Parallel()
	if got, err := excel.ColumnLetter(27); err != nil || got != "AA" {
		t.Fatalf("ColumnLetter(27) = %q, %v; want AA", got, err)
	}
}

func TestLastRowTracksWrites(t *testing.T) {
	t.Parallel()
	wb, err := excel.NewWorkbook("S")
	if err != nil {
		t.Fatalf("NewWorkbook: %v", err)
	}
	defer wb.Close()

	if got := wb.LastRow("S"); got != 0 {
		t.Fatalf("LastRow before any write = %d, want 0", got)
	}
	if got := wb.LastRow("nope"); got != 0 {
		t.Fatalf("LastRow of an unknown sheet = %d, want 0", got)
	}
	if err := wb.SetHeader("S", []string{"A", "B"}); err != nil {
		t.Fatalf("SetHeader: %v", err)
	}
	if got := wb.LastRow("S"); got != 1 {
		t.Fatalf("LastRow after the header = %d, want 1", got)
	}
	if err := wb.WriteRow("S", []any{1, 2}); err != nil {
		t.Fatalf("WriteRow: %v", err)
	}
	if got := wb.LastRow("S"); got != 2 {
		t.Fatalf("LastRow after one data row = %d, want 2", got)
	}
}

// StyleRow must paint the row it is pointed at, and repeated identical highlights
// must not multiply styles in the file. That second half needs NO cache in kuery:
// excelize dedupes identical styles itself (measured — 50 identical NewStyle calls
// all return id 1), which is why StyleRow just builds the style per call. The
// assertion is kept so a future excelize that stops deduping is caught here rather
// than in a bloated report.
func TestStyleRowPaintsAndSharesOneStyle(t *testing.T) {
	t.Parallel()
	wb, err := excel.NewWorkbook("S")
	if err != nil {
		t.Fatalf("NewWorkbook: %v", err)
	}
	defer wb.Close()

	if err := wb.SetHeader("S", []string{"A", "B"}); err != nil {
		t.Fatalf("SetHeader: %v", err)
	}
	styled := make([]int, 0, 3)
	for i := range 3 {
		if err := wb.WriteRow("S", []any{i, i}); err != nil {
			t.Fatalf("WriteRow: %v", err)
		}
		row := wb.LastRow("S")
		if err := wb.StyleRow("S", row, 2, "FFF1F2", "B91C1C"); err != nil {
			t.Fatalf("StyleRow: %v", err)
		}
		styled = append(styled, row)
	}

	// No-ops rather than errors: a caller looping over rows should not have to
	// special-case an empty highlight.
	if err := wb.StyleRow("S", 0, 2, "FFF1F2", ""); err != nil {
		t.Fatalf("StyleRow(row 0): %v", err)
	}
	if err := wb.StyleRow("S", 2, 2, "", ""); err != nil {
		t.Fatalf("StyleRow(no colours): %v", err)
	}
	if err := wb.StyleRow("nope", 2, 2, "FFF1F2", ""); err == nil {
		t.Fatal("StyleRow on an unknown sheet must error")
	}

	data, err := wb.Bytes()
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}
	// The painted rows must carry a style id, and all three must share ONE id.
	ids := map[int]bool{}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	for _, row := range styled {
		cell, _ := excelize.CoordinatesToCellName(1, row)
		id, err := f.GetCellStyle("S", cell)
		if err != nil {
			t.Fatalf("GetCellStyle %s: %v", cell, err)
		}
		if id == 0 {
			t.Fatalf("row %d carries no style", row)
		}
		ids[id] = true
	}
	if len(ids) != 1 {
		t.Fatalf("identical highlights must collapse to one style, got %d: %v", len(ids), ids)
	}
}
