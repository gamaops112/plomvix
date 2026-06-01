package auth

import (
	"net/http"
	"strings"
)

type TokenSource string

const (
	TokenSourceNone   TokenSource = "none"
	TokenSourceBearer TokenSource = "bearer"
	TokenSourceCookie TokenSource = "cookie"
)

func TokenFromRequest(r *http.Request) (string, TokenSource) {
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
		if token != "" {
			return token, TokenSourceBearer
		}
	}

	cookie, err := r.Cookie(TokenCookieName)
	if err == nil && strings.TrimSpace(cookie.Value) != "" {
		return cookie.Value, TokenSourceCookie
	}

	return "", TokenSourceNone
}
