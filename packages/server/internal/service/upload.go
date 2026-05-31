package service

import (
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"orinforum/internal/repository"
)

const (
	maxUploadSize   = 10 << 20 // 10 MB
	uploadsDir      = "uploads"
)

var allowedImageTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/gif":  true,
	"image/webp": true,
}

type UploadService interface {
	Save(ctx context.Context, userID string, fileHeader *multipart.FileHeader) (*model.Upload, error)
	GetByID(ctx context.Context, id string) (*model.Upload, error)
	ListByUser(ctx context.Context, userID string, page, pageSize int) ([]model.Upload, error)
	Delete(ctx context.Context, id string) error
}

type uploadService struct {
	repo repository.UploadRepository
}

func NewUploadService(repo repository.UploadRepository) UploadService {
	return &uploadService{repo: repo}
}

func (s *uploadService) Save(ctx context.Context, userID string, fileHeader *multipart.FileHeader) (*model.Upload, error) {
	// Validate file size
	if fileHeader.Size > maxUploadSize {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "file size exceeds 10MB limit")
	}

	// Detect MIME type
	src, err := fileHeader.Open()
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, "failed to open uploaded file")
	}
	defer src.Close()

	buf := make([]byte, 512)
	n, err := src.Read(buf)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, "failed to read uploaded file")
	}
	buf = buf[:n]

	mimeType := http.DetectContentType(buf)
	// Normalize: some detections may include parameters like "image/jpeg; charset=binary"
	if idx := strings.IndexByte(mimeType, ';'); idx != -1 {
		mimeType = strings.TrimSpace(mimeType[:idx])
	}

	if !allowedImageTypes[mimeType] {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "only image files (jpg, png, gif, webp) are allowed")
	}

	// Ensure uploads directory exists
	if err := os.MkdirAll(uploadsDir, 0o755); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, "failed to create uploads directory")
	}

	// Generate UUID-based stored name, preserving extension
	ext := filepath.Ext(fileHeader.Filename)
	if ext == "" {
		// Fallback extension from MIME type
		switch mimeType {
		case "image/jpeg":
			ext = ".jpg"
		case "image/png":
			ext = ".png"
		case "image/gif":
			ext = ".gif"
		case "image/webp":
			ext = ".webp"
		}
	}
	storedName := pkg.NewID() + ext
	relativePath := filepath.Join(uploadsDir, storedName)

	// Write file to disk
	dst, err := os.Create(relativePath)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, "failed to save file to disk")
	}
	defer dst.Close()

	// Seek back to beginning of source and copy
	if _, err := src.Seek(0, 0); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, "failed to read uploaded file")
	}
	if _, err := dst.ReadFrom(src); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, "failed to write file to disk")
	}

	upload := &model.Upload{
		ID:         pkg.NewID(),
		FileName:   fileHeader.Filename,
		StoredName: storedName,
		Path:       relativePath,
		Size:       fileHeader.Size,
		MimeType:   mimeType,
		UploaderID: userID,
		CreatedAt:  time.Now(),
	}

	if err := s.repo.Create(ctx, upload); err != nil {
		// Clean up file on DB error
		_ = os.Remove(relativePath)
		return nil, fmt.Errorf("failed to save upload metadata: %w", err)
	}

	return upload, nil
}

func (s *uploadService) GetByID(ctx context.Context, id string) (*model.Upload, error) {
	upload, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, pkg.NewAppError(pkg.NOT_FOUND, "upload not found")
	}

	if _, err := os.Stat(upload.Path); os.IsNotExist(err) {
		return nil, pkg.NewAppError(pkg.NOT_FOUND, "file not found")
	}

	return upload, nil
}

func (s *uploadService) ListByUser(ctx context.Context, userID string, page, pageSize int) ([]model.Upload, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.ListByUser(ctx, userID, offset, pageSize)
}

func (s *uploadService) Delete(ctx context.Context, id string) error {
	// Find upload first so we can remove the file from disk
	upload, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return pkg.NewAppError(pkg.NOT_FOUND, "upload not found")
	}

	// Delete from database
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete upload record: %w", err)
	}

	// Best-effort delete file from disk
	_ = os.Remove(upload.Path)

	return nil
}
