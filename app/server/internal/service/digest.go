package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"feedchamber/internal/model"
	"feedchamber/internal/pkg"
	"feedchamber/internal/repository"

	"github.com/sirupsen/logrus"
)

type DigestService interface {
	RunWeekly(ctx context.Context) error
	StartWeeklyTicker(ctx context.Context) context.CancelFunc
}

type digestService struct {
	prefs    repository.UserPreferenceRepository
	users    repository.UserRepository
	posts    repository.PostDigestRepository
	watch    repository.WatchRepository
	email    EmailService
	settings SettingService
}

func NewDigestService(prefs repository.UserPreferenceRepository, users repository.UserRepository, posts repository.PostDigestRepository, watch repository.WatchRepository, email EmailService, settings SettingService) DigestService {
	return &digestService{prefs: prefs, users: users, posts: posts, watch: watch, email: email, settings: settings}
}

func (s *digestService) enabled(ctx context.Context) bool {
	if pkg.GetSettingOrDefault(ctx, s.settings, model.SettingEmailNotifyEnabled, "", "false") != "true" {
		return false
	}
	return pkg.GetSettingOrDefault(ctx, s.settings, "email_digest_enabled", "EMAIL_DIGEST_ENABLED", "false") == "true"
}

func (s *digestService) RunWeekly(ctx context.Context) error {
	if !s.enabled(ctx) {
		return nil
	}
	// hot top 5 last 7 days approx via hot sort first page
	hot, _, err := s.posts.ListWithTotal(ctx, repository.PostListFilter{Status: model.PostStatusApproved, SortBy: "hot"}, 0, 5)
	if err != nil {
		logrus.Warnf("digest: hot posts: %v", err)
		hot = nil
	}
	// users with weekly digest preference (default weekly)
	ids, err := s.prefs.ListUsersWithKeyDefault(ctx, model.PrefEmailDigest, model.PrefEmailDigestWeekly, 0, 500)
	if err != nil {
		return err
	}
	for _, userID := range ids {
		u, err := s.users.FindByID(ctx, userID)
		if err != nil || u == nil || u.Email == "" {
			continue
		}
		// watched posts summary: list watches for user and take post targets
		var lines []string
		if s.watch != nil {
			if watches, err := s.watch.ListByUser(ctx, userID, 0, 20); err == nil {
				for _, w := range watches {
					if w.TargetType != model.WatchTargetPost || w.Level == model.WatchLevelMuted {
						continue
					}
					if item, err := s.posts.FindByIDWithDetails(ctx, w.TargetID); err == nil && item != nil {
						lines = append(lines, fmt.Sprintf("- %s (回复 %d)", item.Title, item.CommentCount))
						if len(lines) >= 10 {
							break
						}
					}
				}
			}
		}
		var hotLines []string
		for _, h := range hot {
			hotLines = append(hotLines, fmt.Sprintf("- %s", h.Title))
		}
		body := "本周摘要\n\n你关注的话题:\n" + strings.Join(lines, "\n")
		if len(lines) == 0 {
			body = "本周摘要\n\n你关注的话题暂无更新。\n"
		}
		if len(hotLines) > 0 {
			body += "\n\n全站热门:\n" + strings.Join(hotLines, "\n")
		}
		if err := s.email.SendNotification(ctx, u.Email, "Feed Chamber 周报", "<pre>"+body+"</pre>", body); err != nil {
			logrus.Warnf("digest: send to %s: %v", u.Email, err)
		}
	}
	return nil
}

func (s *digestService) StartWeeklyTicker(ctx context.Context) context.CancelFunc {
	cctx, cancel := context.WithCancel(ctx)
	go func() {
		// check every hour; fire Monday 09:00 local
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		var lastRunDay string
		for {
			select {
			case <-cctx.Done():
				return
			case now := <-ticker.C:
				if now.Weekday() == time.Monday && now.Hour() == 9 {
					day := now.Format("2006-01-02")
					if day == lastRunDay {
						continue
					}
					lastRunDay = day
					if err := s.RunWeekly(cctx); err != nil {
						logrus.Warnf("digest weekly: %v", err)
					}
				}
			}
		}
	}()
	return cancel
}
