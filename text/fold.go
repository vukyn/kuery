// Package text provides text-normalization helpers shared across services.
package text

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// FoldVN normalizes text for diacritic-insensitive (accent-insensitive) search,
// tuned for Vietnamese. It maps đ/Đ → d (a distinct Vietnamese letter that NFD
// does NOT decompose), NFD-decomposes the rest, strips combining marks, lowercases,
// and trims. Example: "Bản Tình Ca" → "ban tinh ca".
//
// Store FoldVN(value) in a denormalized *_search column and compare it against
// FoldVN(query) so a query typed with or without diacritics matches accented data.
func FoldVN(s string) string {
	// đ/Đ are standalone letters (U+0111/U+0110), not base+combining-mark, so NFD
	// leaves them intact — map them explicitly before decomposition.
	s = strings.Map(func(r rune) rune {
		switch r {
		case 'đ', 'Đ':
			return 'd'
		default:
			return r
		}
	}, s)

	s = norm.NFD.String(s)

	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if unicode.Is(unicode.Mn, r) {
			// Mn = non-spacing combining mark (the stripped accents).
			continue
		}
		b.WriteRune(r)
	}

	return strings.ToLower(strings.TrimSpace(b.String()))
}
