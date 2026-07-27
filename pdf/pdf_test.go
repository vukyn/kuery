package pdf_test

import (
	"bytes"
	"testing"

	"github.com/johnfercher/maroto/v2/pkg/consts/align"
	"github.com/vukyn/kuery/pdf"
)

// TestReport_Generate builds a full report (title, meta, section, table with a
// right-aligned amount column, summary) using Vietnamese text and asserts a valid
// non-empty PDF comes back. Diacritics exercise the embedded font path.
func TestReport_Generate(t *testing.T) {
	t.Parallel()

	doc, err := pdf.NewDocument()
	if err != nil {
		t.Fatalf("NewDocument: %v", err)
	}
	doc.Title("Báo cáo nhật ký chăm sóc")
	doc.Meta("Vườn: Sầu riêng Đắk Lắk", "Kỳ: 01/07/2026 – 27/07/2026")
	doc.Section("Chi phí vật tư")
	doc.Table(
		[]string{"Vật tư", "Số lượng", "Đơn giá", "Thành tiền"},
		[][]string{
			{"Phân bón NPK", "3", "150.000đ", "450.000đ"},
			{"Thuốc trừ sâu", "2", "200.000đ", "400.000đ"},
		},
		align.Left, align.Center, align.Right, align.Right,
	)
	doc.Summary([2]string{"Tổng chi phí", "850.000đ"})

	data, err := doc.Bytes()
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("empty pdf bytes")
	}
	if !bytes.HasPrefix(data, []byte("%PDF")) {
		t.Fatalf("output is not a PDF (prefix = %q)", data[:min(8, len(data))])
	}
}

func TestReport_Landscape_WriteTo(t *testing.T) {
	t.Parallel()

	doc, err := pdf.NewDocument(pdf.Landscape())
	if err != nil {
		t.Fatalf("NewDocument: %v", err)
	}
	doc.Title("Báo cáo tình trạng cây")
	doc.Table([]string{"Ngày", "Đánh giá", "Mức sâu bệnh"}, [][]string{
		{"27/07", "Tốt", "Thấp"},
	})

	var buf bytes.Buffer
	n, err := doc.WriteTo(&buf)
	if err != nil || n == 0 || buf.Len() == 0 {
		t.Fatalf("WriteTo: n=%d err=%v len=%d", n, err, buf.Len())
	}
	if !bytes.HasPrefix(buf.Bytes(), []byte("%PDF")) {
		t.Fatal("output is not a PDF")
	}
}

func TestTable_EmptyHeaders_NoPanic(t *testing.T) {
	t.Parallel()
	doc, err := pdf.NewDocument()
	if err != nil {
		t.Fatalf("NewDocument: %v", err)
	}
	doc.Table(nil, nil) // no-op, must not panic
	doc.Title("x")
	if _, err := doc.Bytes(); err != nil {
		t.Fatalf("Bytes: %v", err)
	}
}
