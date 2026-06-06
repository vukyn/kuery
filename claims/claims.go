package claims

import (
	"slices"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/vukyn/kuery/cryp"
)

const (
	TokenIDKey   = "jti"
	UserIDKey    = "uid"
	EmailKey     = "email"
	ExpiredAtKey = "exp"
	PermsKey     = "perms"
	RolesKey     = "roles"
	IsAdminKey   = "adm"
)

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

// WithPerms attaches permission codes to the claims
func (c Claims) WithPerms(perms []string) Claims {
	c.MapClaims[PermsKey] = perms
	return c
}

// WithRoles attaches role codes to the claims
func (c Claims) WithRoles(roles []string) Claims {
	c.MapClaims[RolesKey] = roles
	return c
}

// WithIsAdmin attaches the admin flag to the claims
func (c Claims) WithIsAdmin(isAdmin bool) Claims {
	c.MapClaims[IsAdminKey] = isAdmin
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

func (c Claims) GetPerms() []string {
	return toStringSlice(c.MapClaims[PermsKey])
}

func (c Claims) GetRoles() []string {
	return toStringSlice(c.MapClaims[RolesKey])
}

func (c Claims) GetIsAdmin() bool {
	val := c.MapClaims[IsAdminKey]
	if val == nil {
		return false
	}
	if isAdmin, ok := val.(bool); ok {
		return isAdmin
	}
	return false
}

func (c Claims) HasPerm(perm string) bool {
	return slices.Contains(c.GetPerms(), perm)
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
