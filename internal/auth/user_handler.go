package auth

import (
	"encoding/json"
	"net/http"
	"regexp"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/plomvix/plomvix/internal/config"
	"github.com/plomvix/plomvix/pkg/utils"
)

var usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,64}$`)

type UserHandler struct {
	store *Store
	cfg   *config.Config
}

func NewUserHandler(store *Store, cfg *config.Config) *UserHandler {
	return &UserHandler{store: store, cfg: cfg}
}

func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.BadRequest(w, r, utils.CodeValidationFailed, "invalid request body")
		return
	}

	var errs []string
	if !usernameRegex.MatchString(req.Username) {
		errs = append(errs,
			"username must be 3-64 characters, alphanumeric, underscore, or hyphen only")
	}
	if len(req.Password) < 8 {
		errs = append(errs, "password must be at least 8 characters")
	}
	if len(errs) > 0 {
		utils.BadRequest(w, r, utils.CodeValidationFailed, "validation failed", errs...)
		return
	}

	exists, err := h.store.UserExists(req.Username)
	if err != nil {
		utils.InternalError(w, r, "failed to check username")
		return
	}
	if exists {
		utils.Conflict(w, r, "username already exists")
		return
	}

	hash, err := HashPassword(req.Password)
	if err != nil {
		utils.InternalError(w, r, "failed to hash password")
		return
	}

	now := time.Now()
	user := &User{
		ID:           uuid.New().String(),
		Username:     req.Username,
		PasswordHash: hash,
		Role:         RoleAdmin,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := h.store.CreateUser(user); err != nil {
		utils.InternalError(w, r, "failed to create user")
		return
	}

	utils.Created(w, r, user.ToResponse())
}

func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	users, err := h.store.ListUsers()
	if err != nil {
		utils.InternalError(w, r, "failed to list users")
		return
	}
	responses := make([]UserResponse, len(users))
	for i, u := range users {
		responses[i] = u.ToResponse()
	}
	utils.OK(w, r, map[string]interface{}{
		"users": responses,
		"count": len(responses),
	})
}

func (h *UserHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	user, err := h.store.GetUserByID(id)
	if err != nil {
		utils.NotFound(w, r, "user not found")
		return
	}
	utils.OK(w, r, user.ToResponse())
}

func (h *UserHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	user, err := h.store.GetUserByID(id)
	if err != nil {
		utils.NotFound(w, r, "user not found")
		return
	}

	var req struct {
		Username *string `json:"username"`
		Password *string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.BadRequest(w, r, utils.CodeValidationFailed, "invalid request body")
		return
	}

	var errs []string

	if req.Username != nil {
		if !usernameRegex.MatchString(*req.Username) {
			errs = append(errs,
				"username must be 3-64 characters, alphanumeric, underscore, or hyphen only")
		}
	}
	if req.Password != nil && len(*req.Password) < 8 {
		errs = append(errs, "password must be at least 8 characters")
	}
	if len(errs) > 0 {
		utils.BadRequest(w, r, utils.CodeValidationFailed, "validation failed", errs...)
		return
	}

	if req.Username != nil && *req.Username != user.Username {
		exists, err := h.store.UserExists(*req.Username)
		if err != nil {
			utils.InternalError(w, r, "failed to check username")
			return
		}
		if exists {
			utils.Conflict(w, r, "username already exists")
			return
		}
		user.Username = *req.Username
	}

	if req.Password != nil {
		hash, err := HashPassword(*req.Password)
		if err != nil {
			utils.InternalError(w, r, "failed to hash password")
			return
		}
		user.PasswordHash = hash
	}

	user.UpdatedAt = time.Now()
	if err := h.store.UpdateUser(user); err != nil {
		utils.InternalError(w, r, "failed to update user")
		return
	}
	utils.OK(w, r, user.ToResponse())
}

func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	caller := RequireUser(r.Context())

	if id == caller.ID {
		utils.BadRequest(w, r, utils.CodeValidationFailed, "cannot delete your own account")
		return
	}

	if _, err := h.store.GetUserByID(id); err != nil {
		utils.NotFound(w, r, "user not found")
		return
	}

	users, err := h.store.ListUsers()
	if err != nil {
		utils.InternalError(w, r, "failed to list users")
		return
	}
	if len(users) == 1 {
		utils.BadRequest(w, r, utils.CodeValidationFailed, "cannot delete the last admin user")
		return
	}

	if err := h.store.DeleteUser(id); err != nil {
		utils.InternalError(w, r, "failed to delete user")
		return
	}
	utils.OK(w, r, map[string]interface{}{"message": "user deleted"})
}
