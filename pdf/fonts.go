package pdf

import _ "embed"

// Be Vietnam Pro (SIL OFL 1.1 — see fonts/OFL.txt) is embedded so every service
// gets full Vietnamese-diacritic rendering with zero per-service font setup. The
// two styles (Regular + Bold) are registered under one family (FontFamily) in
// NewDocument. Bytes are compiled into the binary via go:embed — no runtime file
// dependency, works inside the single self-contained service artifact.

//go:embed fonts/BeVietnamPro-Regular.ttf
var fontRegular []byte

//go:embed fonts/BeVietnamPro-Bold.ttf
var fontBold []byte
