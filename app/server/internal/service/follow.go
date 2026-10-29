package service

import (
	"context"

	"orinforum/internal/pkg"
	"orinforum/internal/pkg/eventbus"
	"orinforum/internal/repository"
)

type followService struct {
	repo     repository.FollowRepository
	userRepo repository.UserRepository
	bus      *eventbus.EventBus
}

func NewFollowService(repo repository.FollowRepository, userRepo repository.UserRepository, bus *eventbus.EventBus) FollowService {
	return &followService{
		repo:     repo,
		userRepo: userRepo,
		bus:      bus,
	}
}

func (s *followService) Follow(ctx context.Context, followerID, followingID string) error {
	if followerID == followingID {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "不能关注自己")
	}
	user, err := s.userRepo.FindByID(ctx, followingID)
	if err != nil {
		return err
	}
	if user == nil {
		return pkg.NewAppError(pkg.NOT_FOUND, "user not found")
	}
	if err := s.repo.Follow(ctx, followerID, followingID); err != nil {
		return err
	}
	// 发关注通知（通过事件总线）
	sender, err := s.userRepo.FindByID(ctx, followerID)
	if err == nil {
		s.bus.Publish(eventbus.Event{
			Type: eventbus.EventUserFollowed,
			Ctx:  ctx,
			Payload: map[string]any{
				"user_id":     followingID,
				"sender_id":   followerID,
				"sender_name": displayNameOf(sender),
				"follower_id": followerID,
			},
		})
	}
	return nil
}

func (s *followService) Unfollow(ctx context.Context, followerID, followingID string) error {
	return s.repo.Unfollow(ctx, followerID, followingID)
}

func (s *followService) IsFollowing(ctx context.Context, followerID, followingID string) (bool, error) {
	return s.repo.IsFollowing(ctx, followerID, followingID)
}

func (s *followService) GetFollowers(ctx context.Context, userID string, page, pageSize int) ([]string, error) {
	page, pageSize = pkg.NormalizePagination(page, pageSize)
	return s.repo.GetFollowers(ctx, userID, (page-1)*pageSize, pageSize)
}

func (s *followService) GetFollowing(ctx context.Context, userID string, page, pageSize int) ([]string, error) {
	page, pageSize = pkg.NormalizePagination(page, pageSize)
	return s.repo.GetFollowing(ctx, userID, (page-1)*pageSize, pageSize)
}

func (s *followService) CountFollowers(ctx context.Context, userID string) (int64, error) {
	return s.repo.CountFollowers(ctx, userID)
}

func (s *followService) CountFollowing(ctx context.Context, userID string) (int64, error) {
	return s.repo.CountFollowing(ctx, userID)
}
