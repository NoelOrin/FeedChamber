package service

import (
	"context"
	"errors"
	"time"

	"feedchamber/internal/db"
	"feedchamber/internal/model"
	"feedchamber/internal/pkg"
	"feedchamber/internal/repository"
)

type announcementService struct {
	repo  repository.AnnouncementRepository
	txMgr db.TransactionManager
}

func NewAnnouncementService(repo repository.AnnouncementRepository, txMgr db.TransactionManager) AnnouncementService {
	return &announcementService{repo: repo, txMgr: txMgr}
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
	var result *model.Announcement
	err := s.txMgr.WithTx(ctx, func(txCtx context.Context) error {
		existing, err := s.repo.FindByID(txCtx, ann.ID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return pkg.NewAppError(pkg.NOT_FOUND, "公告不存在")
			}
			return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}

		if ann.DisplayType != 0 && !model.IsValidDisplayType(ann.DisplayType) {
			return pkg.NewAppError(pkg.INVALID_PARAMS, "无效的展示类型")
		}
		if ann.Position != "" && !model.IsValidPosition(ann.Position) {
			return pkg.NewAppError(pkg.INVALID_PARAMS, "无效的位置")
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

		if err := s.repo.Update(txCtx, existing); err != nil {
			return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
		result = existing
		return nil
	})
	return result, err
}

func (s *announcementService) Delete(ctx context.Context, id string) error {
	return s.txMgr.WithTx(ctx, func(txCtx context.Context) error {
		_, err := s.repo.FindByID(txCtx, id)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return pkg.NewAppError(pkg.NOT_FOUND, "公告不存在")
			}
			return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
		return s.repo.Delete(txCtx, id)
	})
}

func (s *announcementService) GetByID(ctx context.Context, id string) (*model.Announcement, error) {
	ann, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, pkg.NewAppError(pkg.NOT_FOUND, "公告不存在")
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return ann, nil
}

func (s *announcementService) List(ctx context.Context, page, pageSize int) ([]model.Announcement, error) {
	page, pageSize = pkg.NormalizePagination(page, pageSize)
	list, err := s.repo.List(ctx, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return list, nil
}

func (s *announcementService) ListPublished(ctx context.Context, page, pageSize int) ([]model.Announcement, error) {
	page, pageSize = pkg.NormalizePagination(page, pageSize)
	list, err := s.repo.ListPublished(ctx, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return list, nil
}

func (s *announcementService) Publish(ctx context.Context, id string) error {
	return s.txMgr.WithTx(ctx, func(txCtx context.Context) error {
		ann, err := s.repo.FindByID(txCtx, id)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return pkg.NewAppError(pkg.NOT_FOUND, "公告不存在")
			}
			return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
		ann.Status = model.AnnStatusPublished
		ann.UpdatedAt = time.Now()
		return s.repo.Update(txCtx, ann)
	})
}

func (s *announcementService) Unpublish(ctx context.Context, id string) error {
	return s.txMgr.WithTx(ctx, func(txCtx context.Context) error {
		ann, err := s.repo.FindByID(txCtx, id)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return pkg.NewAppError(pkg.NOT_FOUND, "公告不存在")
			}
			return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
		ann.Status = model.AnnStatusDraft
		ann.UpdatedAt = time.Now()
		return s.repo.Update(txCtx, ann)
	})
}

func (s *announcementService) IncrementView(ctx context.Context, id string) error {
	return s.repo.IncrementViewCount(ctx, id)
}
