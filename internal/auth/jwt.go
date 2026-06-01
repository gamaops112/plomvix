package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/plomvix/plomvix/internal/config"
)

type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Role     Role   `json:"role"`
	JTI      string `json:"jti"`
	jwt.RegisteredClaims
}

func GenerateTokenWithClaims(user *User, cfg *config.Config) (string, *Claims, error) {
	now := time.Now()
	claims := &Claims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		JTI:      uuid.New().String(),
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(
				now.Add(time.Duration(cfg.Auth.JWTExpirySeconds) * time.Second)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(cfg.Auth.JWTSecret))
	if err != nil {
		return "", nil, err
	}
	return signed, claims, nil
}

func GenerateToken(user *User, cfg *config.Config) (string, error) {
	token, _, err := GenerateTokenWithClaims(user, cfg)
	return token, err
}

func ParseToken(tokenString string, cfg *config.Config) (*Claims, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&Claims{},
		func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v",
					token.Header["alg"])
			}
			return []byte(cfg.Auth.JWTSecret), nil
		},
	)
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}
	return claims, nil
}
