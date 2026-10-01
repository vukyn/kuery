package conv

import (
	"math"
	"testing"
)

func TestFormatBytes(t *testing.T) {
	cases := []struct {
		name string
		size int64
		want string
	}{
		{"zero", 0, "0 B"},
		{"largest whole-byte count", 1023, "1023 B"},
		{"exactly one kibibyte switches unit", 1024, "1.0 KB"},
		{"fraction keeps one decimal", 1536, "1.5 KB"},
		{"exactly one mebibyte", 1 << 20, "1.0 MB"},
		{"exactly one gibibyte", 1 << 30, "1.0 GB"},
		{"unit is chosen before rounding", 1<<20 - 1, "1024.0 KB"},
		{"max int64 reaches the largest unit", math.MaxInt64, "8.0 EB"},
		{"negative whole bytes keep the sign", -5, "-5 B"},
		{"negative is scaled by magnitude", -1536, "-1.5 KB"},
		{"min int64 does not overflow", math.MinInt64, "-8.0 EB"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := FormatBytes(testCase.size); got != testCase.want {
				t.Errorf("FormatBytes(%d) = %q, want %q", testCase.size, got, testCase.want)
			}
		})
	}
}
