package storage

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
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
	if err := os.MkdirAll(filepath.Dir(s.path(key)), 0o755); err != nil {
		return fmt.Errorf("create uploads dir: %w", err)
	}
	path := s.path(key)
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(f, reader); err != nil {
		// io.Copy failed — remove the residual empty/partial file
		_ = os.Remove(path)
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

func (s *LocalStorage) List(_ context.Context, prefix string) ([]ObjectInfo, error) {
	root := s.dir
	items := make([]ObjectInfo, 0)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if prefix != "" && !strings.HasPrefix(rel, prefix) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		items = append(items, ObjectInfo{Key: rel, Size: info.Size(), ModifiedAt: info.ModTime()})
		return nil
	})
	if os.IsNotExist(err) {
		return []ObjectInfo{}, nil
	}
	return items, err
}

func (s *LocalStorage) PresignPostURL(_ context.Context, _ string, _ string, _ int64, _ time.Duration) (*PresignedPost, error) {
	return nil, fmt.Errorf("presign POST is not supported for local storage")
}

func (s *LocalStorage) PublicURL(key string) string {
	return "/api/v1/upload/file/" + key
}
