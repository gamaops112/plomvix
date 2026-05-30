package auth

import (
	"crypto/rand"
	"encoding/base64"

	"github.com/plomvix/plomvix/internal/config"
)

func GenerateAPIKey(cfg *config.Config) (plaintext string, hash string, err error) {
	b := make([]byte, cfg.Auth.APIKeyLength)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	plaintext = base64.RawURLEncoding.EncodeToString(b)
	hash, err = HashPassword(plaintext)
	if err != nil {
		return "", "", err
	}
	return plaintext, hash, nil
}

func CheckAPIKey(plaintext, hash string) error {
	return CheckPassword(plaintext, hash)
}

func FindUserByAPIKey(store *Store, plaintext string) (*User, error) {
	users, err := store.ListUsers()
	if err != nil {
		return nil, err
	}
	for _, u := range users {
		if u.APIKeyHash == "" {
			continue
		}
		if CheckAPIKey(plaintext, u.APIKeyHash) == nil {
			return u, nil
		}
	}
	return nil, ErrUserNotFound
}
