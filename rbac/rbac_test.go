package rbac

import (
	"net/http"
	"net/http/httptest"
	"testing"

	pkgCtx "github.com/vukyn/kuery/ctx"

	"github.com/gofiber/fiber/v2"
)

// newTestApp wires a route guarded by the given middleware, with the caller's
// perms populated as the service's auth middleware would.
func newTestApp(perms []string, guard fiber.Handler) *fiber.App {
	app := fiber.New()
	app.Get("/protected",
		func(c *fiber.Ctx) error {
			pkgCtx.SetPermsToFiberCtx(c, perms)
			return c.Next()
		},
		guard,
		func(c *fiber.Ctx) error {
			return c.SendStatus(fiber.StatusOK)
		},
	)
	return app
}

func TestPerm(t *testing.T) {
	if got, want := Perm("user", "read"), "user:read"; got != want {
		t.Errorf("Perm() = %q, want %q", got, want)
	}
}

func TestRequirePermission(t *testing.T) {
	tests := []struct {
		name       string
		perms      []string
		wantStatus int
	}{
		{"allow when permission held", []string{"user:read", "role:read"}, http.StatusOK},
		{"deny when permission missing", []string{"role:read"}, http.StatusForbidden},
		{"deny when no permissions", nil, http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := newTestApp(tt.perms, RequirePermission("user:read"))
			resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/protected", nil))
			if err != nil {
				t.Fatalf("app.Test() error = %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != tt.wantStatus {
				t.Errorf("status = %d, want %d", resp.StatusCode, tt.wantStatus)
			}
		})
	}
}

func TestRequireAnyPermission(t *testing.T) {
	tests := []struct {
		name       string
		perms      []string
		wantStatus int
	}{
		{"allow when first permission held", []string{"user:read"}, http.StatusOK},
		{"allow when second permission held", []string{"user:create"}, http.StatusOK},
		{"deny when none held", []string{"role:read"}, http.StatusForbidden},
		{"deny when no permissions", nil, http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := newTestApp(tt.perms, RequireAnyPermission("user:read", "user:create"))
			resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/protected", nil))
			if err != nil {
				t.Fatalf("app.Test() error = %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != tt.wantStatus {
				t.Errorf("status = %d, want %d", resp.StatusCode, tt.wantStatus)
			}
		})
	}
}
