package ctx

import (
	"context"

	pkgClaims "github.com/vukyn/kuery/claims"

	"github.com/gofiber/fiber/v2"
	"github.com/sarulabs/di/v2"
)

type ContextKey string

var (
	UserIDKey             ContextKey = "user_id"
	EmailKey              ContextKey = "email"
	TokenIDKey            ContextKey = "token_id"
	ClientIPKey           ContextKey = "client_ip"
	UserAgentKey          ContextKey = "user_agent"
	IsAdminKey            ContextKey = "is_admin"
	DiContainerRequestKey ContextKey = "di_container_request"
)

func SetClaimsToFiberCtx(ctx *fiber.Ctx, claims pkgClaims.Claims) {
	ctx.Locals(string(UserIDKey), claims.GetUserID())
	ctx.Locals(string(EmailKey), claims.GetEmail())
	ctx.Locals(string(TokenIDKey), claims.GetTokenID())
}

func SetUserIDToFiberCtx(ctx *fiber.Ctx, userID string) {
	ctx.Locals(string(UserIDKey), userID)
}

func SetUserEmailToFiberCtx(ctx *fiber.Ctx, email string) {
	ctx.Locals(string(EmailKey), email)
}

func SetUserIsAdminToFiberCtx(ctx *fiber.Ctx, isAdmin bool) {
	ctx.Locals(string(IsAdminKey), isAdmin)
}

func NewContextFromFiberCtx(fiberCtx *fiber.Ctx) context.Context {
	userID := GetUserIdFromFiberCtx(fiberCtx)
	email := GetUserEmailFromFiberCtx(fiberCtx)
	tokenID := GetTokenIDFromFiberCtx(fiberCtx)
	userAgent := GetUserAgentFromFiberCtx(fiberCtx)
	clientIP := GetClientIPFromFiberCtx(fiberCtx)
	isAdmin := GetUserIsAdminFromFiberCtx(fiberCtx)

	ctx := context.Background()
	ctx = context.WithValue(ctx, UserIDKey, userID)
	ctx = context.WithValue(ctx, EmailKey, email)
	ctx = context.WithValue(ctx, TokenIDKey, tokenID)
	ctx = context.WithValue(ctx, UserAgentKey, userAgent)
	ctx = context.WithValue(ctx, ClientIPKey, clientIP)
	ctx = context.WithValue(ctx, IsAdminKey, isAdmin)
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

func GetIsAdmin(ctx context.Context) bool {
	isAdmin := ctx.Value(IsAdminKey)
	if isAdmin == nil {
		return false
	}
	if isAdmin, ok := isAdmin.(bool); ok {
		return isAdmin
	}
	return false
}

func GetUserIdFromFiberCtx(ctx *fiber.Ctx) string {
	val := ctx.Locals(string(UserIDKey))
	if val == nil {
		return ""
	}
	return val.(string)
}

func GetUserEmailFromFiberCtx(ctx *fiber.Ctx) string {
	val := ctx.Locals(string(EmailKey))
	if val == nil {
		return ""
	}
	return val.(string)
}

func GetTokenIDFromFiberCtx(ctx *fiber.Ctx) string {
	val := ctx.Locals(string(TokenIDKey))
	if val == nil {
		return ""
	}
	return val.(string)
}

func GetUserAgentFromFiberCtx(ctx *fiber.Ctx) string {
	return ctx.Get("User-Agent")
}

func GetClientIPFromFiberCtx(ctx *fiber.Ctx) string {
	return ctx.IP()
}

func GetUserIsAdminFromFiberCtx(ctx *fiber.Ctx) bool {
	val := ctx.Locals(string(IsAdminKey))
	if val == nil {
		return false
	}
	if isAdmin, ok := val.(bool); ok {
		return isAdmin
	}
	return false
}

func SetDiContainerRequestToFiberCtx(ctx *fiber.Ctx, request di.Container) {
	ctx.Locals(string(DiContainerRequestKey), request)
}

func GetDiContainerRequestFromFiberCtx(ctx *fiber.Ctx) di.Container {
	container := ctx.Locals(string(DiContainerRequestKey))
	if container == nil {
		return di.Container{}
	}
	if container, ok := container.(di.Container); ok {
		return container
	}
	return di.Container{}
}
