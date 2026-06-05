package claims

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/vukyn/kuery/cryp"
)

const (
	TokenIDKey   = "jti"
	UserIDKey    = "uid"
	EmailKey     = "email"
	ExpiredAtKey = "exp"
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
