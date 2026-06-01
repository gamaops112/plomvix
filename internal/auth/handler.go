package auth

import (
	"encoding/json"
	"net/http"

	"github.com/plomvix/plomvix/internal/config"
	"github.com/plomvix/plomvix/pkg/utils"
)

type Handler struct {
	store     *Store
	blacklist *Blacklist
	cfg       *config.Config
}

func NewHandler(store *Store, blacklist *Blacklist, cfg *config.Config) *Handler {
	return &Handler{store: store, blacklist: blacklist, cfg: cfg}
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.BadRequest(w, r, utils.CodeValidationFailed, "invalid request body")
		return
	}
	if req.Username == "" || req.Password == "" {
		utils.BadRequest(w, r, utils.CodeValidationFailed, "username and password are required")
		return
	}

	user, err := h.store.GetUserByUsername(req.Username)
	if err != nil {
		utils.Unauthorized(w, r, "invalid username or password")
		return
	}
	if err := CheckPassword(req.Password, user.PasswordHash); err != nil {
		utils.Unauthorized(w, r, "invalid username or password")
		return
	}

	token, claims, err := GenerateTokenWithClaims(user, h.cfg)
	if err != nil {
		utils.InternalError(w, r, "failed to generate token")
		return
	}

	http.SetCookie(w, NewTokenCookie(token, claims.ExpiresAt.Time, h.cfg))

	utils.OK(w, r, map[string]interface{}{
		"token":      token,
		"expires_in": h.cfg.Auth.JWTExpirySeconds,
		"user":       user.ToResponse(),
	})
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	tokenString, source := TokenFromRequest(r)
	if source != TokenSourceNone {
		claims, err := ParseToken(tokenString, h.cfg)
		if err == nil {
			h.blacklist.Add(claims.JTI, claims.ExpiresAt.Time)
		}
	}

	http.SetCookie(w, NewClearTokenCookie(h.cfg))

	utils.OK(w, r, map[string]interface{}{
		"message": "logged out successfully",
	})
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	// Extract the cookie token directly — this endpoint is public so
	// auth middleware doesn't set the user in context.
	tokenString, source := TokenFromRequest(r)
	if source == TokenSourceNone {
		utils.Unauthorized(w, r, "no authentication token provided")
		return
	}

	claims, err := ParseTokenExpired(tokenString, h.cfg)
	if err != nil {
		utils.Unauthorized(w, r, "invalid or expired token")
		return
	}

	freshUser, err := h.store.GetUserByID(claims.UserID)
	if err != nil {
		utils.Unauthorized(w, r, "user not found")
		return
	}

	h.blacklist.Add(claims.JTI, claims.ExpiresAt.Time)

	newToken, newClaims, err := GenerateTokenWithClaims(freshUser, h.cfg)
	if err != nil {
		utils.InternalError(w, r, "failed to generate token")
		return
	}

	http.SetCookie(w, NewTokenCookie(newToken, newClaims.ExpiresAt.Time, h.cfg))

	utils.OK(w, r, map[string]interface{}{
		"token":      newToken,
		"expires_in": h.cfg.Auth.JWTExpirySeconds,
	})
}
