package service

import (
	"context"
	"time"

	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"orinforum/internal/pkg/eventbus"
	"orinforum/internal/repository"
)

type watchService struct {
	repo    repository.WatchRepository
	tagRepo repository.TagRepository
	bus     *eventbus.EventBus
}

func NewWatchService(repo repository.WatchRepository, tagRepo repository.TagRepository, bus *eventbus.EventBus) WatchService {
	return &watchService{repo: repo, tagRepo: tagRepo, bus: bus}
}

func validTargetType(t string) bool {
	switch t {
	case model.WatchTargetPost, model.WatchTargetTag:
		return true
	default:
		return false
	}
}

func validLevel(l string) bool {
	switch l {
	case model.WatchLevelWatching, model.WatchLevelTracking, model.WatchLevelMuted:
		return true
	default:
		return false
	}
}

func (s *watchService) Set(ctx context.Context, userID, targetType, targetID, level string) (*model.Watch, error) {
	if userID == "" || targetID == "" || !validTargetType(targetType) {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "无效的关注目标")
	}
	if level == "" {
		level = model.WatchLevelWatching
	}
	if !validLevel(level) {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "无效的关注级别")
	}
	w := &model.Watch{
		ID:         pkg.NewID(),
		UserID:     userID,
		TargetType: targetType,
		TargetID:   targetID,
		Level:      level,
		CreatedAt:  time.Now(),
	}
	if err := s.repo.Upsert(ctx, w); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return w, nil
}

func (s *watchService) Delete(ctx context.Context, userID, targetType, targetID string) error {
	if userID == "" || targetID == "" || !validTargetType(targetType) {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "无效的关注目标")
	}
	if err := s.repo.Delete(ctx, userID, targetType, targetID); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nil
}

func (s *watchService) Get(ctx context.Context, userID, targetType, targetID string) (*model.Watch, error) {
	w, err := s.repo.Get(ctx, userID, targetType, targetID)
	if err != nil {
		if err == repository.ErrNotFound {
			return nil, nil
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return w, nil
}

func (s *watchService) List(ctx context.Context, userID string, page, pageSize int) ([]model.Watch, int, error) {
	page, pageSize = pkg.NormalizePagination(page, pageSize)
	items, err := s.repo.ListByUser(ctx, userID, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, 0, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	total, err := s.repo.CountByUser(ctx, userID)
	if err != nil {
		return nil, 0, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return items, int(total), nil
}

func (s *watchService) NotifyWatchers(ctx context.Context, postID, authorID, senderName, postTitle string) {
	if s.repo == nil || s.bus == nil || postID == "" {
		return
	}
	seen := map[string]struct{}{}
	add := func(ids []string) {
		for _, id := range ids {
			if id == "" || id == authorID {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
		}
	}

	if ids, err := s.repo.ListActiveUserIDs(ctx, model.WatchTargetPost, postID); err == nil {
		add(ids)
	}

	// 标签关注者。版面关注（space）已于 2026-10-28 移除，别再补回来：
	// 它需要先查 post 拿 SpaceID，去掉后 postRepo 这个依赖也就成了纯多余（已从构造函数摘掉）。
	if s.tagRepo != nil {
		if tags, err := s.tagRepo.GetTagsByPostID(ctx, postID); err == nil {
			for _, t := range tags {
				if ids, err := s.repo.ListActiveUserIDs(ctx, model.WatchTargetTag, t.ID); err == nil {
					add(ids)
				}
			}
		}
	}

	for userID := range seen {
		s.bus.Publish(eventbus.Event{
			Type: eventbus.EventContentWatched,
			Ctx:  ctx,
			Payload: map[string]any{
				"user_id":     userID,
				"sender_id":   authorID,
				"sender_name": senderName,
				"post_id":     postID,
				"post_title":  postTitle,
			},
		})
	}
}
