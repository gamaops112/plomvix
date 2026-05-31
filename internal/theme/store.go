package theme

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Store manages the file-backed global theme document.
type Store struct {
	path string
	mu   sync.RWMutex
}

func NewStore(path string) *Store {
	return &Store{path: path}
}

func (s *Store) Path() string {
	return s.path
}

// Load returns the current theme from disk, creating defaults if the file does not exist.
func (s *Store) Load() (*Theme, error) {
	s.mu.RLock()
	data, err := os.ReadFile(s.path)
	s.mu.RUnlock()

	if err == nil {
		var t Theme
		if uErr := json.Unmarshal(data, &t); uErr != nil {
			return nil, fmt.Errorf("failed to parse theme file %q: %w", s.path, uErr)
		}
		if vErr := Validate(&t); vErr != nil {
			return nil, vErr
		}
		return &t, nil
	}

	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to read theme file %q: %w", s.path, err)
	}

	return s.Reset()
}

// saveLocked writes the theme to disk. Must be called with s.mu held.
func (s *Store) saveLocked(t *Theme) (*Theme, error) {
	if err := Validate(t); err != nil {
		return nil, err
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create theme directory: %w", err)
	}
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal theme: %w", err)
	}
	data = append(data, '\n')

	tmpPath := filepath.Join(dir, filepath.Base(s.path)+".tmp")
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to write temp theme file: %w", err)
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		os.Remove(tmpPath)
		return nil, fmt.Errorf("failed to rename theme file: %w", err)
	}
	return t, nil
}

// Save validates and persists a theme to disk.
func (s *Store) Save(t *Theme) (*Theme, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked(t)
}

// Reset replaces the current theme with defaults and returns the saved theme.
func (s *Store) Reset() (*Theme, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t := DefaultTheme()
	return s.saveLocked(t)
}

// ExportJSON returns the current theme as indented JSON bytes.
func (s *Store) ExportJSON() ([]byte, error) {
	t, err := s.Load()
	if err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal theme: %w", err)
	}
	return append(data, '\n'), nil
}
