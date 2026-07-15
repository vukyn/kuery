package ctx

import (
	"context"

	pkgClaims "github.com/vukyn/kuery/claims"

	"github.com/gofiber/fiber/v3"
	"github.com/sarulabs/di/v2"
)

type ContextKey string

var (
	UserIDKey             ContextKey = "user_id"
	EmailKey              ContextKey = "email"
	TokenIDKey            ContextKey = "token_id"
	ClientIPKey           ContextKey = "client_ip"
	UserAgentKey          ContextKey = "user_agent"
	PermsKey              ContextKey = "perms"
	DiContainerRequestKey ContextKey = "di_container_request"
)

// SetClaimsToFiberCtx is isme-internal: it populates the request ctx from a
// freshly-validated session token. Downstream services use the kuery/auth
// middleware instead, which resolves the per-app perms via GetPermsForApp.
// The perms slice here means "perms for this app" and is set by the caller's
// own middleware (e.g. isme uses GetPermsForApp("isme")).
func SetClaimsToFiberCtx(ctx fiber.Ctx, claims pkgClaims.Claims) {
	ctx.Locals(string(UserIDKey), claims.GetUserID())
	ctx.Locals(string(EmailKey), claims.GetEmail())
	ctx.Locals(string(TokenIDKey), claims.GetTokenID())
}

func SetUserIDToFiberCtx(ctx fiber.Ctx, userID string) {
	ctx.Locals(string(UserIDKey), userID)
}

func SetUserEmailToFiberCtx(ctx fiber.Ctx, email string) {
	ctx.Locals(string(EmailKey), email)
}

func SetTokenIDToFiberCtx(ctx fiber.Ctx, tokenID string) {
	ctx.Locals(string(TokenIDKey), tokenID)
}

// SetPermsToFiberCtx stores the caller's permission codes for the current app.
func SetPermsToFiberCtx(ctx fiber.Ctx, perms []string) {
	ctx.Locals(string(PermsKey), perms)
}

func NewContextFromFiberCtx(fiberCtx fiber.Ctx) context.Context {
	userID := GetUserIdFromFiberCtx(fiberCtx)
	email := GetUserEmailFromFiberCtx(fiberCtx)
	tokenID := GetTokenIDFromFiberCtx(fiberCtx)
	userAgent := GetUserAgentFromFiberCtx(fiberCtx)
	clientIP := GetClientIPFromFiberCtx(fiberCtx)
	perms := GetPermsFromFiberCtx(fiberCtx)

	ctx := context.Background()
	ctx = context.WithValue(ctx, UserIDKey, userID)
	ctx = context.WithValue(ctx, EmailKey, email)
	ctx = context.WithValue(ctx, TokenIDKey, tokenID)
	ctx = context.WithValue(ctx, UserAgentKey, userAgent)
	ctx = context.WithValue(ctx, ClientIPKey, clientIP)
	ctx = context.WithValue(ctx, PermsKey, perms)
	return ctx
}

func GetUserID(ctx context.Context) string {
	userID := ctx.Value(UserIDKey)
	if userID == nil {
		return ""
	}
	if userID, ok := userID.(string); ok {
		return userID
	}
	return ""
}

func GetEmail(ctx context.Context) string {
	email := ctx.Value(EmailKey)
	if email == nil {
		return ""
	}
	if email, ok := email.(string); ok {
		return email
	}
	return ""
}

func GetTokenID(ctx context.Context) string {
	tokenID := ctx.Value(TokenIDKey)
	if tokenID == nil {
		return ""
	}
	if tokenID, ok := tokenID.(string); ok {
		return tokenID
	}
	return ""
}

func GetClientIP(ctx context.Context) string {
	clientIP := ctx.Value(ClientIPKey)
	if clientIP == nil {
		return ""
	}
	if clientIP, ok := clientIP.(string); ok {
		return clientIP
	}
	return ""
}

func GetUserAgent(ctx context.Context) string {
	userAgent := ctx.Value(UserAgentKey)
	if userAgent == nil {
		return ""
	}
	return userAgent.(string)
}

func GetPerms(ctx context.Context) []string {
	perms := ctx.Value(PermsKey)
	if perms == nil {
		return nil
	}
	if perms, ok := perms.([]string); ok {
		return perms
	}
	return nil
}

func GetUserIdFromFiberCtx(ctx fiber.Ctx) string {
	val := ctx.Locals(string(UserIDKey))
	if val == nil {
		return ""
	}
	return val.(string)
}

func GetUserEmailFromFiberCtx(ctx fiber.Ctx) string {
	val := ctx.Locals(string(EmailKey))
	if val == nil {
		return ""
	}
	return val.(string)
}

func GetTokenIDFromFiberCtx(ctx fiber.Ctx) string {
	val := ctx.Locals(string(TokenIDKey))
	if val == nil {
		return ""
	}
	return val.(string)
}

func GetUserAgentFromFiberCtx(ctx fiber.Ctx) string {
	return ctx.Get("User-Agent")
}

func GetClientIPFromFiberCtx(ctx fiber.Ctx) string {
	return ctx.IP()
}

func GetPermsFromFiberCtx(ctx fiber.Ctx) []string {
	val := ctx.Locals(string(PermsKey))
	if val == nil {
		return nil
	}
	if perms, ok := val.([]string); ok {
		return perms
	}
	return nil
}

func SetDiContainerRequestToFiberCtx(ctx fiber.Ctx, request di.Container) {
	ctx.Locals(string(DiContainerRequestKey), request)
}

func GetDiContainerRequestFromFiberCtx(ctx fiber.Ctx) di.Container {
	container := ctx.Locals(string(DiContainerRequestKey))
	if container == nil {
		return di.Container{}
	}
	if container, ok := container.(di.Container); ok {
		return container
	}
	return di.Container{}
}
