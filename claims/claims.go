package claims

import (
	"slices"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/vukyn/kuery/cryp"
)

const (
	TokenIDKey        = "jti"
	UserIDKey         = "uid"
	EmailKey          = "email"
	ExpiredAtKey      = "exp"
	AudienceKey       = "aud"
	ResourceAccessKey = "resource_access"
)

// AppAccess is the per-app access block stored under resource_access[app_code].
// Only permission codes are carried in the token (no roles claim).
type AppAccess struct {
	Perms []string `json:"perms"`
}

type Claims struct {
	jwt.MapClaims
}

func NewClaims(userID, email string, expireIn int64) Claims {
	return Claims{
		jwt.MapClaims{
			TokenIDKey:   cryp.ULID(),
			UserIDKey:    userID,
			EmailKey:     email,
			ExpiredAtKey: time.Now().Add(time.Duration(expireIn) * time.Second).Unix(),
		},
	}
}

// WithAudience restricts the token to the given app codes (the "aud" claim).
func (c Claims) WithAudience(appCodes []string) Claims {
	c.MapClaims[AudienceKey] = appCodes
	return c
}

// WithResourceAccess attaches per-app permission codes to the claims, stored as a
// nested map ({"<app_code>":{"perms":[...]}}) so the JWT round-trip stays lossless.
func (c Claims) WithResourceAccess(access map[string][]string) Claims {
	resourceAccess := make(map[string]any, len(access))
	for appCode, perms := range access {
		resourceAccess[appCode] = map[string]any{
			"perms": perms,
		}
	}
	c.MapClaims[ResourceAccessKey] = resourceAccess
	return c
}

func (c Claims) GetTokenID() string {
	val := c.MapClaims[TokenIDKey]
	if val == nil {
		return ""
	}
	return val.(string)
}

func (c Claims) GetUserID() string {
	val := c.MapClaims[UserIDKey]
	if val == nil {
		return ""
	}
	return val.(string)
}

func (c Claims) GetEmail() string {
	val := c.MapClaims[EmailKey]
	if val == nil {
		return ""
	}
	return val.(string)
}

func (c Claims) GetExpiredAt() time.Time {
	switch val := c.MapClaims[ExpiredAtKey].(type) {
	case int64:
		return time.Unix(val, 0)
	case float64:
		return time.Unix(int64(val), 0)
	}
	return time.Time{}
}

func (c Claims) IsExpired() bool {
	return c.GetExpiredAt().Before(time.Now())
}

// GetAudience returns the app codes the token is valid for. Per the JWT spec the
// "aud" claim may be encoded as a single string or an array, so both are accepted.
func (c Claims) GetAudience() []string {
	val := c.MapClaims[AudienceKey]
	if str, ok := val.(string); ok {
		return []string{str}
	}
	return toStringSlice(val)
}

// GetPermsForApp returns the permission codes granted for the given app code.
func (c Claims) GetPermsForApp(appCode string) []string {
	access := toAppAccessMap(c.MapClaims[ResourceAccessKey])
	return access[appCode]
}

// HasPermForApp reports whether the caller holds the given permission for the app.
func (c Claims) HasPermForApp(appCode, perm string) bool {
	return slices.Contains(c.GetPermsForApp(appCode), perm)
}

// toStringSlice normalizes claim values to []string
// (JSON decoding turns arrays into []any)
func toStringSlice(val any) []string {
	switch values := val.(type) {
	case []string:
		return values
	case []any:
		result := make([]string, 0, len(values))
		for _, value := range values {
			if str, ok := value.(string); ok {
				result = append(result, str)
			}
		}
		return result
	}
	return nil
}

// toAppAccessMap defensively decodes the resource_access claim into a
// {app_code: perms} map. The claim is stored as a nested map of
// {app_code: {"perms": [...]}}; JSON decoding turns the inner objects into
// map[string]any and the perm arrays into []any, so both shapes are handled.
func toAppAccessMap(val any) map[string][]string {
	outer, ok := val.(map[string]any)
	if !ok {
		return nil
	}
	result := make(map[string][]string, len(outer))
	for appCode, inner := range outer {
		innerMap, ok := inner.(map[string]any)
		if !ok {
			continue
		}
		result[appCode] = toStringSlice(innerMap["perms"])
	}
	return result
}
