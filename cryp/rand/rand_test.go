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
