package auth

import (
	"slices"
	"testing"

	pkgCtx "github.com/vukyn/kuery/ctx"

	"github.com/gofiber/fiber/v2"
)

func newAppsApp(pubPEM string, appCodes []string) (*fiber.App, *capture) {
	cap := &capture{}
	app := fiber.New()
	app.Get("/protected", NewAuthMiddlewareForApps(pubPEM, appCodes), func(c *fiber.Ctx) error {
		cap.reached = true
		cap.perms = pkgCtx.GetPermsFromFiberCtx(c)
		return c.SendStatus(fiber.StatusOK)
	})
	return app, cap
}

func TestAppsMiddlewareFirstAppAccepted(t *testing.T) {
	privPEM, pubPEM := rsaPEMPair(t)
	app, cap := newAppsApp(pubPEM, []string{"rainy", "remuxer"})
	token := signToken(t, privPEM, []string{"rainy"}, map[string][]string{
		"rainy": {"track:read"},
	})

	if status := request(t, app, "Bearer "+token); status != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", status, fiber.StatusOK)
	}
	if want := []string{"track:read"}; !slices.Equal(cap.perms, want) {
		t.Errorf("perms = %v, want %v", cap.perms, want)
	}
}

func TestAppsMiddlewareSecondAppAccepted(t *testing.T) {
	privPEM, pubPEM := rsaPEMPair(t)
	app, cap := newAppsApp(pubPEM, []string{"rainy", "remuxer"})
	token := signToken(t, privPEM, []string{"remuxer"}, map[string][]string{
		"remuxer": {"music:read"},
	})

	if status := request(t, app, "Bearer "+token); status != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", status, fiber.StatusOK)
	}
	if want := []string{"music:read"}; !slices.Equal(cap.perms, want) {
		t.Errorf("perms = %v, want %v", cap.perms, want)
	}
}

func TestAppsMiddlewareOtherAppForbidden(t *testing.T) {
	privPEM, pubPEM := rsaPEMPair(t)
	app, cap := newAppsApp(pubPEM, []string{"rainy", "remuxer"})
	token := signToken(t, privPEM, []string{"bistro"}, map[string][]string{
		"bistro": {"x:read"},
	})

	if status := request(t, app, "Bearer "+token); status != fiber.StatusForbidden {
		t.Fatalf("status = %d, want %d", status, fiber.StatusForbidden)
	}
	if cap.reached {
		t.Error("handler reached on a forbidden token")
	}
}

func TestAppsMiddlewareEmptyListForbidden(t *testing.T) {
	privPEM, pubPEM := rsaPEMPair(t)
	app, cap := newAppsApp(pubPEM, nil)
	token := signToken(t, privPEM, []string{"rainy"}, map[string][]string{
		"rainy": {"track:read"},
	})

	if status := request(t, app, "Bearer "+token); status != fiber.StatusForbidden {
		t.Fatalf("status = %d, want %d", status, fiber.StatusForbidden)
	}
	if cap.reached {
		t.Error("handler reached with an empty app list")
	}
}

func TestAppsMiddlewareBadSignatureUnauthorized(t *testing.T) {
	signingPriv, _ := rsaPEMPair(t)
	_, verifyingPub := rsaPEMPair(t)
	app, _ := newAppsApp(verifyingPub, []string{"rainy", "remuxer"})
	token := signToken(t, signingPriv, []string{"rainy"}, map[string][]string{
		"rainy": {"track:read"},
	})

	if status := request(t, app, "Bearer "+token); status != fiber.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", status, fiber.StatusUnauthorized)
	}
}

func TestAppsMiddlewarePermsOfMatchedAppOnly(t *testing.T) {
	privPEM, pubPEM := rsaPEMPair(t)
	app, cap := newAppsApp(pubPEM, []string{"rainy", "remuxer"})
	token := signToken(t, privPEM, []string{"remuxer"}, map[string][]string{
		"rainy":   {"track:write"},
		"remuxer": {"music:read"},
	})

	if status := request(t, app, "Bearer "+token); status != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", status, fiber.StatusOK)
	}
	if want := []string{"music:read"}; !slices.Equal(cap.perms, want) {
		t.Errorf("perms = %v, want only the matched app's %v", cap.perms, want)
	}
}
