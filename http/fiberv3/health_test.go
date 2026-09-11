package http

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	pkgHealth "github.com/vukyn/kuery/http/health"

	"github.com/gofiber/fiber/v3"
)

// TestHealthHandlerServesUnwrappedReportV3 pins the response shape the endpoint
// promises: the report itself at the top level, NOT inside the platform's
// base.Response{code,message,data} envelope, so `curl | jq .status` works.
func TestHealthHandlerServesUnwrappedReportV3(t *testing.T) {
	app := fiber.New()
	app.Get("/__version", Health(pkgHealth.New("github.com/gofiber/fiber/v3")))

	response, err := app.Test(httptest.NewRequest("GET", "/__version", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusOK)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	fields := map[string]json.RawMessage{}
	if err := json.Unmarshal(body, &fields); err != nil {
		t.Fatalf("unmarshal body %s: %v", body, err)
	}

	if _, wrapped := fields["data"]; wrapped {
		t.Fatalf("body %s is wrapped in the base.Response envelope; the report must be top level", body)
	}
	for _, key := range []string{"status", "go_version", "modules", "started_at", "uptime_seconds"} {
		if _, ok := fields[key]; !ok {
			t.Fatalf("body %s is missing required field %q", body, key)
		}
	}

	var status string
	if err := json.Unmarshal(fields["status"], &status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if status != pkgHealth.StatusOK {
		t.Fatalf("status = %q, want %q", status, pkgHealth.StatusOK)
	}
}
