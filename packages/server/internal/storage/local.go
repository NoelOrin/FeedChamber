package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// LocalStorage 本地磁盘存储，兼容开发环境。
type LocalStorage struct {
	dir string
}

func NewLocalStorage(dir string) *LocalStorage {
	if dir == "" {
		dir = "uploads"
	}
	return &LocalStorage{dir: dir}
}

func (s *LocalStorage) path(key string) string {
	return filepath.Join(s.dir, key)
}

func (s *LocalStorage) Put(_ context.Context, key string, reader io.Reader, _ int64, _ string) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("create uploads dir: %w", err)
	}
	f, err := os.Create(s.path(key))
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(f, reader); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}

func (s *LocalStorage) Get(_ context.Context, key string) (io.ReadCloser, error) {
	f, err := os.Open(s.path(key))
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	return f, nil
}

func (s *LocalStorage) Delete(_ context.Context, key string) error {
	err := os.Remove(s.path(key))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (s *LocalStorage) PresignPostURL(_ context.Context, _ string, _ string, _ int64, _ time.Duration) (*PresignedPost, error) {
	return nil, fmt.Errorf("presign POST is not supported for local storage")
}

func (s *LocalStorage) PublicURL(key string) string {
	return "/api/v1/upload/file/" + key
}
