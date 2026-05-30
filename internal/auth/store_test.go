package auth

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := NewStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func makeUser(username string) *User {
	return &User{
		ID:           uuid.New().String(),
		Username:     username,
		PasswordHash: "hash",
		Role:         RoleAdmin,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

func TestStoreCreateAndGet(t *testing.T) {
	store := newTestStore(t)
	u := makeUser("alice")

	if err := store.CreateUser(u); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	got, err := store.GetUserByID(u.ID)
	if err != nil {
		t.Fatalf("GetUserByID failed: %v", err)
	}
	if got.ID != u.ID || got.Username != "alice" {
		t.Fatal("retrieved user does not match")
	}

	got2, err := store.GetUserByUsername("alice")
	if err != nil {
		t.Fatalf("GetUserByUsername failed: %v", err)
	}
	if got2.ID != u.ID {
		t.Fatal("GetUserByUsername returned wrong user")
	}

	if err := store.CreateUser(makeUser("alice")); err != ErrUserAlreadyExists {
		t.Fatalf("expected ErrUserAlreadyExists, got: %v", err)
	}

	if _, err := store.GetUserByID("nonexistent"); err != ErrUserNotFound {
		t.Fatalf("expected ErrUserNotFound, got: %v", err)
	}
	if _, err := store.GetUserByUsername("nonexistent"); err != ErrUserNotFound {
		t.Fatalf("expected ErrUserNotFound, got: %v", err)
	}
}

func TestStoreUpdate(t *testing.T) {
	store := newTestStore(t)
	u := makeUser("bob")
	store.CreateUser(u)

	u.Username = "bob-updated"
	if err := store.UpdateUser(u); err != nil {
		t.Fatalf("UpdateUser failed: %v", err)
	}

	got, _ := store.GetUserByID(u.ID)
	if got.Username != "bob-updated" {
		t.Fatalf("expected bob-updated, got: %s", got.Username)
	}

	ghost := &User{ID: "ghost"}
	if err := store.UpdateUser(ghost); err != ErrUserNotFound {
		t.Fatalf("expected ErrUserNotFound, got: %v", err)
	}
}

func TestStoreDelete(t *testing.T) {
	store := newTestStore(t)
	u := makeUser("carol")
	store.CreateUser(u)

	if err := store.DeleteUser(u.ID); err != nil {
		t.Fatalf("DeleteUser failed: %v", err)
	}
	if _, err := store.GetUserByID(u.ID); err != ErrUserNotFound {
		t.Fatalf("expected ErrUserNotFound after delete, got: %v", err)
	}
	if err := store.DeleteUser("ghost"); err != ErrUserNotFound {
		t.Fatalf("expected ErrUserNotFound for ghost, got: %v", err)
	}
}

func TestStoreList(t *testing.T) {
	store := newTestStore(t)
	users, err := store.ListUsers()
	if err != nil {
		t.Fatalf("ListUsers failed: %v", err)
	}
	if len(users) != 0 {
		t.Fatalf("expected empty list, got: %d", len(users))
	}

	u1 := makeUser("user1")
	u1.CreatedAt = time.Now().Add(-2 * time.Second)
	u2 := makeUser("user2")
	u2.CreatedAt = time.Now().Add(-1 * time.Second)
	u3 := makeUser("user3")
	u3.CreatedAt = time.Now()

	store.CreateUser(u1)
	store.CreateUser(u2)
	store.CreateUser(u3)

	users, err = store.ListUsers()
	if err != nil {
		t.Fatalf("ListUsers failed: %v", err)
	}
	if len(users) != 3 {
		t.Fatalf("expected 3 users, got: %d", len(users))
	}
	if users[0].Username != "user1" {
		t.Fatalf("expected user1 first, got: %s", users[0].Username)
	}
	if users[2].Username != "user3" {
		t.Fatalf("expected user3 last, got: %s", users[2].Username)
	}
}

func TestUserExists(t *testing.T) {
	store := newTestStore(t)
	u := makeUser("dave")
	store.CreateUser(u)

	exists, err := store.UserExists("dave")
	if err != nil {
		t.Fatalf("UserExists failed: %v", err)
	}
	if !exists {
		t.Fatal("expected UserExists=true for dave")
	}

	exists, err = store.UserExists("unknown")
	if err != nil {
		t.Fatalf("UserExists failed: %v", err)
	}
	if exists {
		t.Fatal("expected UserExists=false for unknown")
	}
}
