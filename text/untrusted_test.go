package text

import "testing"

func TestStripControl(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "plain text untouched", input: "nguyenvana", want: "nguyenvana"},
		{name: "newlines and tabs dropped", input: "gho\nst\r\tadmin", want: "ghostadmin"},
		{name: "vietnamese preserved", input: "Nguyễn Văn A", want: "Nguyễn Văn A"},
		{name: "empty stays empty", input: "", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StripControl(tt.input); got != tt.want {
				t.Fatalf("StripControl(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name  string
		input string
		limit int
		want  string
	}{
		{name: "shorter than limit untouched", input: "abc", limit: 10, want: "abc"},
		{name: "exactly at limit untouched", input: "abc", limit: 3, want: "abc"},
		{name: "longer gets marker", input: "abcdef", limit: 3, want: "abc" + TruncationMarker},
		// Cutting by runes, not bytes: 3 runes of Vietnamese stay 3 readable runes.
		{name: "multi-byte cut by rune", input: "Nguyễn", limit: 3, want: "Ngu" + TruncationMarker},
		{name: "zero limit empties", input: "abc", limit: 0, want: ""},
		{name: "negative limit empties", input: "abc", limit: -1, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Truncate(tt.input, tt.limit); got != tt.want {
				t.Fatalf("Truncate(%q, %d) = %q, want %q", tt.input, tt.limit, got, tt.want)
			}
		})
	}
}

func TestSanitizeUntrusted(t *testing.T) {
	// Both defences in one call: the injected newline is gone AND the oversized
	// body is cut to the limit plus the marker.
	got := SanitizeUntrusted("ad\nmin"+string(make([]rune, 0))+"xxxxxxxxxx", 8)
	if want := "adminxxx" + TruncationMarker; got != want {
		t.Fatalf("SanitizeUntrusted = %q, want %q", got, want)
	}
	if runeCount := len([]rune(got)); runeCount != 9 {
		t.Fatalf("expected 8 runes + marker, got %d", runeCount)
	}
}
