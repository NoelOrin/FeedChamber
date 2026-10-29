package service

import (
	"context"
	"io"
	"mime/multipart"

	"feedchamber/internal/model"
	"feedchamber/internal/storage"
)

type UploadService interface {
	// Save 服务端代理上传（本地存储模式）
	Save(ctx context.Context, userID string, fileHeader *multipart.FileHeader) (*model.Upload, error)
	// Presign 生成预签名 POST 信息（直传模式）
	Presign(ctx context.Context, userID, fileName, contentType string) (*storage.PresignedPost, *model.Upload, error)
	GetByID(ctx context.Context, id string) (*model.Upload, error)
	OpenByKey(ctx context.Context, key string) (io.ReadCloser, *model.Upload, error)
	ListByUser(ctx context.Context, userID string, page, pageSize int) ([]model.Upload, error)
	Delete(ctx context.Context, id, actorUUID string, isAdmin bool) error
}
