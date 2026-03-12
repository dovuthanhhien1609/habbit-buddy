package api

import (
	"github.com/golang-jwt/jwt/v5"
)

// validateWSToken validates a JWT string and returns the userID (sub claim).
// Returns empty string if invalid.
func validateWSToken(tokenStr, secret string) string {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return ""
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return ""
	}
	sub, _ := claims["sub"].(string)
	return sub
}
