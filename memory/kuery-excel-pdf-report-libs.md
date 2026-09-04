---
name: kuery-excel-pdf-report-libs
description: kuery/excel (excelize wrapper) + kuery/pdf (maroto v2 + embedded VN font) shared report libs for gardener R7 export
metadata: 
  node_type: memory
  type: project
  modified: 2026-07-27T07:20:46.388Z
---

Built two shared report libs in kuery for gardener's R7 report export (Excel/PDF), tech was unchosen. Lib-first, feature after.

- **kuery/excel** — XLSX wrapper over `github.com/xuri/excelize/v2`. Ported from o2o `B-excel` (`/Users/vuky10/Hasaki/o2o/o2o-platform/code/contexts/backend/shared/B-excel`). Three types: `Book` (read/parse upload), `Writer` (single-sheet streaming), `Workbook` (multi-sheet + tables/dropdowns/freeze/col-width/header-fill). Security kept: zip-bomb unzip caps + formula-injection neutralize (`= + - @ TAB CR` → `'`-prefix). Output bytes/io.Writer (no local SaveAs). Added SetColWidth beyond o2o original.
- **kuery/pdf** — report wrapper over `github.com/johnfercher/maroto/v2` (v2.4.0). Chosen over go-pdf/fpdf (boilerplate) + chromedp (heavy fly.io image). Pure-Go, no CGO. **Be Vietnam Pro TTF (Regular+Bold, SIL OFL) go:embed'd** in `pdf/fonts/` → VN diacritics free, no per-service font setup. `Document` builder: Title/Meta/Section/Table(headers,rows,aligns)/Summary/Spacer/AddRow escape-hatch → Bytes()/WriteTo(). `pdf.FontFamily`="bevietnampro" const, `pdf.Landscape()` option. Font bytes MUST stay committed (embedded).

Scope decision (user): kuery = font + primitives; report-specific layout stays in gardener (mirrors excel split).

Status: both build + test green, vet clean. **SHIPPED kuery v1.42.0** (PR #32). **v1.43.0** (PR #33) fixed `pdf.Summary` label column (was 2/12 → long VI labels like "Số nhật ký chăm sóc" wrapped; now label 6/12 + value 4/12, row 7mm). **v1.44.0** (PR #34) tightened `pdf.NewDocument` top margin 10mm→6mm. **v1.45.0**→**v1.46.0**: Summary value alignment iterated — final = label right(8/12) + value **right(4/12)** (clean numeric column). gardener re-added `doc.Spacer(4)` between Meta and Summary. Tags pruned to 5 newest (currently v1.42–v1.46). gardener on v1.46.0.

PDF layout iterated live from user screenshots. LESSON: `pdf.Summary` is opinionated — after several kuery releases for pixel tweaks, moved gardener's summary rendering to a LOCAL `summaryRow` helper (builder_pdf.go) via `doc.AddRow` escape-hatch (imports maroto text/col/props/fontstyle + `pdf.FontFamily`). Final APPROVED layout: leading spacer 6/12 + label right 4/12 + value right 2/12 (values right-aligned to page edge like the cost table, label pushed right beside them). Tune ratios in-repo, no kuery release. For future PDF report tweaks: hand-roll rows in the service, don't re-release kuery.

GOTCHA: right after tagging a new kuery version, `go get` may hit sum.golang.org 500 (checksum-DB lag) — retry with `GOSUMDB=off go get github.com/vukyn/kuery@vX`.

Report feature scope (decided): 3 reports — **care-log cost + tree-status → Excel** (detail data), **summary/dashboard → PDF** (1-page). **Owner/manager only** (RequireAnyRole, no worker export → roll-ups always included). Filters: **period + plot + garden**. New `report` domain composes existing usecases, no entity/repo (like `sync`). Decisions: care-log Excel = **2-sheet** ("Chi phí" per-doc+grand-total / "Chi tiết dòng" per-line); export UI **in /dashboard** (CostDashboard, already RequireManagement).

Progress:
- **Backend P1+P2 DONE** → gardener PR #61 (branch feat/report-export). 3 endpoints `GET /api/v1/reports/{care-log-cost.xlsx,tree-status.xlsx,summary.pdf}`, file downloads, X-Garden-ID wrapped per-handler. Added `ListForReport` + From/To filter to care_log & tree_status; report domain also injects garden+plot usecases (nil-guarded) to render names not ULIDs. Cancelled excluded from totals+count, flagged in Excel. No migrations. 173 tests pass. NOT merged yet.
- **P3 frontend** = export section in CostDashboard. Designer mock DONE in `demo/orchard-cost-dashboard-design.html` (online+offline states, 3 buttons, period-inherit, optional plot picker) — left uncommitted, awaiting user review before coder port (mock-first rule [[review-mock-before-coder-port]]).

See [[gardener-carelog-revamp]] [[gardener-manager-dashboard-and-reset]] [[kuery-shared-lib-rule]] [[no-artifact-in-pet-platform]].
