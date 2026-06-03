package service

import (
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"orinforum/internal/repository"
	"orinforum/internal/storage"
)

const (
	maxUploadSize = 10 << 20 // 10 MB
	presignExpiry = 15 * time.Minute
	uploadsPrefix = "uploads"
)

var allowedImageTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/gif":  true,
	"image/webp": true,
}

type UploadService interface {
	// Save 服务端代理上传（本地存储模式）
	Save(ctx context.Context, userID string, fileHeader *multipart.FileHeader) (*model.Upload, error)
	// Presign 生成预签名 POST 信息（直传模式）
	Presign(ctx context.Context, userID, fileName, contentType string) (*storage.PresignedPost, *model.Upload, error)
	// ConfirmConfirm 直传完成后，前端回调确认保存元数据
	Confirm(ctx context.Context, uploadID string) (*model.Upload, error)
	GetByID(ctx context.Context, id string) (*model.Upload, error)
	ListByUser(ctx context.Context, userID string, page, pageSize int) ([]model.Upload, error)
	Delete(ctx context.Context, id string) error
}

type uploadService struct {
	repo    repository.UploadRepository
	storage storage.Provider
}

func NewUploadService(repo repository.UploadRepository, st storage.Provider) UploadService {
	return &uploadService{repo: repo, storage: st}
}

func (s *uploadService) Save(ctx context.Context, userID string, fileHeader *multipart.FileHeader) (*model.Upload, error) {
	if fileHeader.Size > maxUploadSize {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "文件大小超过10MB限制")
	}

	mimeType, err := detectMIME(fileHeader)
	if err != nil {
		return nil, err
	}

	storedName := pkg.NewID() + extFromNameOrMIME(fileHeader.Filename, mimeType)
	key := storage.BuildKey(uploadsPrefix, storedName)

	src, err := fileHeader.Open()
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, "打开上传文件失败")
	}
	defer src.Close()

	if err := s.storage.Put(ctx, key, src, fileHeader.Size, mimeType); err != nil {
		return nil, fmt.Errorf("storage put: %w", err)
	}

	upload := &model.Upload{
		ID:         pkg.NewID(),
		FileName:   fileHeader.Filename,
		StoredName: storedName,
		Key:        key,
		Path:       key,
		URL:        s.storage.PublicURL(key),
		Size:       fileHeader.Size,
		MimeType:   mimeType,
		UploaderID: userID,
		CreatedAt:  time.Now(),
	}

	if err := s.repo.Create(ctx, upload); err != nil {
		_ = s.storage.Delete(ctx, key)
		return nil, fmt.Errorf("save upload metadata: %w", err)
	}

	return upload, nil
}

func (s *uploadService) Presign(ctx context.Context, userID, fileName, contentType string) (*storage.PresignedPost, *model.Upload, error) {
	if !allowedImageTypes[contentType] {
		return nil, nil, pkg.NewAppError(pkg.INVALID_PARAMS, "仅允许图片文件(jpg, png, gif, webp)")
	}

	storedName := pkg.NewID() + extFromNameOrMIME(fileName, contentType)
	key := storage.BuildKey(uploadsPrefix, storedName)

	post, err := s.storage.PresignPostURL(ctx, key, contentType, maxUploadSize, presignExpiry)
	if err != nil {
		return nil, nil, fmt.Errorf("presign post: %w", err)
	}

	upload := &model.Upload{
		ID:         pkg.NewID(),
		FileName:   fileName,
		StoredName: storedName,
		Key:        key,
		URL:        s.storage.PublicURL(key),
		Size:       0, // 直传模式下大小由 Confirm 更新
		MimeType:   contentType,
		UploaderID: userID,
		CreatedAt:  time.Now(),
	}

	if err := s.repo.Create(ctx, upload); err != nil {
		return nil, nil, fmt.Errorf("save upload metadata: %w", err)
	}

	return post, upload, nil
}

func (s *uploadService) Confirm(ctx context.Context, uploadID string) (*model.Upload, error) {
	upload, err := s.repo.FindByID(ctx, uploadID)
	if err != nil {
		return nil, pkg.NewAppError(pkg.NOT_FOUND, "上传记录不存在")
	}
	return upload, nil
}

func (s *uploadService) GetByID(ctx context.Context, id string) (*model.Upload, error) {
	upload, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, pkg.NewAppError(pkg.NOT_FOUND, "上传记录不存在")
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
	upload, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return pkg.NewAppError(pkg.NOT_FOUND, "上传记录不存在")
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete upload record: %w", err)
	}

	_ = s.storage.Delete(ctx, upload.Key)
	return nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

func detectMIME(fh *multipart.FileHeader) (string, error) {
	f, err := fh.Open()
	if err != nil {
		return "", pkg.NewAppError(pkg.INTERNAL_ERROR, "打开上传文件失败")
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if err != nil {
		return "", pkg.NewAppError(pkg.INTERNAL_ERROR, "读取上传文件失败")
	}

	mimeType := http.DetectContentType(buf[:n])
	if idx := strings.IndexByte(mimeType, ';'); idx != -1 {
		mimeType = strings.TrimSpace(mimeType[:idx])
	}

	if !allowedImageTypes[mimeType] {
		return "", pkg.NewAppError(pkg.INVALID_PARAMS, "仅允许图片文件(jpg, png, gif, webp)")
	}
	return mimeType, nil
}

func extFromNameOrMIME(name, mimeType string) string {
	if ext := filepath.Ext(name); ext != "" {
		return ext
	}
	switch mimeType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	}
	return ""
}
