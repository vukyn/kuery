package text

import (
	"strings"
	"unicode"
)

// TruncationMarker is appended to a value Truncate actually cut, so a shortened
// string reads as shortened rather than as the whole thing.
const TruncationMarker = "…"

// StripControl removes every Unicode control character from s. Use it on any
// value that came from a request and is about to be logged or stored: a newline
// inside an attacker-supplied field forges extra log lines (log injection), and
// control bytes corrupt terminal output and log parsers.
func StripControl(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
}

// Truncate cuts s to at most limit RUNES (never mid-rune, so multi-byte
// Vietnamese text survives) and appends TruncationMarker when it cut. A limit of
// zero or less returns the empty string.
func Truncate(s string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit]) + TruncationMarker
}

// SanitizeUntrusted is StripControl followed by Truncate — the standard treatment
// for a request-supplied value that is about to be persisted or logged. It bounds
// what one oversized request can write and removes the log-injection vector in a
// single call.
func SanitizeUntrusted(s string, limit int) string {
	return Truncate(StripControl(s), limit)
}
