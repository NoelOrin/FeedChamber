package service

import (
	"context"
	"time"

	"feedchamber/internal/model"
	"feedchamber/internal/pkg"
	"feedchamber/internal/repository"

	"github.com/sirupsen/logrus"
)

type BadgeService interface {
	SeedDefaults(ctx context.Context) error
	List(ctx context.Context) ([]model.Badge, error)
	ListUserBadges(ctx context.Context, userID string) ([]model.Badge, error)
	Evaluate(ctx context.Context, userID, trigger string)
}

type badgeService struct {
	repo  repository.BadgeRepository
	users repository.UserRepository
}

func NewBadgeService(repo repository.BadgeRepository, users repository.UserRepository) BadgeService {
	return &badgeService{repo: repo, users: users}
}

func (s *badgeService) SeedDefaults(ctx context.Context) error {
	defaults := []model.Badge{
		{ID: pkg.NewID(), Code: model.BadgeFirstPost, Name: "首帖", Description: "发布第一篇帖子", Icon: "✍️", CreatedAt: time.Now()},
		{ID: pkg.NewID(), Code: model.BadgeStreak7, Name: "连续签到7天", Description: "连续签到达到7天", Icon: "🔥", CreatedAt: time.Now()},
		{ID: pkg.NewID(), Code: model.BadgeLiked10, Name: "获赞10", Description: "累计获得10个赞", Icon: "👍", CreatedAt: time.Now()},
	}
	for _, b := range defaults {
		if err := s.repo.UpsertBadge(ctx, &b); err != nil {
			return err
		}
	}
	return nil
}

func (s *badgeService) List(ctx context.Context) ([]model.Badge, error) {
	return s.repo.ListBadges(ctx)
}

func (s *badgeService) ListUserBadges(ctx context.Context, userID string) ([]model.Badge, error) {
	return s.repo.ListUserBadges(ctx, userID)
}

func (s *badgeService) Evaluate(ctx context.Context, userID, trigger string) {
	if userID == "" || s.repo == nil || s.users == nil {
		return
	}
	user, err := s.users.FindByID(ctx, userID)
	if err != nil || user == nil {
		return
	}
	tryAward := func(code string, ok bool) {
		if !ok {
			return
		}
		b, err := s.repo.GetByCode(ctx, code)
		if err != nil || b == nil {
			return
		}
		if err := s.repo.Award(ctx, &model.UserBadge{ID: pkg.NewID(), UserID: userID, BadgeID: b.ID, AwardedAt: time.Now()}); err != nil {
			logrus.Warnf("badge award %s to %s: %v", code, userID, err)
		}
	}
	switch trigger {
	case "post":
		n, _ := s.users.CountPosts(ctx, userID)
		tryAward(model.BadgeFirstPost, n >= 1)
	case "checkin":
		tryAward(model.BadgeStreak7, user.CurrentStreak >= 7)
	case "like":
		n, _ := s.users.CountLikesReceived(ctx, userID)
		tryAward(model.BadgeLiked10, n >= 10)
	}
	// always re-check firstpost/streak/liked opportunistically
	n, _ := s.users.CountPosts(ctx, userID)
	tryAward(model.BadgeFirstPost, n >= 1)
	tryAward(model.BadgeStreak7, user.CurrentStreak >= 7)
	if likes, err := s.users.CountLikesReceived(ctx, userID); err == nil {
		tryAward(model.BadgeLiked10, likes >= 10)
	}
}
