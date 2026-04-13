package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rogerrlee/tasks-watcher/internal/models"
)

// TokenClaims are the JWT claims for a user session.
type TokenClaims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

// GenerateToken creates a new JWT for a user, valid for duration.
func GenerateToken(userID string, secret string, duration time.Duration) (string, int64, error) {
	expiresAt := time.Now().Add(duration).Unix()
	claims := TokenClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Unix(expiresAt, 0)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "tasks-watcher",
			Subject:   userID,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(secret))
	return tokenStr, expiresAt, err
}

// ValidateToken parses and validates a JWT, returning the user ID if valid.
func ValidateToken(tokenStr, secret string) (userID string, valid bool) {
	token, err := jwt.ParseWithClaims(tokenStr, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return "", false
	}
	claims, ok := token.Claims.(*TokenClaims)
	if !ok || !token.Valid {
		return "", false
	}
	return claims.UserID, true
}

// HashToken produces a SHA-256 hash of a token (for denylist storage).
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// GenerateSession creates a user session with token.
func GenerateSession(userID string, secret string) (*models.Session, string, error) {
	token, expiresAt, err := GenerateToken(userID, secret, 24*time.Hour)
	if err != nil {
		return nil, "", err
	}
	s := &models.Session{
		UserID:    userID,
		TokenHash: HashToken(token),
		ExpiresAt: expiresAt,
	}
	return s, token, nil
}
