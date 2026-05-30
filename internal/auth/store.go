package auth

import (
	"encoding/json"
	"errors"
	"sort"
	"time"

	bolt "go.etcd.io/bbolt"
)

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrUserAlreadyExists = errors.New("username already exists")
)

var usersBucket = []byte("users")

type Store struct {
	db *bolt.DB
}

func NewStore(path string) (*Store, error) {
	db, err := bolt.Open(path, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, err
	}
	if err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(usersBucket)
		return err
	}); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) CreateUser(u *User) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(usersBucket)
		cursor := b.Cursor()
		for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
			var existing User
			if err := json.Unmarshal(v, &existing); err != nil {
				continue
			}
			if existing.Username == u.Username {
				return ErrUserAlreadyExists
			}
		}
		data, err := json.Marshal(u)
		if err != nil {
			return err
		}
		return b.Put([]byte(u.ID), data)
	})
}

func (s *Store) GetUserByID(id string) (*User, error) {
	var user User
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(usersBucket)
		v := b.Get([]byte(id))
		if v == nil {
			return ErrUserNotFound
		}
		return json.Unmarshal(v, &user)
	})
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *Store) GetUserByUsername(username string) (*User, error) {
	var user User
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(usersBucket)
		cursor := b.Cursor()
		for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
			var u User
			if err := json.Unmarshal(v, &u); err != nil {
				continue
			}
			if u.Username == username {
				user = u
				return nil
			}
		}
		return ErrUserNotFound
	})
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *Store) UpdateUser(u *User) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(usersBucket)
		v := b.Get([]byte(u.ID))
		if v == nil {
			return ErrUserNotFound
		}
		data, err := json.Marshal(u)
		if err != nil {
			return err
		}
		return b.Put([]byte(u.ID), data)
	})
}

func (s *Store) DeleteUser(id string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(usersBucket)
		v := b.Get([]byte(id))
		if v == nil {
			return ErrUserNotFound
		}
		return b.Delete([]byte(id))
	})
}

func (s *Store) ListUsers() ([]*User, error) {
	var users []*User
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(usersBucket)
		cursor := b.Cursor()
		for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
			var u User
			if err := json.Unmarshal(v, &u); err != nil {
				continue
			}
			users = append(users, &u)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if users == nil {
		users = make([]*User, 0)
	}
	sort.Slice(users, func(i, j int) bool {
		return users[i].CreatedAt.Before(users[j].CreatedAt)
	})
	return users, nil
}

func (s *Store) UserExists(username string) (bool, error) {
	exists := false
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(usersBucket)
		cursor := b.Cursor()
		for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
			var u User
			if err := json.Unmarshal(v, &u); err != nil {
				continue
			}
			if u.Username == username {
				exists = true
				break
			}
		}
		return nil
	})
	return exists, err
}
