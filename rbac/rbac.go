package rbac

import (
	"slices"

	pkgCtx "github.com/vukyn/kuery/ctx"
	pkgHttp "github.com/vukyn/kuery/http/fiber"

	"github.com/gofiber/fiber/v2"
)

// Perm builds a permission code from a resource and an action
func Perm(resource, action string) string {
	return resource + ":" + action
}

// RequirePermission returns a Fiber middleware that allows the request
// only when the caller holds the given permission. Admins always pass.
// Must run after the service's auth middleware has populated the claims.
func RequirePermission(perm string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if pkgCtx.GetUserIsAdminFromFiberCtx(c) {
			return c.Next()
		}
		if slices.Contains(pkgCtx.GetPermsFromFiberCtx(c), perm) {
			return c.Next()
		}
		return pkgHttp.Forbidden(c)
	}
}

// RequireAnyPermission returns a Fiber middleware that allows the request
// when the caller holds at least one of the given permissions. Admins always pass.
// Must run after the service's auth middleware has populated the claims.
func RequireAnyPermission(perms ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if pkgCtx.GetUserIsAdminFromFiberCtx(c) {
			return c.Next()
		}
		callerPerms := pkgCtx.GetPermsFromFiberCtx(c)
		for _, perm := range perms {
			if slices.Contains(callerPerms, perm) {
				return c.Next()
			}
		}
		return pkgHttp.Forbidden(c)
	}
}
