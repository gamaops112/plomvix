package auth

import (
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/plomvix/plomvix/internal/config"
	"github.com/plomvix/plomvix/internal/logger"
)

func BootstrapAdminUser(store *Store, cfg *config.Config) error {
	users, err := store.ListUsers()
	if err != nil {
		return err
	}
	if len(users) > 0 {
		return nil
	}

	passwordHash, err := HashPassword(cfg.Auth.DefaultAdminPassword)
	if err != nil {
		return err
	}

	_, apiKeyHash, err := GenerateAPIKey(cfg)
	if err != nil {
		return err
	}

	user := User{
		ID:           uuid.New().String(),
		Username:     cfg.Auth.DefaultAdminUsername,
		PasswordHash: passwordHash,
		Role:         RoleAdmin,
		APIKeyHash:   apiKeyHash,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := store.CreateUser(&user); err != nil {
		return err
	}

	logger.Info("default admin user created",
		zap.String("username", cfg.Auth.DefaultAdminUsername),
	)
	logger.Warn("default admin credentials are set to defaults — change before exposing to network")

	return nil
}
