// Package pdf is the shared report-PDF wrapper over github.com/johnfercher/maroto/v2 —
// one lib for every PDF-report touchpoint across the platform services.
//
// It solves the two things every service would otherwise re-do:
//
//   - VIETNAMESE FONT — a Be Vietnam Pro TTF (Regular + Bold, SIL OFL) is embedded
//     via go:embed and registered as the default font, so diacritics render out of
//     the box. No per-service font file, no CGO.
//   - REPORT PRIMITIVES — a small Document builder (title/meta header, section
//     heading, table, summary rows, page-number footer) over maroto's declarative
//     grid, so a service composes a report from a few calls instead of hand-laying
//     rows/cols.
//
// Report-specific layout (which columns, which totals) stays in the calling
// service; this lib supplies the font + the building blocks — mirroring the
// excel package's split. Output is bytes / io.Writer (never a local-disk save),
// so a caller streams straight to an HTTP response or object storage.
//
// maroto is pure-Go (no CGO) — fine for the platform's fly.io single-binary
// deploy. NOT safe for concurrent use on a single Document; construct per report.
package pdf

import (
	"fmt"
	"io"

	"github.com/johnfercher/maroto/v2"
	"github.com/johnfercher/maroto/v2/pkg/components/col"
	"github.com/johnfercher/maroto/v2/pkg/components/line"
	"github.com/johnfercher/maroto/v2/pkg/components/text"
	"github.com/johnfercher/maroto/v2/pkg/config"
	"github.com/johnfercher/maroto/v2/pkg/consts/align"
	"github.com/johnfercher/maroto/v2/pkg/consts/fontstyle"
	"github.com/johnfercher/maroto/v2/pkg/consts/orientation"
	"github.com/johnfercher/maroto/v2/pkg/core"
	"github.com/johnfercher/maroto/v2/pkg/core/entity"
	"github.com/johnfercher/maroto/v2/pkg/props"
)

// FontFamily is the embedded font registered as the document default. Callers
// that build maroto components directly (bypassing the Document helpers) pass this
// as props.Text{Family: pdf.FontFamily} to keep diacritics working.
const FontFamily = "bevietnampro"

// maxGridSize is maroto's column grid width (12, matching the default) — a Table
// splits it evenly across its columns.
const maxGridSize = 12

// headerFill / lineGrey are the report palette (a calm slate header, light grid
// lines). ARGB-less RGB, maroto's props.Color is R/G/B ints.
var (
	headerFill = &props.Color{Red: 45, Green: 55, Blue: 72}    // slate-800 header band
	headerFont = &props.Color{Red: 255, Green: 255, Blue: 255} // white on the band
	mutedFont  = &props.Color{Red: 100, Green: 116, Blue: 139} // slate-500 meta text
)

// Document is a report builder over a maroto instance. Build it top-to-bottom
// (Title → Meta → Section → Table → Summary …) then call Bytes / WriteTo.
type Document struct {
	m core.Maroto
}

// Option customizes a Document at construction.
type Option func(config.Builder)

// Landscape switches the page to landscape (default is portrait A4) — useful for
// wide cost tables.
func Landscape() Option {
	return func(b config.Builder) { b.WithOrientation(orientation.Horizontal) }
}

// NewDocument starts an A4 report with the embedded Vietnamese font as default and
// a "{current} / {total}" page number in the footer. Returns an error only if the
// embedded font fails to register (should never happen — the bytes are compiled in).
func NewDocument(opts ...Option) (*Document, error) {
	fonts := []*entity.CustomFont{
		{Family: FontFamily, Style: fontstyle.Normal, Bytes: fontRegular},
		{Family: FontFamily, Style: fontstyle.Bold, Bytes: fontBold},
	}

	builder := config.NewBuilder().
		WithCustomFonts(fonts).
		WithDefaultFont(&props.Font{Family: FontFamily, Size: 10}).
		WithPageNumber(props.PageNumber{Pattern: "{current} / {total}", Place: props.Bottom, Family: FontFamily, Color: mutedFont})

	for _, opt := range opts {
		opt(builder)
	}

	return &Document{m: maroto.New(builder.Build())}, nil
}

// Title adds a large bold centered title row.
func (d *Document) Title(title string) {
	d.m.AddRow(12, text.NewCol(maxGridSize, title, props.Text{
		Family: FontFamily, Style: fontstyle.Bold, Size: 16, Align: align.Center,
	}))
}

// Meta adds a row of small muted key/value lines below the title (e.g. garden,
// date range, generated-at). Each entry becomes one centered line. Empty is a no-op.
func (d *Document) Meta(lines ...string) {
	for _, l := range lines {
		d.m.AddRow(5, text.NewCol(maxGridSize, l, props.Text{
			Family: FontFamily, Size: 9, Align: align.Center, Color: mutedFont,
		}))
	}
}

// Section adds a bold left-aligned section heading with a thin rule under it.
func (d *Document) Section(heading string) {
	d.m.AddRow(8, text.NewCol(maxGridSize, heading, props.Text{
		Family: FontFamily, Style: fontstyle.Bold, Size: 12, Top: 2,
	}))
	d.m.AddRow(1, line.NewCol(maxGridSize))
}

// Table renders a headered table. headers sets the columns; each row in rows is
// rendered left-to-right and MUST have len(headers) cells (extra cells are
// ignored, missing cells render blank). The header band is filled + white; data
// rows are lightly bordered. aligns is optional per-column alignment (defaults to
// Left); pass fewer entries than headers to align only the leading columns
// (e.g. right-align a trailing amount column).
func (d *Document) Table(headers []string, rows [][]string, aligns ...align.Type) {
	if len(headers) == 0 {
		return
	}
	colSize := maxGridSize / len(headers)
	if colSize < 1 {
		colSize = 1
	}

	alignOf := func(i int) align.Type {
		if i < len(aligns) {
			return aligns[i]
		}
		return align.Left
	}

	// Header band.
	headerCols := make([]core.Col, len(headers))
	for i, h := range headers {
		headerCols[i] = text.NewCol(colSize, h, props.Text{
			Family: FontFamily, Style: fontstyle.Bold, Size: 10, Align: alignOf(i), Color: headerFont, Left: 1, Right: 1,
		})
	}
	d.m.AddRow(8, headerCols...).WithStyle(&props.Cell{BackgroundColor: headerFill})

	// Data rows.
	for _, r := range rows {
		cells := make([]core.Col, len(headers))
		for i := range headers {
			val := ""
			if i < len(r) {
				val = r[i]
			}
			cells[i] = text.NewCol(colSize, val, props.Text{
				Family: FontFamily, Size: 9, Align: alignOf(i), Left: 1, Right: 1,
			})
		}
		d.m.AddRow(7, cells...)
	}
}

// Summary adds right-aligned label/value lines (e.g. Tổng chi phí · 1.500.000đ)
// under a table. The label is bold; value bold and right-aligned in a narrow
// trailing column. Empty is a no-op.
func (d *Document) Summary(pairs ...[2]string) {
	for _, p := range pairs {
		// Label gets a wide column (6/12) so multi-word labels like "Số nhật ký
		// chăm sóc" stay on one line; value takes the trailing 4/12, both
		// right-aligned toward the page edge.
		d.m.AddRow(7,
			col.New(2),
			text.NewCol(6, p[0], props.Text{Family: FontFamily, Style: fontstyle.Bold, Size: 10, Align: align.Right, Right: 1}),
			text.NewCol(4, p[1], props.Text{Family: FontFamily, Style: fontstyle.Bold, Size: 10, Align: align.Right, Right: 1}),
		)
	}
}

// Spacer adds vertical blank space of the given height (mm).
func (d *Document) Spacer(height float64) {
	if height <= 0 {
		return
	}
	d.m.AddRow(height, col.New(maxGridSize))
}

// AddRow escape-hatches to the underlying maroto grid for report-specific layout
// the helpers don't cover (custom columns, images, barcodes). Use pdf.FontFamily
// on any text props so diacritics keep working.
func (d *Document) AddRow(height float64, cols ...core.Col) core.Row {
	return d.m.AddRow(height, cols...)
}

// Bytes generates the PDF and returns it as bytes (ready for an HTTP body or R2
// PutObject). The Document must not be used after Bytes.
func (d *Document) Bytes() ([]byte, error) {
	doc, err := d.m.Generate()
	if err != nil {
		return nil, fmt.Errorf("pdf: generate: %w", err)
	}
	return doc.GetBytes(), nil
}

// WriteTo generates + writes the PDF to dst.
func (d *Document) WriteTo(dst io.Writer) (int64, error) {
	b, err := d.Bytes()
	if err != nil {
		return 0, err
	}
	n, err := dst.Write(b)
	if err != nil {
		return int64(n), fmt.Errorf("pdf: write-to: %w", err)
	}
	return int64(n), nil
}
