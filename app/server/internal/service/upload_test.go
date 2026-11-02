package service

import (
	"context"
	"errors"
	"testing"

	"feedchamber/internal/mock"
	"feedchamber/internal/model"
	"feedchamber/internal/pkg"

	"github.com/stretchr/testify/assert"
)

func newTestUploadService(repo *mock.UploadRepository, st *mock.StorageProvider, settings *mock.SettingService) UploadService {
	if settings == nil {
		settings = mock.NewSettingService()
	}
	return NewUploadService(repo, st, settings)
}

func TestUploadDelete_Success(t *testing.T) {
	repo := mock.NewUploadRepository()
	storage := &mock.StorageProvider{}
	svc := newTestUploadService(repo, storage, nil)

	upload := &model.Upload{
		ID:         "upload-1",
		Key:        "uploads/test.jpg",
		UploaderID: "user-1",
	}
	_ = repo.Create(context.Background(), upload)

	err := svc.Delete(context.Background(), "upload-1", "user-1", false)
	assert.NoError(t, err)
	assert.Contains(t, storage.DeleteKeys, "uploads/test.jpg")
}

func TestUploadDelete_NotOwner(t *testing.T) {
	repo := mock.NewUploadRepository()
	storage := &mock.StorageProvider{}
	svc := newTestUploadService(repo, storage, nil)

	upload := &model.Upload{
		ID:         "upload-1",
		Key:        "uploads/test.jpg",
		UploaderID: "user-1",
	}
	_ = repo.Create(context.Background(), upload)

	err := svc.Delete(context.Background(), "upload-1", "user-2", false)
	assert.Error(t, err)

	var appErr *pkg.AppError
	assert.True(t, errors.As(err, &appErr))
	assert.Equal(t, pkg.FORBIDDEN, appErr.Code)
	assert.Equal(t, "只能删除自己的上传文件", appErr.Message)
}

func TestUploadDelete_NotFound(t *testing.T) {
	repo := mock.NewUploadRepository()
	storage := &mock.StorageProvider{}
	svc := newTestUploadService(repo, storage, nil)

	err := svc.Delete(context.Background(), "nonexistent", "user-1", false)
	assert.Error(t, err)

	var appErr *pkg.AppError
	assert.True(t, errors.As(err, &appErr))
	assert.Equal(t, pkg.NOT_FOUND, appErr.Code)
}

func TestUploadDelete_RepoFindError(t *testing.T) {
	repo := mock.NewUploadRepository()
	repo.SetFindErr(errors.New("db error"))
	storage := &mock.StorageProvider{}
	svc := newTestUploadService(repo, storage, nil)

	err := svc.Delete(context.Background(), "upload-1", "user-1", false)
	assert.Error(t, err)

	var appErr *pkg.AppError
	assert.True(t, errors.As(err, &appErr))
	assert.Equal(t, pkg.INTERNAL_ERROR, appErr.Code)
}

func TestUploadDelete_RepoDeleteError(t *testing.T) {
	repo := mock.NewUploadRepository()
	repo.SetDeleteErr(errors.New("db error"))
	storage := &mock.StorageProvider{}
	svc := newTestUploadService(repo, storage, nil)

	upload := &model.Upload{
		ID:         "upload-1",
		Key:        "uploads/test.jpg",
		UploaderID: "user-1",
	}
	_ = repo.Create(context.Background(), upload)

	err := svc.Delete(context.Background(), "upload-1", "user-1", false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "delete upload record")
}

func TestUploadGetByID_NotFound(t *testing.T) {
	repo := mock.NewUploadRepository()
	storage := &mock.StorageProvider{}
	svc := newTestUploadService(repo, storage, nil)

	_, err := svc.GetByID(context.Background(), "nonexistent")
	assert.Error(t, err)

	var appErr *pkg.AppError
	assert.True(t, errors.As(err, &appErr))
	assert.Equal(t, pkg.NOT_FOUND, appErr.Code)
}

func TestUploadGetByID_Success(t *testing.T) {
	repo := mock.NewUploadRepository()
	storage := &mock.StorageProvider{}
	svc := newTestUploadService(repo, storage, nil)

	upload := &model.Upload{
		ID:         "upload-1",
		UploaderID: "user-1",
	}
	_ = repo.Create(context.Background(), upload)

	result, err := svc.GetByID(context.Background(), "upload-1")
	assert.NoError(t, err)
	assert.Equal(t, "upload-1", result.ID)
}

func TestUploadListByUser(t *testing.T) {
	repo := mock.NewUploadRepository()
	storage := &mock.StorageProvider{}
	svc := newTestUploadService(repo, storage, nil)

	u1 := &model.Upload{ID: "u1", UploaderID: "user-1"}
	u2 := &model.Upload{ID: "u2", UploaderID: "user-1"}
	u3 := &model.Upload{ID: "u3", UploaderID: "user-2"}
	_ = repo.Create(context.Background(), u1)
	_ = repo.Create(context.Background(), u2)
	_ = repo.Create(context.Background(), u3)

	result, err := svc.ListByUser(context.Background(), "user-1", 1, 10)
	assert.NoError(t, err)
	assert.Len(t, result, 2)
}

func TestUploadDelete_StorageDeleteFailureSilenced(t *testing.T) {
	repo := mock.NewUploadRepository()
	storage := &mock.StorageProvider{DeleteErr: errors.New("storage unavailable")}
	svc := newTestUploadService(repo, storage, nil)

	upload := &model.Upload{
		ID:         "upload-1",
		Key:        "uploads/test.jpg",
		UploaderID: "user-1",
	}
	_ = repo.Create(context.Background(), upload)

	// Storage delete failure must NOT propagate — it's silently ignored
	err := svc.Delete(context.Background(), "upload-1", "user-1", false)
	assert.NoError(t, err)
}

func TestUploadDelete_AdminBypass(t *testing.T) {
	repo := mock.NewUploadRepository()
	storage := &mock.StorageProvider{}
	svc := newTestUploadService(repo, storage, nil)

	upload := &model.Upload{
		ID:         "upload-1",
		Key:        "uploads/admin-test.jpg",
		UploaderID: "user-1",
	}
	_ = repo.Create(context.Background(), upload)

	// Admin (isAdmin=true) can delete another user's upload
	err := svc.Delete(context.Background(), "upload-1", "admin-uuid", true)
	assert.NoError(t, err)
	assert.Contains(t, storage.DeleteKeys, "uploads/admin-test.jpg")
}
