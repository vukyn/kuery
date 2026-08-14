package log

import (
	"strings"
	"testing"
)

// Init's error is fatal at startup, so its message is the whole diagnosis the
// reader gets. It must NAME the alternatives — and it must name ones that work.
func TestInitRejectionNamesTheValidValues(t *testing.T) {
	t.Run("mode", func(t *testing.T) {
		err := Init(Config{Mode: "dev", Level: LOG_LEVEL_INFO})
		if err == nil {
			t.Fatalf("an unknown mode must be rejected")
		}
		// The offending value is quoted: an empty Mode (an unset env var) is the
		// most common cause, and unquoted it renders as a blank gap in the sentence.
		if !strings.Contains(err.Error(), `"dev"`) {
			t.Fatalf("the rejected value must be quoted in the message: %v", err)
		}
		for _, mode := range LogModes {
			if !strings.Contains(err.Error(), mode) {
				t.Fatalf("message must offer %q, got: %v", mode, err)
			}
		}
	})

	t.Run("level", func(t *testing.T) {
		err := Init(Config{Mode: LOG_MODE_JSON, Level: "verbose"})
		if err == nil {
			t.Fatalf("an unknown level must be rejected")
		}
		if !strings.Contains(err.Error(), `"verbose"`) {
			t.Fatalf("the rejected value must be quoted in the message: %v", err)
		}
		for _, level := range LogLevels {
			if !strings.Contains(err.Error(), level) {
				t.Fatalf("message must offer %q, got: %v", level, err)
			}
		}
	})

	t.Run("an unset value is quoted, not blank", func(t *testing.T) {
		err := Init(Config{Mode: "", Level: LOG_LEVEL_INFO})
		if err == nil || !strings.Contains(err.Error(), `""`) {
			t.Fatalf("an empty mode must read as an empty value, got: %v", err)
		}
	})
}

// 🚨 The message is a promise. Nothing in the compiler ties LogModes/LogLevels to
// the switch statements in Init, so a value added to one and not the other makes
// the error advertise something Init then rejects — the worst possible answer to
// "what should I have written". This is the only guard against that drift.
func TestInitAcceptsEveryDocumentedValue(t *testing.T) {
	for _, mode := range LogModes {
		if err := Init(Config{Mode: mode, Level: LOG_LEVEL_INFO}); err != nil {
			t.Fatalf("LogModes offers %q but Init rejects it: %v", mode, err)
		}
	}
	for _, level := range LogLevels {
		if err := Init(Config{Mode: LOG_MODE_JSON, Level: level}); err != nil {
			t.Fatalf("LogLevels offers %q but Init rejects it: %v", level, err)
		}
	}
}
