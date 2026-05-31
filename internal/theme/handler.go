package theme

import (
	"encoding/json"
	"net/http"

	"github.com/plomvix/plomvix/pkg/utils"
)

// Handler serves theme API endpoints.
type Handler struct {
	store *Store
}

// NewHandler creates a theme Handler.
func NewHandler(store *Store) *Handler {
	return &Handler{store: store}
}

// GetTheme returns the current theme document.
func (h *Handler) GetTheme(w http.ResponseWriter, r *http.Request) {
	t, err := h.store.Load()
	if err != nil {
		utils.InternalError(w, r, "failed to load theme")
		return
	}
	utils.OK(w, r, t)
}

// UpdateTheme decodes, validates, and saves a complete theme document.
func (h *Handler) UpdateTheme(w http.ResponseWriter, r *http.Request) {
	var t Theme
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		utils.BadRequest(w, r, utils.CodeValidationFailed, "failed to decode theme body")
		return
	}
	saved, err := h.store.Save(&t)
	if err != nil {
		if _, ok := err.(*json.SyntaxError); ok {
			utils.BadRequest(w, r, utils.CodeValidationFailed, "invalid JSON in theme body")
			return
		}
		utils.BadRequest(w, r, utils.CodeValidationFailed, err.Error())
		return
	}
	utils.OK(w, r, saved)
}

// ResetTheme resets the theme to factory defaults.
func (h *Handler) ResetTheme(w http.ResponseWriter, r *http.Request) {
	t, err := h.store.Reset()
	if err != nil {
		utils.InternalError(w, r, "failed to reset theme")
		return
	}
	utils.OK(w, r, t)
}

// ExportTheme returns the current theme as a raw downloadable JSON file.
func (h *Handler) ExportTheme(w http.ResponseWriter, r *http.Request) {
	data, err := h.store.ExportJSON()
	if err != nil {
		utils.InternalError(w, r, "failed to export theme")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="plomvix-theme.json"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
