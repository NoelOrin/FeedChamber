package service

import (
	"context"
	"errors"
	"feedchamber/internal/db"
	"feedchamber/internal/model"
	"feedchamber/internal/pkg"
	"feedchamber/internal/repository"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

type checkInService struct {
	repo     repository.CheckInRepository
	userRepo repository.UserRepository
	txMgr    db.TransactionManager
	badgeSvc BadgeService
	trustSvc TrustService
}

// SetTrustService 注入等级评估服务（签到后重算 XP 与等级）。
func (s *checkInService) SetTrustService(svc TrustService) { s.trustSvc = svc }

func NewCheckInService(repo repository.CheckInRepository, userRepo repository.UserRepository, txMgr db.TransactionManager, badgeSvc BadgeService) CheckInService {
	return &checkInService{repo: repo, userRepo: userRepo, txMgr: txMgr, badgeSvc: badgeSvc}
}

func (s *checkInService) today() string {
	return time.Now().Format("2006-01-02")
}

func (s *checkInService) yesterday() string {
	return time.Now().AddDate(0, 0, -1).Format("2006-01-02")
}

func (s *checkInService) CheckIn(ctx context.Context, userID string) (*model.CheckIn, error) {
	today := s.today()

	var ci *model.CheckIn
	err := s.txMgr.WithTx(ctx, func(txCtx context.Context) error {
		// Check if already checked in today
		existing, err := s.repo.FindByUserAndDate(txCtx, userID, today)
		if err != nil {
			if !errors.Is(err, repository.ErrNotFound) {
				return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
			}
		} else if existing != nil && existing.ID != "" {
			return pkg.NewAppError(pkg.ALREADY_EXISTS, "今日已签到")
		}

		// Get user for current stats
		user, err := s.userRepo.FindByID(txCtx, userID)
		if err != nil {
			return pkg.NewAppError(pkg.USER_NOT_FOUND)
		}

		// Calculate streak
		var streak int
		lastDate := user.LastCheckInDate
		if lastDate == s.yesterday() {
			streak = user.CurrentStreak + 1
		} else {
			streak = 1
		}

		// points_earned 是旧版「签到直接加分」的遗留列：自 P1 起签到 XP 由
		// TrustService.Recompute 按 checkin_count + 连续里程碑聚合，这里只落 0。
		points := 0

		// Create check-in record
		ci = &model.CheckIn{
			UserID:       userID,
			CheckInDate:  today,
			PointsEarned: points,
			Streak:       streak,
		}
		if err := s.repo.Create(txCtx, ci); err != nil {
			// 并发检查：如果违反唯一约束，说明已签到
			if errors.Is(err, repository.ErrDuplicate) || strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "duplicate") {
				return pkg.NewAppError(pkg.ALREADY_EXISTS, "今日已签到")
			}
			return err
		}

		// Update user stats
		maxStreak := user.MaxStreak
		if streak > maxStreak {
			maxStreak = streak
		}
		user.TotalCheckIns++
		user.CurrentStreak = streak
		user.MaxStreak = maxStreak
		user.LastCheckInDate = today
		// 签到的 XP 由行为聚合派生（checkin_count + 连续里程碑），这里只更新
		// 签到统计；不再直写 total_xp / trust_level。
		return s.userRepo.Update(txCtx, user)
	})
	if err == nil {
		if s.trustSvc != nil {
			if _, terr := s.trustSvc.Recompute(ctx, userID); terr != nil {
				logrus.Warnf("trust: recompute %s after checkin: %v", userID, terr)
			}
		}
		if s.badgeSvc != nil {
			s.badgeSvc.Evaluate(ctx, userID, "checkin")
		}
	}
	return ci, err
}

func (s *checkInService) GetStatus(ctx context.Context, userID string) (*CheckInStatus, error) {
	today := s.today()

	_, err := s.repo.FindByUserAndDate(ctx, userID, today)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	checkedInToday := err == nil

	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, pkg.NewAppError(pkg.USER_NOT_FOUND)
	}

	return &CheckInStatus{
		CheckedInToday: checkedInToday,
		CurrentStreak:  user.CurrentStreak,
		MaxStreak:      user.MaxStreak,
		TotalCheckIns:  user.TotalCheckIns,
		TotalXP:        user.TotalXP,
	}, nil
}

func (s *checkInService) GetHistory(ctx context.Context, userID string, year, month int) ([]model.CheckIn, error) {
	yearMonth := fmt.Sprintf("%04d-%02d", year, month)
	records, err := s.repo.ListByUserAndMonth(ctx, userID, yearMonth)
	if err != nil {
		return nil, err
	}
	if records == nil {
		return []model.CheckIn{}, nil
	}
	return records, nil
}
