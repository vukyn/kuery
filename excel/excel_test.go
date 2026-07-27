package excel_test

import (
	"bytes"
	"testing"

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
