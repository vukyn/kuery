package conv

import (
	"fmt"
)

// FormatBytes renders a byte count in binary (1024-based) units.
// Counts below 1024 are printed as whole bytes ("%d B"); larger counts are
// scaled to the biggest unit that keeps the value at or above 1 and printed
// with one decimal place ("%.1f %cB", units K, M, G, T, P, E).
// Negative counts keep their sign and are scaled by their magnitude, so a
// delta reads the same way as a size.
// Rounding happens after the unit is chosen, so a count just below the next
// unit can print as "1024.0" of the smaller one.
//
// Example:
//
//	fmt.Println(conv.FormatBytes(0))       // 0 B
//	fmt.Println(conv.FormatBytes(1023))    // 1023 B
//	fmt.Println(conv.FormatBytes(1024))    // 1.0 KB
//	fmt.Println(conv.FormatBytes(1536))    // 1.5 KB
//	fmt.Println(conv.FormatBytes(1 << 30)) // 1.0 GB
//	fmt.Println(conv.FormatBytes(-1536))   // -1.5 KB
func FormatBytes(size int64) string {
	const unit = 1024

	sign := ""
	magnitude := uint64(size)
	if size < 0 {
		sign = "-"
		// Negating in two's complement then reading as unsigned gives the true
		// magnitude even for the smallest int64, whose negation overflows int64.
		magnitude = uint64(-size)
	}

	if magnitude < unit {
		return fmt.Sprintf("%s%d B", sign, magnitude)
	}
	divisor, exponent := uint64(unit), 0
	for remaining := magnitude / unit; remaining >= unit; remaining /= unit {
		divisor *= unit
		exponent++
	}
	return fmt.Sprintf("%s%.1f %cB", sign, float64(magnitude)/float64(divisor), "KMGTPE"[exponent])
}
