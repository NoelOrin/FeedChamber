package service

import (
	"context"
	"errors"
	"time"

	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"orinforum/internal/repository"

	"gorm.io/gorm"
)

type AnnouncementService interface {
	Create(ctx context.Context, ann *model.Announcement) (*model.Announcement, error)
	Update(ctx context.Context, ann *model.Announcement) (*model.Announcement, error)
	Delete(ctx context.Context, id string) error
	GetByID(ctx context.Context, id string) (*model.Announcement, error)
	List(ctx context.Context, page, pageSize int) ([]model.Announcement, error)
	ListPublished(ctx context.Context, page, pageSize int) ([]model.Announcement, error)
	Publish(ctx context.Context, id string) error
	Unpublish(ctx context.Context, id string) error
	IncrementView(ctx context.Context, id string) error
}

type announcementService struct {
	repo repository.AnnouncementRepository
}

func NewAnnouncementService(repo repository.AnnouncementRepository) AnnouncementService {
	return &announcementService{repo: repo}
}

func (s *announcementService) Create(ctx context.Context, ann *model.Announcement) (*model.Announcement, error) {
	now := time.Now()
	ann.ID = pkg.NewID()
	ann.CreatedAt = now
	ann.UpdatedAt = now

	if ann.Status == "" {
		ann.Status = model.AnnStatusDraft
	}
	if !model.IsValidDisplayType(ann.DisplayType) {
		ann.DisplayType = model.DisplayTypeToast
	}
	if ann.Position == "" {
		ann.Position = model.DefaultPosition
	}

	if err := s.repo.Create(ctx, ann); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return ann, nil
}

func (s *announcementService) Update(ctx context.Context, ann *model.Announcement) (*model.Announcement, error) {
	existing, err := s.repo.FindByID(ctx, ann.ID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, pkg.NewAppError(pkg.NOT_FOUND, "公告不存在")
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	if ann.DisplayType != 0 && !model.IsValidDisplayType(ann.DisplayType) {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "无效的展示类型")
	}
	if ann.Position != "" && !model.IsValidPosition(ann.Position) {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "无效的位置")
	}

	existing.Title = ann.Title
	existing.Content = ann.Content
	existing.DisplayType = ann.DisplayType
	if ann.Position != "" {
		existing.Position = ann.Position
	}
	existing.IsPinned = ann.IsPinned
	existing.Sort = ann.Sort
	existing.ExpiresAt = ann.ExpiresAt
	if ann.Status != "" {
		existing.Status = ann.Status
	}
	existing.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return existing, nil
}

func (s *announcementService) Delete(ctx context.Context, id string) error {
	_, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return pkg.NewAppError(pkg.NOT_FOUND, "公告不存在")
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return s.repo.Delete(ctx, id)
}

func (s *announcementService) GetByID(ctx context.Context, id string) (*model.Announcement, error) {
	ann, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, pkg.NewAppError(pkg.NOT_FOUND, "公告不存在")
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return ann, nil
}

func (s *announcementService) List(ctx context.Context, page, pageSize int) ([]model.Announcement, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	list, err := s.repo.List(ctx, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return list, nil
}

func (s *announcementService) ListPublished(ctx context.Context, page, pageSize int) ([]model.Announcement, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	list, err := s.repo.ListPublished(ctx, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return list, nil
}

func (s *announcementService) Publish(ctx context.Context, id string) error {
	ann, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return pkg.NewAppError(pkg.NOT_FOUND, "公告不存在")
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	ann.Status = model.AnnStatusPublished
	ann.UpdatedAt = time.Now()
	return s.repo.Update(ctx, ann)
}

func (s *announcementService) Unpublish(ctx context.Context, id string) error {
	ann, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return pkg.NewAppError(pkg.NOT_FOUND, "公告不存在")
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	ann.Status = model.AnnStatusDraft
	ann.UpdatedAt = time.Now()
	return s.repo.Update(ctx, ann)
}

func (s *announcementService) IncrementView(ctx context.Context, id string) error {
	return s.repo.IncrementViewCount(ctx, id)
}
