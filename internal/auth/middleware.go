package auth

import (
	"github.com/plomvix/plomvix/internal/config"
	"github.com/plomvix/plomvix/pkg/utils"
	"net/http"
)

func Middleware(store *Store, blacklist *Blacklist, cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
				user, err := FindUserByAPIKey(store, apiKey)
				if err != nil {
					utils.Unauthorized(w, r, "invalid API key")
					return
				}
				next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), user)))
				return
			}

			tokenString, source := TokenFromRequest(r)
			if source != TokenSourceNone {
				claims, err := ParseToken(tokenString, cfg)
				if err != nil {
					utils.Unauthorized(w, r, "invalid or expired token")
					return
				}
				if blacklist.IsBlacklisted(claims.JTI) {
					utils.Unauthorized(w, r, "token has been revoked")
					return
				}
				user, err := store.GetUserByID(claims.UserID)
				if err != nil {
					utils.Unauthorized(w, r, "user not found")
					return
				}
				next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), user)))
				return
			}

			utils.Unauthorized(w, r, "authentication required")
		})
	}
}

func RequireAdmin() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := RequireUser(r.Context())
			if user.Role != RoleAdmin {
				utils.Forbidden(w, r, "admin role required")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
