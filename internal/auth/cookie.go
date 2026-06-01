package auth

import (
	"net/http"
	"time"

	"github.com/plomvix/plomvix/internal/config"
)

const TokenCookieName = "plomvix_token"

func NewTokenCookie(token string, expires time.Time, cfg *config.Config) *http.Cookie {
	return &http.Cookie{
		Name:     TokenCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   cfg.IsProduction(),
		SameSite: http.SameSiteLaxMode,
		Expires:  expires,
		MaxAge:   cfg.Auth.JWTExpirySeconds,
	}
}

func NewClearTokenCookie(cfg *config.Config) *http.Cookie {
	return &http.Cookie{
		Name:     TokenCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   cfg.IsProduction(),
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	}
}
