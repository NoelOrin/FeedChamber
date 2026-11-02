package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"feedchamber/internal/model"
	"feedchamber/internal/pkg"
	"feedchamber/internal/repository"
	"feedchamber/internal/storage"
)

const (
	maxUploadSize = 10 << 20 // 10 MB
	presignExpiry = 15 * time.Minute
	uploadsPrefix = "uploads"
)

// allowedExtensions maps allowed MIME types to their canonical file extension.
// Extensions not in this set are rejected to prevent uploading executable or
// HTML content disguised with an allowed MIME type.
var allowedExtensions = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

type uploadService struct {
	repo       repository.UploadRepository
	storage    storage.Provider
	settingSvc SettingService
}

func NewUploadService(repo repository.UploadRepository, st storage.Provider, settingSvc SettingService) UploadService {
	return &uploadService{repo: repo, storage: st, settingSvc: settingSvc}
}

func (s *uploadService) getMaxUploadSize(ctx context.Context) int64 {
	return int64(pkg.GetSettingInt(ctx, s.settingSvc, "upload_max_size", "UPLOAD_MAX_SIZE", 10<<20))
}

func (s *uploadService) getAllowedTypes(ctx context.Context) map[string]bool {
	val := pkg.GetSettingOrDefault(ctx, s.settingSvc, "upload_allowed_types", "UPLOAD_ALLOWED_TYPES", "image/jpeg,image/png,image/gif,image/webp")
	types := make(map[string]bool)
	for _, t := range strings.Split(val, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			types[t] = true
		}
	}
	return types
}

func (s *uploadService) Save(ctx context.Context, userID string, fileHeader *multipart.FileHeader) (*model.Upload, error) {
	maxSize := s.getMaxUploadSize(ctx)
	if fileHeader.Size > maxSize {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, fmt.Sprintf("文件大小超过 %dMB 限制", maxSize/(1<<20)))
	}

	mimeType, err := detectMIME(fileHeader)
	if err != nil {
		return nil, err
	}

	allowed := s.getAllowedTypes(ctx)
	if !allowed[mimeType] {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "不允许的文件类型")
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
	allowed := s.getAllowedTypes(ctx)
	if !allowed[contentType] {
		return nil, nil, pkg.NewAppError(pkg.INVALID_PARAMS, "不允许的文件类型")
	}

	// Validate that the file extension is consistent with the declared MIME type.
	// This prevents a client from passing content_type="image/jpeg" but
	// filename="evil.html" to bypass the extension check.
	if _, ok := allowedExtensions[contentType]; !ok {
		return nil, nil, pkg.NewAppError(pkg.INVALID_PARAMS, "不允许的文件类型")
	}

	maxSize := s.getMaxUploadSize(ctx)
	expirySec := pkg.GetSettingInt(ctx, s.settingSvc, "upload_presign_expiry", "", 900)
	expiry := time.Duration(expirySec) * time.Second

	storedName := pkg.NewID() + extFromNameOrMIME(fileName, contentType)
	key := storage.BuildKey(uploadsPrefix, storedName)

	post, err := s.storage.PresignPostURL(ctx, key, contentType, maxSize, expiry)
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

func (s *uploadService) GetByID(ctx context.Context, id string) (*model.Upload, error) {
	upload, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, pkg.NewAppError(pkg.NOT_FOUND, "上传记录不存在")
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return upload, nil
}

func (s *uploadService) OpenByKey(ctx context.Context, key string) (io.ReadCloser, *model.Upload, error) {
	upload, err := s.repo.FindByKey(ctx, key)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil, pkg.NewAppError(pkg.NOT_FOUND, "上传记录不存在")
		}
		return nil, nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	reader, err := s.storage.Get(ctx, upload.Key)
	if err != nil {
		return nil, nil, pkg.NewAppError(pkg.INTERNAL_ERROR, "读取文件失败")
	}
	return reader, upload, nil
}

func (s *uploadService) ListByUser(ctx context.Context, userID string, page, pageSize int) ([]model.Upload, error) {
	page, pageSize = pkg.NormalizePagination(page, pageSize)
	offset := (page - 1) * pageSize
	return s.repo.ListByUser(ctx, userID, offset, pageSize)
}

func (s *uploadService) Delete(ctx context.Context, id, actorUUID string, isAdmin bool) error {
	upload, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return pkg.NewAppError(pkg.NOT_FOUND, "上传记录不存在")
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	if !isAdmin && upload.UploaderID != actorUUID {
		return pkg.NewAppError(pkg.FORBIDDEN, "只能删除自己的上传文件")
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

	if !allowedImageTypes()[mimeType] {
		return "", pkg.NewAppError(pkg.INVALID_PARAMS, "仅允许图片文件(jpg, png, gif, webp)")
	}
	return mimeType, nil
}

// allowedImageTypes returns the set of allowed MIME types derived from allowedExtensions.
func allowedImageTypes() map[string]bool {
	m := make(map[string]bool, len(allowedExtensions))
	for k := range allowedExtensions {
		m[k] = true
	}
	return m
}

func extFromNameOrMIME(name, mimeType string) string {
	// Only allow extensions that match an allowed MIME type.
	// This prevents a client from setting filename="evil.html" with
	// content_type="image/jpeg" to bypass the extension check.
	canonical, ok := allowedExtensions[mimeType]
	if !ok {
		return ""
	}
	// If the filename extension matches the canonical one, use it; otherwise
	// force the canonical extension from the MIME type.
	ext := filepath.Ext(name)
	if ext == canonical || strings.EqualFold(ext, canonical) {
		return ext
	}
	return canonical
}
