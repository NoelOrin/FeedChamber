package service

import (
	"context"
	"errors"

	"orinforum/internal/pkg"
	"orinforum/internal/pkg/eventbus"
	"orinforum/internal/repository"

	"github.com/sirupsen/logrus"
)

type likeService struct {
	repo        repository.LikeRepository
	contentRepo repository.ContentRepository
	postRepo    repository.PostLookupRepository
	userRepo    repository.UserRepository
	bus         *eventbus.EventBus
	badgeSvc    BadgeService
	trustSvc    TrustService
}

// SetTrustService 注入等级评估服务（容器装配后调用，与 PostService 同模式）。
func (s *likeService) SetTrustService(svc TrustService) { s.trustSvc = svc }

func NewLikeService(repo repository.LikeRepository, contentRepo repository.ContentRepository, postRepo repository.PostLookupRepository, userRepo repository.UserRepository, bus *eventbus.EventBus, badgeSvc BadgeService) LikeService {
	return &likeService{repo: repo, contentRepo: contentRepo, postRepo: postRepo, userRepo: userRepo, bus: bus, badgeSvc: badgeSvc}
}

func (s *likeService) ToggleContentLike(ctx context.Context, contentID, userID string) (bool, error) {
	content, err := s.contentRepo.FindByID(ctx, contentID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return false, pkg.NewAppError(pkg.NOT_FOUND, "评论或帖子不存在")
		}
		return false, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	liked, err := s.repo.ToggleContentLike(ctx, contentID, userID)
	if err != nil {
		return false, err
	}
	// G-02: 同步更新内容的 like_count 计数器
	// All counter updates must succeed; if any fails, rollback the toggle to avoid drift.
	var counterErr error
	if liked {
		if content != nil && content.AuthorID != userID {
			if s.badgeSvc != nil {
				s.badgeSvc.Evaluate(ctx, content.AuthorID, "like")
			}
		}
		if err := s.contentRepo.IncrementLikeCount(ctx, contentID); err != nil {
			counterErr = err
			logrus.Errorf("like: increment like_count drift for content %s: %v", contentID, err)
		} else if err := s.postRepo.IncrementLikeCount(ctx, content.RootID); err != nil {
			// content counter incremented but post counter failed — rollback content counter
			_ = s.contentRepo.DecrementLikeCount(ctx, contentID)
			counterErr = err
			logrus.Errorf("like: increment like_count drift for post %s: %v", content.RootID, err)
		}
	} else {
		if err := s.contentRepo.DecrementLikeCount(ctx, contentID); err != nil {
			counterErr = err
			logrus.Errorf("like: decrement like_count drift for content %s: %v", contentID, err)
		} else if err := s.postRepo.DecrementLikeCount(ctx, content.RootID); err != nil {
			// content counter decremented but post counter failed — rollback content counter
			_ = s.contentRepo.IncrementLikeCount(ctx, contentID)
			counterErr = err
			logrus.Errorf("like: decrement like_count drift for post %s: %v", content.RootID, err)
		}
	}

	// If any counter update failed, rollback the toggle itself to keep like state and counters consistent.
	if counterErr != nil {
		if _, rbErr := s.repo.ToggleContentLike(ctx, contentID, userID); rbErr != nil {
			logrus.Errorf("like: toggle rollback failed for content %s: %v", contentID, rbErr)
		}
		return false, counterErr
	}
	// 点赞 / 取消点赞都会改变作者的「被赞」指标：两种方向都要重算。
	if content != nil && s.trustSvc != nil {
		if _, err := s.trustSvc.Recompute(ctx, content.AuthorID); err != nil {
			logrus.Warnf("trust: recompute %s after like toggle: %v", content.AuthorID, err)
		}
	}
	if liked && content.AuthorID != userID {
		sender, ue := s.userRepo.FindByID(ctx, userID)
		if ue == nil {
			// RootID is the root content id; resolve owning post for deep-link navigation.
			postID := ""
			if post, pe := s.postRepo.FindByContentID(ctx, content.RootID); pe == nil && post != nil {
				postID = post.ID
			}
			if postID == "" {
				logrus.Warnf("like: skip content.liked notification, post not found for content %s root %s", contentID, content.RootID)
			} else {
				s.bus.Publish(eventbus.Event{
					Type: eventbus.EventContentLiked,
					Ctx:  ctx,
					Payload: map[string]any{
						"user_id":     content.AuthorID,
						"sender_id":   userID,
						"sender_name": displayNameOf(sender),
						"content_id":  contentID,
						"post_id":     postID,
					},
				})
			}
		}
	}
	return liked, nil
}

func (s *likeService) HasContentLiked(ctx context.Context, contentID, userID string) (bool, error) {
	return s.repo.HasContentLiked(ctx, contentID, userID)
}

func (s *likeService) CountContentLikes(ctx context.Context, contentID string) (int64, error) {
	return s.repo.CountContentLikes(ctx, contentID)
}
