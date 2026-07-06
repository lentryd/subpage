package subpage

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var ErrInvalidToken = errors.New("invalid token")

// SessionClaims is the payload of the "session" cookie JWT.
type SessionClaims struct {
	SessionID string `json:"sessionId"`
	Su        string `json:"su"`
	jwt.RegisteredClaims
}

// NewSessionJWT signs a session cookie JWT valid for 33 minutes, matching
// RootService.generateJwtForCookie.
func NewSessionJWT(sessionID, su, secret string) (string, error) {
	claims := SessionClaims{
		SessionID: sessionID,
		Su:        su,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(33 * time.Minute)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

// VerifySessionJWT verifies the session cookie JWT's signature and expiry.
func VerifySessionJWT(token, secret string) (*SessionClaims, error) {
	var claims SessionClaims
	_, err := jwt.ParseWithClaims(token, &claims, func(t *jwt.Token) (any, error) {
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}))
	if err != nil {
		return nil, ErrInvalidToken
	}
	return &claims, nil
}
