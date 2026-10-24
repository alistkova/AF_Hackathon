package content

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Storage interface {
	Put(ctx context.Context, key string, data []byte) error
	Get(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error
}

type LocalStorage struct {
	root string
}

func NewLocalStorage(root string) *LocalStorage {
	return &LocalStorage{root: strings.TrimSpace(root)}
}

func (s *LocalStorage) Put(_ context.Context, key string, data []byte) error {
	path, err := s.resolve(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create storage dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", key, err)
	}
	return nil
}

func (s *LocalStorage) Get(_ context.Context, key string) ([]byte, error) {
	path, err := s.resolve(key)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("read %s: %w", key, err)
	}
	return data, nil
}

func (s *LocalStorage) Delete(_ context.Context, key string) error {
	path, err := s.resolve(key)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete %s: %w", key, err)
	}
	return nil
}

func (s *LocalStorage) resolve(key string) (string, error) {
	if s.root == "" {
		return "", fmt.Errorf("storage root is not configured")
	}
	trimmed := strings.TrimSpace(key)
	cleaned := filepath.Clean(trimmed)
	if cleaned == "." || cleaned == "" || trimmed == "" {
		return "", fmt.Errorf("invalid storage key %q", key)
	}
	if strings.HasPrefix(trimmed, "/") || strings.HasPrefix(trimmed, `\`) ||
		filepath.IsAbs(cleaned) || strings.Contains(cleaned, "..") || strings.Contains(cleaned, ":") {
		return "", fmt.Errorf("invalid storage key %q", key)
	}
	return filepath.Join(s.root, cleaned), nil
}
