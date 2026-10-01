package rand

import (
	"strings"
	"testing"

	"github.com/vukyn/kuery/t"
)

// TestRandMixedStringNeverEmpty is the guard for the failure this function used
// to have: it returned "" on an error path instead of failing, and its callers
// — which mint share tokens, API keys and slugs — do not check a string. An
// empty token is a credential that fails OPEN, so "never empty" is the property
// that matters, not just "usually right".
func TestRandMixedStringNeverEmpty(t2 *testing.T) {
	cases := []struct {
		name       string
		length     int
		hasNumber  bool
		hasSpecial bool
	}{
		{"letters only", 16, false, false},
		{"with numbers", 20, true, false},
		{"with numbers and specials", 32, true, true},
		{"single character", 1, true, true},
	}

	for _, testCase := range cases {
		t2.Run(testCase.name, func(t2 *testing.T) {
			// Repeated because the old bug was probabilistic in shape: one
			// failing draw out of n produced a short-or-empty result.
			for range 200 {
				got := RandMixedString(testCase.length, testCase.hasNumber, testCase.hasSpecial)
				if got == "" {
					t2.Fatal("returned an empty string — an empty token would be issued as a credential")
				}
				if len(got) != testCase.length {
					t2.Fatalf("length = %d, want %d (got %q)", len(got), testCase.length, got)
				}

				charset := t.LowerLetters + t.UpperLetters
				if testCase.hasNumber {
					charset += t.Numbers
				}
				if testCase.hasSpecial {
					charset += t.SpecialChars
				}
				for _, character := range got {
					if !strings.ContainsRune(charset, character) {
						t2.Fatalf("character %q is outside the requested charset (got %q)", character, got)
					}
				}
			}
		})
	}
}

// TestRandMixedStringVaries is the weakest useful check that the result is
// actually drawn rather than constant — a constant return would satisfy every
// assertion above.
func TestRandMixedStringVaries(t2 *testing.T) {
	seen := make(map[string]struct{}, 64)
	for range 64 {
		seen[RandMixedString(24, true, true)] = struct{}{}
	}
	if len(seen) < 60 {
		t2.Fatalf("only %d distinct values out of 64 draws — the output is not random", len(seen))
	}
}

func TestFromCharsetUsesOnlyTheCharset(t2 *testing.T) {
	const charset = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"
	for range 500 {
		code, err := FromCharset(6, charset)
		if err != nil {
			t2.Fatalf("FromCharset: %v", err)
		}
		if len(code) != 6 {
			t2.Fatalf("len = %d, want 6 (%q)", len(code), code)
		}
		for _, r := range code {
			if !strings.ContainsRune(charset, r) {
				t2.Fatalf("rune %q not in charset (%q)", r, code)
			}
		}
	}
}

func TestFromCharsetVaries(t2 *testing.T) {
	seen := map[string]bool{}
	for range 200 {
		code, err := FromCharset(8, "abcdefghijklmnopqrstuvwxyz")
		if err != nil {
			t2.Fatalf("FromCharset: %v", err)
		}
		seen[code] = true
	}
	// 26^8 outcomes: any repeat in 200 draws means the source is not random.
	if len(seen) != 200 {
		t2.Fatalf("got %d distinct codes out of 200", len(seen))
	}
}

func TestFromCharsetSupportsMultibyteRunes(t2 *testing.T) {
	code, err := FromCharset(4, "đăâ")
	if err != nil {
		t2.Fatalf("FromCharset: %v", err)
	}
	if got := len([]rune(code)); got != 4 {
		t2.Fatalf("rune count = %d, want 4 (%q)", got, code)
	}
}

func TestFromCharsetRejectsBadInput(t2 *testing.T) {
	if _, err := FromCharset(0, "abc"); err == nil {
		t2.Fatal("n = 0 must fail")
	}
	if _, err := FromCharset(4, ""); err == nil {
		t2.Fatal("empty charset must fail")
	}
}
