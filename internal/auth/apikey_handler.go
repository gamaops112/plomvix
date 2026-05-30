package auth

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/plomvix/plomvix/internal/config"
	"github.com/plomvix/plomvix/pkg/utils"
)

type APIKeyHandler struct {
	store *Store
	cfg   *config.Config
}

func NewAPIKeyHandler(store *Store, cfg *config.Config) *APIKeyHandler {
	return &APIKeyHandler{store: store, cfg: cfg}
}

func (h *APIKeyHandler) Generate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	user, err := h.store.GetUserByID(id)
	if err != nil {
		utils.NotFound(w, r, "user not found")
		return
	}

	plaintext, hash, err := GenerateAPIKey(h.cfg)
	if err != nil {
		utils.InternalError(w, r, "failed to generate API key")
		return
	}

	user.APIKeyHash = hash
	user.UpdatedAt = time.Now()
	if err := h.store.UpdateUser(user); err != nil {
		utils.InternalError(w, r, "failed to save API key")
		return
	}

	utils.Created(w, r, map[string]interface{}{
		"api_key": plaintext,
		"user_id": user.ID,
		"message": "Store this key securely. It will not be shown again.",
	})
}

func (h *APIKeyHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	user, err := h.store.GetUserByID(id)
	if err != nil {
		utils.NotFound(w, r, "user not found")
		return
	}

	user.APIKeyHash = ""
	user.UpdatedAt = time.Now()
	if err := h.store.UpdateUser(user); err != nil {
		utils.InternalError(w, r, "failed to revoke API key")
		return
	}

	utils.OK(w, r, map[string]interface{}{"message": "API key revoked"})
}

func (h *APIKeyHandler) Status(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	user, err := h.store.GetUserByID(id)
	if err != nil {
		utils.NotFound(w, r, "user not found")
		return
	}

	utils.OK(w, r, map[string]interface{}{
		"has_api_key": user.APIKeyHash != "",
		"user_id":     user.ID,
	})
}
