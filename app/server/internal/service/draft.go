package service

import (
	"context"
	"encoding/json"
	"time"

	"feedchamber/internal/model"
	"feedchamber/internal/pkg"
	"feedchamber/internal/repository"
)

type draftService struct {
	repo repository.DraftRepository
}

func NewDraftService(repo repository.DraftRepository) DraftService {
	return &draftService{repo: repo}
}

func (s *draftService) Save(ctx context.Context, userID, kind, targetID, title, body, spaceID string, tags []string) (*model.Draft, error) {
	if userID == "" || kind == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "user_id 和 kind 不能为空")
	}
	if kind != "post" && kind != "reply" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "kind 必须为 post 或 reply")
	}
	raw, _ := json.Marshal(tags)
	draft := &model.Draft{
		UserID:    userID,
		Kind:      kind,
		TargetID:  targetID,
		Title:     title,
		Body:      body,
		SpaceID:   spaceID,
		Tags:      string(raw),
		UpdatedAt: time.Now(),
	}
	return s.repo.Upsert(ctx, draft)
}

func (s *draftService) List(ctx context.Context, userID string, page, pageSize int) ([]model.Draft, int, error) {
	page, pageSize = pkg.NormalizePagination(page, pageSize)
	return s.repo.ListByUser(ctx, userID, (page-1)*pageSize, pageSize)
}

func (s *draftService) Delete(ctx context.Context, userID, id string) error {
	if id == "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "id 不能为空")
	}
	return s.repo.Delete(ctx, id, userID)
}
