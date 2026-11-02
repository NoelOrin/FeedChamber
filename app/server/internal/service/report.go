package service

import (
	"context"
	"errors"
	"feedchamber/internal/model"
	"feedchamber/internal/pkg"
	"feedchamber/internal/pkg/eventbus"
	"feedchamber/internal/repository"
	"time"
)

type reportService struct {
	repo        repository.ReportRepository
	postRepo    repository.PostRepository
	contentRepo repository.ContentRepository
	banSvc      BannedService
	bus         *eventbus.EventBus
}

func NewReportService(repo repository.ReportRepository, postRepo repository.PostRepository, contentRepo repository.ContentRepository, banSvc BannedService, bus *eventbus.EventBus) ReportService {
	return &reportService{repo: repo, postRepo: postRepo, contentRepo: contentRepo, banSvc: banSvc, bus: bus}
}

func (s *reportService) Create(ctx context.Context, reporterID, targetType, targetID, flagType, reason string) (*model.Report, error) {
	if targetType != "post" && targetType != "comment" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "targetType必须为post或comment")
	}
	if existing, err := s.repo.FindPendingByTarget(ctx, targetType, targetID); err == nil && existing != nil {
		if err := s.repo.IncrementCount(ctx, existing.ID, reporterID); err != nil {
			return nil, err
		}
		return existing, nil
	}
	title, body := s.resolveTargetSnapshot(ctx, targetType, targetID)
	report := &model.Report{
		ID:          pkg.NewID(),
		ReporterID:  reporterID,
		TargetType:  targetType,
		TargetID:    targetID,
		FlagType:    flagType,
		Reason:      reason,
		TargetTitle: title,
		TargetBody:  body,
		Count:       1,
		Status:      "pending",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := s.repo.Create(ctx, report); err != nil {
		return nil, err
	}
	if s.bus != nil {
		s.bus.Publish(eventbus.Event{Type: eventbus.EventReportCreated, Ctx: ctx, Payload: map[string]any{
			"report_id": report.ID, "reporter_id": reporterID, "target_type": targetType, "target_id": targetID,
		}})
	}
	return report, nil
}

func (s *reportService) resolveTargetSnapshot(ctx context.Context, targetType, targetID string) (string, string) {
	switch targetType {
	case "post":
		if s.postRepo == nil {
			return "", ""
		}
		p, err := s.postRepo.FindByID(ctx, targetID)
		if err != nil {
			return "", ""
		}
		body := p.ContentDigest
		if s.contentRepo != nil {
			if c, err := s.contentRepo.FindByID(ctx, p.ContentID); err == nil && c != nil {
				body = c.Body
			}
		}
		return p.Title, body
	case "comment":
		if s.contentRepo == nil {
			return "", ""
		}
		c, err := s.contentRepo.FindByID(ctx, targetID)
		if err != nil {
			return "", ""
		}
		title := ""
		if s.postRepo != nil {
			if p, err := s.postRepo.FindByContentID(ctx, c.RootID); err == nil && p != nil {
				title = p.Title
			}
		}
		return title, c.Body
	default:
		return "", ""
	}
}

func (s *reportService) GetByID(ctx context.Context, id string) (*model.Report, error) {
	report, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, pkg.NewAppError(pkg.NOT_FOUND, "举报不存在")
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return report, nil
}

func (s *reportService) List(ctx context.Context, status string, page, pageSize int) ([]model.Report, int64, error) {
	page, pageSize = pkg.NormalizePagination(page, pageSize)
	offset := (page - 1) * pageSize
	var (
		items []model.Report
		total int64
		err   error
	)
	// repository.StatusFilterAll（"__all__"）是「不过滤状态」哨兵：后台举报列表
	// 默认传它，若当作字面状态查库会恒 0 行。
	if status != "" && status != repository.StatusFilterAll {
		items, err = s.repo.ListByStatus(ctx, status, offset, pageSize)
		if err == nil {
			total, err = s.repo.CountByStatus(ctx, status)
		}
	} else {
		items, err = s.repo.List(ctx, offset, pageSize)
		if err == nil {
			total, err = s.repo.Count(ctx)
		}
	}
	return items, total, err
}

func (s *reportService) UpdateStatus(ctx context.Context, id, status, action, actorID, note string) error {
	if status != "pending" && status != "resolved" && status != "dismissed" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "status必须为pending、resolved或dismissed")
	}
	r, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return pkg.NewAppError(pkg.NOT_FOUND, "举报不存在")
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if status == "resolved" {
		switch action {
		case "", "none":
			// no-op
		case "delete_target":
			if err := s.deleteTarget(ctx, r); err != nil {
				return err
			}
		case "lock_post":
			if err := s.lockPostTarget(ctx, r); err != nil {
				return err
			}
		case "ban_user":
			if err := s.banTargetAuthor(ctx, r, actorID, note); err != nil {
				return err
			}
		default:
			return pkg.NewAppError(pkg.INVALID_PARAMS, "unknown action")
		}
	}
	return s.repo.UpdateStatus(ctx, id, status, actorID, action, note)
}

func (s *reportService) deleteTarget(ctx context.Context, r *model.Report) error {
	switch r.TargetType {
	case "post":
		if s.postRepo == nil {
			return pkg.NewAppError(pkg.INTERNAL_ERROR, "post repository unavailable")
		}
		return s.postRepo.Delete(ctx, r.TargetID)
	case "comment":
		if s.contentRepo == nil {
			return pkg.NewAppError(pkg.INTERNAL_ERROR, "content repository unavailable")
		}
		return s.contentRepo.Delete(ctx, r.TargetID)
	default:
		return pkg.NewAppError(pkg.INVALID_PARAMS, "unsupported target type")
	}
}

func (s *reportService) lockPostTarget(ctx context.Context, r *model.Report) error {
	if s.postRepo == nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, "post repository unavailable")
	}
	postID := r.TargetID
	if r.TargetType == "comment" {
		c, err := s.contentRepo.FindByID(ctx, r.TargetID)
		if err != nil {
			return pkg.NewAppError(pkg.NOT_FOUND, "评论不存在")
		}
		p, err := s.postRepo.FindByContentID(ctx, c.RootID)
		if err != nil {
			return pkg.NewAppError(pkg.NOT_FOUND, "帖子不存在")
		}
		postID = p.ID
	} else if r.TargetType != "post" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "lock_post only supports post/comment targets")
	}
	post, err := s.postRepo.FindByID(ctx, postID)
	if err != nil {
		return pkg.NewAppError(pkg.NOT_FOUND, "帖子不存在")
	}
	if post.Locked {
		return nil
	}
	_, err = s.postRepo.ToggleLocked(ctx, postID)
	return err
}

func (s *reportService) banTargetAuthor(ctx context.Context, r *model.Report, actorID, note string) error {
	if s.banSvc == nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, "ban service unavailable")
	}
	authorID, err := s.resolveAuthorID(ctx, r)
	if err != nil {
		return err
	}
	reason := note
	if reason == "" {
		reason = "举报处理封禁: " + r.Reason
	}
	_, err = s.banSvc.Ban(ctx, authorID, reason, actorID, true, nil)
	return err
}

func (s *reportService) resolveAuthorID(ctx context.Context, r *model.Report) (string, error) {
	switch r.TargetType {
	case "post":
		p, err := s.postRepo.FindByID(ctx, r.TargetID)
		if err != nil {
			return "", pkg.NewAppError(pkg.NOT_FOUND, "帖子不存在")
		}
		return p.AuthorID, nil
	case "comment":
		c, err := s.contentRepo.FindByID(ctx, r.TargetID)
		if err != nil {
			return "", pkg.NewAppError(pkg.NOT_FOUND, "评论不存在")
		}
		return c.AuthorID, nil
	default:
		return "", pkg.NewAppError(pkg.INVALID_PARAMS, "unsupported target type")
	}
}

func (s *reportService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}
