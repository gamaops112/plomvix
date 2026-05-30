package auth

import (
	"encoding/json"
	"net/http"
	"strings"

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

	token, err := GenerateToken(user, h.cfg)
	if err != nil {
		utils.InternalError(w, r, "failed to generate token")
		return
	}

	utils.OK(w, r, map[string]interface{}{
		"token":      token,
		"expires_in": h.cfg.Auth.JWTExpirySeconds,
		"user":       user.ToResponse(),
	})
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	_ = RequireUser(r.Context())

	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := ParseToken(tokenString, h.cfg)
		if err == nil {
			h.blacklist.Add(claims.JTI, claims.ExpiresAt.Time)
		}
	}

	utils.OK(w, r, map[string]interface{}{
		"message": "logged out successfully",
	})
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	user := RequireUser(r.Context())

	freshUser, err := h.store.GetUserByID(user.ID)
	if err != nil {
		utils.InternalError(w, r, "failed to fetch user")
		return
	}

	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if claims, err := ParseToken(tokenString, h.cfg); err == nil {
			h.blacklist.Add(claims.JTI, claims.ExpiresAt.Time)
		}
	}

	newToken, err := GenerateToken(freshUser, h.cfg)
	if err != nil {
		utils.InternalError(w, r, "failed to generate token")
		return
	}

	utils.OK(w, r, map[string]interface{}{
		"token":      newToken,
		"expires_in": h.cfg.Auth.JWTExpirySeconds,
	})
}
