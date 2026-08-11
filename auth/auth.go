package auth

import (
	"context"
	"slices"
	"strings"

	pkgCtx "github.com/vukyn/kuery/ctx"
	pkgHttp "github.com/vukyn/kuery/http/fiber"
	pkgJWT "github.com/vukyn/kuery/jwt"

	"github.com/gofiber/fiber/v2"
)

// RevocationChecker answers whether the session behind an otherwise-valid access
// token has been revoked. A nil checker disables the check.
//
// An access token is a bearer credential that stays cryptographically valid until
// it expires, so logout and password changes cannot take effect before the token's
// TTL elapses unless something consults issuer-side state. This interface is that
// something; the implementation (a DB lookup, a cache in front of one, a Redis
// set) belongs to the service, not to kuery.
type RevocationChecker interface {
	IsRevoked(ctx context.Context, sessionID string) (bool, error)
}

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
	return NewAuthMiddlewareWithRevocation(publicKeyPEM, appCode, nil)
}

// NewAuthMiddlewareWithRevocation is NewAuthMiddleware plus a session-revocation
// check: a token whose signature and audience are fine is still rejected when the
// checker reports that its session has been revoked.
//
// Order inside the handler: extract bearer -> verify signature -> check audience
// -> populate the request context -> revocation check. The context is populated
// BEFORE the check on purpose, so a service's own logging/recovery sees who the
// caller was even on a request that ends in a revocation 401.
//
// Responses: those of NewAuthMiddleware, plus
//   - session revoked -> 401
//
// Revocation-check semantics:
//   - checker == nil: the check is skipped entirely. This is the behavior of
//     NewAuthMiddleware and of every service that does not track sessions.
//   - empty "sid" claim: the check is skipped. Tokens minted before the claim
//     existed carry no session id; they cannot be looked up, and they age out on
//     their own within the access-token TTL.
//   - IsRevoked returns an error: the request CONTINUES, and the returned bool is
//     ignored. The middleware deliberately owns no fail-open/fail-closed policy —
//     the checker does, because only it knows whether the error was a transient
//     backend blip (where rejecting every request would turn one blip into a mass
//     logout) or a real signal, and only it can log the error with context. So a
//     checker that wants a failure to reject the request must decide that itself
//     and return (true, nil); returning (true, err) is read as "continue".
func NewAuthMiddlewareWithRevocation(publicKeyPEM string, appCode string, checker RevocationChecker) fiber.Handler {
	return func(c *fiber.Ctx) error {
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

		sessionID := claims.GetSessionID()

		pkgCtx.SetUserIDToFiberCtx(c, claims.GetUserID())
		pkgCtx.SetUserEmailToFiberCtx(c, claims.GetEmail())
		pkgCtx.SetTokenIDToFiberCtx(c, claims.GetTokenID())
		pkgCtx.SetSessionIDToFiberCtx(c, sessionID)
		pkgCtx.SetPermsToFiberCtx(c, claims.GetPermsForApp(appCode))

		if checker != nil && sessionID != "" {
			// c.UserContext() rather than context.Background(): the check runs on
			// the request path, so it must be cancelled when the client goes away.
			revoked, err := checker.IsRevoked(c.UserContext(), sessionID)
			if err == nil && revoked {
				return pkgHttp.Unauthorized(c)
			}
		}

		return c.Next()
	}
}

// bearerToken extracts the token from an "Authorization: Bearer <token>" header.
func bearerToken(c *fiber.Ctx) (string, bool) {
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
