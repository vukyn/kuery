package auth

import (
	"slices"
	"strings"

	pkgCtx "github.com/vukyn/kuery/ctxv3"
	pkgHttp "github.com/vukyn/kuery/http/fiberv3"
	pkgJWT "github.com/vukyn/kuery/jwt"

	"github.com/gofiber/fiber/v3"
)

// NewAuthMiddleware returns a Fiber middleware that verifies the bearer access
// token locally using the given RS256 public key (no network call to the issuer),
// enforces that the token's audience includes appCode, and populates the request
// context with the caller's identity and the permissions granted for appCode.
//
// Responses:
//   - missing/malformed Authorization header -> 401
//   - invalid/expired token signature        -> 401
//   - appCode not in the token audience      -> 403
func NewAuthMiddleware(publicKeyPEM string, appCode string) fiber.Handler {
	return func(c fiber.Ctx) error {
		token, ok := bearerToken(c)
		if !ok {
			return pkgHttp.Unauthorized(c)
		}

		claims, err := pkgJWT.ValidateJWTWithRSAPublicKey(token, publicKeyPEM)
		if err != nil {
			return pkgHttp.Unauthorized(c)
		}

		if !slices.Contains(claims.GetAudience(), appCode) {
			return pkgHttp.Forbidden(c)
		}

		pkgCtx.SetUserIDToFiberCtx(c, claims.GetUserID())
		pkgCtx.SetUserEmailToFiberCtx(c, claims.GetEmail())
		pkgCtx.SetTokenIDToFiberCtx(c, claims.GetTokenID())
		pkgCtx.SetPermsToFiberCtx(c, claims.GetPermsForApp(appCode))

		return c.Next()
	}
}

// bearerToken extracts the token from an "Authorization: Bearer <token>" header.
func bearerToken(c fiber.Ctx) (string, bool) {
	header := c.Get(fiber.HeaderAuthorization)
	if header == "" {
		return "", false
	}
	const prefix = "Bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return "", false
	}
	token := strings.TrimSpace(header[len(prefix):])
	if token == "" {
		return "", false
	}
	return token, true
}
