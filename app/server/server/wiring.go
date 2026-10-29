package server

import (
	"orinforum/internal/container"
	"orinforum/internal/handler"
)

// newHandler maps the assembled container onto the HTTP layer's dependency
// list. It lives in the composition root — not in the container and not in the
// handler package — so neither side has to know about the other.
//
// The list is intentionally exhaustive: adding a service to a handler means
// adding a line here, which is what keeps "what does the HTTP layer consume"
// answerable from one place.
func newHandler(c *container.Container) *handler.Handler {
	return handler.New(&handler.Deps{
		Store: c.Store,
		KV:    c.KV,
		Cfg:   c.Config,

		Admin:        c.AdminSvc,
		Announcement: c.AnnouncementSvc,
		AuditLog:     c.AuditLogSvc,
		Auth:         c.AuthSvc,
		Backup:       c.BackupSvc,
		Badge:        c.BadgeSvc,
		Banned:       c.BannedSvc,
		CheckIn:      c.CheckInSvc,
		Content:      c.ContentSvc,
		Draft:        c.DraftSvc,
		Email:        c.EmailSvc,
		Favorite:     c.FavoriteSvc,
		Follow:       c.FollowSvc,
		Group:        c.GroupSvc,
		Invite:       c.InviteSvc,
		Leaderboard:  c.LeaderboardSvc,
		Like:         c.LikeSvc,
		MFA:          c.MFASvc,
		Moderation:   c.ModerationSvc,
		Motion:       c.MotionSvc,
		Notification: c.NotificationSvc,
		OAuth:        c.OAuthSvc,
		Permission:   c.PermissionSvc,
		PM:           c.PMSvc,
		Plugin:       c.PluginSvc,
		Post:         c.PostSvc,
		Report:       c.ReportSvc,
		Revision:     c.RevisionSvc,
		Role:         c.RoleSvc,
		Sessions:     c.SessionSvc,
		Setting:      c.SettingSvc,
		Space:        c.SpaceSvc,
		Tag:          c.TagSvc,
		Theme:        c.ThemeSvc,
		Trust:        c.TrustSvc,
		Gate:         c.GateSvc,
		Upload:       c.UploadSvc,
		User:         c.UserSvc,
		Watch:        c.WatchSvc,
		UserBlock:    c.UserBlockSvc,
		Webhook:      c.WebhookSvc,
	})
}
