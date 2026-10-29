// Package container wires the application's repositories and services.
//
// It is deliberately split in two: New constructs the object graph and nothing
// else — no goroutines, no event subscriptions, no data mutation — while Start
// (runtime.go) launches the background workers. Construction must stay safe to
// call in tests without spawning goroutines that outlive them.
package container

import (
	"feedchamber/internal/config"
	"feedchamber/internal/db"
	"feedchamber/internal/pkg/eventbus"
	"feedchamber/internal/pkg/kvstore"
	"feedchamber/internal/repository"
	"feedchamber/internal/service"
	"feedchamber/internal/storage"
)

// Container holds every repository and service used by the application.
// Fields are exported so the assembly in server/iris.go can pluck the
// dependencies it hands to the HTTP layer and the gateway.
type Container struct {
	// Infrastructure
	Config   config.Config
	Store    db.Store
	KV       kvstore.Store
	EventBus *eventbus.EventBus
	Storage  storage.Provider

	// Transaction manager — one instance shared by every service that needs a
	// transaction, instead of a fresh one per constructor.
	Tx db.TransactionManager

	// Shared authorities handed in at startup (see Deps).
	PermissionSvc service.PermissionService
	SettingSvc    service.SettingService
	BannedSvc     service.BannedService
	SpaceSvc      service.SpaceService
	// SessionSvc is the server-side session authority: Login mints credentials
	// through it and revocation paths (role change, password change, ban) delete
	// rows to sign users out.
	SessionSvc service.SessionStore

	// Repositories (alphabetical)
	AdminRepo        repository.AdminRepository
	AnnouncementRepo repository.AnnouncementRepository
	AuditLogRepo     repository.AuditLogRepository
	BackupRepo       repository.BackupRepository
	BadgeRepo        repository.BadgeRepository
	BannedRepo       repository.BannedRepository
	CheckInRepo      repository.CheckInRepository
	ConversationRepo repository.ConversationRepository
	ContentRepo      repository.ContentRepository
	CredentialRepo   repository.CredentialRepository
	DraftRepo        repository.DraftRepository
	FavoriteRepo     repository.FavoriteRepository
	FollowRepo       repository.FollowRepository
	GateRepo         repository.GateRepository
	GroupRepo        repository.GroupRepository
	InviteRepo       repository.InviteRepository
	LikeRepo         repository.LikeRepository
	MetaRepo         repository.MetaRepository
	ModerationRepo   repository.ModerationRepository
	MotionRepo       repository.MotionRepository
	NotificationRepo repository.NotificationRepository
	OAuthRepo        repository.OAuthRepository
	PluginRepo       repository.PluginRepository
	PostRepo         repository.PostRepository
	ReportRepo       repository.ReportRepository
	RevisionRepo     repository.RevisionRepository
	RoleRepo         repository.RoleRepository
	SettingRepo      repository.SettingRepository
	SpaceRepo        repository.SpaceRepository
	TagRepo          repository.TagRepository
	TopicReadRepo    repository.TopicReadRepository
	UploadRepo       repository.UploadRepository
	UserRepo         repository.UserRepository
	UserBlockRepo    repository.UserBlockRepository
	UserMFARepo      repository.UserMFARepository
	UserPrefRepo     repository.UserPreferenceRepository
	TrustRepo        repository.TrustRepository
	WatchRepo        repository.WatchRepository
	WebhookRepo      repository.WebhookRepository

	// Services (alphabetical)
	AdminSvc        service.AdminService
	AnnouncementSvc service.AnnouncementService
	AuditLogSvc     service.AuditLogService
	AuthSvc         service.AuthService
	BackupSvc       service.BackupService
	BadgeSvc        service.BadgeService
	CheckInSvc      service.CheckInService
	ContentSvc      service.ContentService
	DigestSvc       service.DigestService
	DraftSvc        service.DraftService
	EmailSvc        service.EmailService
	FavoriteSvc     service.FavoriteService
	FollowSvc       service.FollowService
	GateSvc         service.GateService
	GroupSvc        service.GroupService
	InviteSvc       service.InviteService
	LeaderboardSvc  service.LeaderboardService
	LikeSvc         service.LikeService
	MFASvc          service.MFAService
	ModerationSvc   service.ModerationService
	MotionSvc       service.MotionService
	NotificationSvc service.NotificationService
	OAuthSvc        service.OAuthService
	PMSvc           service.PMService
	PluginSvc       service.PluginService
	PostSvc         service.PostService
	ReportSvc       service.ReportService
	RevisionSvc     service.RevisionService
	RoleSvc         service.RoleService
	TagSvc          service.TagService
	ThemeSvc        service.ThemeService
	TrustSvc        service.TrustService
	UploadSvc       service.UploadService
	UserSvc         service.UserService
	WatchSvc        service.WatchService
	UserBlockSvc    service.UserBlockService
	WebhookSvc      service.WebhookService
}

// New constructs the object graph. Services must be built in dependency order;
// repositories have no internal dependencies, so their order is irrelevant.
func New(d Deps) *Container {
	c := &Container{
		Config:        d.Config,
		Store:         d.Store,
		KV:            d.KV,
		EventBus:      d.EventBus,
		Storage:       d.Storage,
		Tx:            db.NewTransactionManager(d.Store),
		PermissionSvc: d.Permission,
		SettingSvc:    d.Setting,
		BannedSvc:     d.Banned,
		SpaceSvc:      d.Space,
		SessionSvc:    d.Sessions,
	}

	// ── 1. Repositories ───────────────────────────────────────────
	c.AdminRepo = repository.NewAdminRepository(d.Store)
	c.AnnouncementRepo = repository.NewAnnouncementRepository(d.Store)
	c.AuditLogRepo = repository.NewAuditLogRepository(d.Store)
	c.BackupRepo = repository.NewBackupRepository(d.Store)
	c.BadgeRepo = repository.NewBadgeRepository(d.Store)
	c.BannedRepo = repository.NewBannedRepository(d.Store)
	c.CheckInRepo = repository.NewCheckInRepository(d.Store)
	c.ConversationRepo = repository.NewConversationRepository(d.Store)
	c.ContentRepo = repository.NewContentRepository(d.Store)
	c.CredentialRepo = repository.NewCredentialRepository(d.Store)
	c.DraftRepo = repository.NewDraftRepository(d.Store)
	c.FavoriteRepo = repository.NewFavoriteRepository(d.Store)
	c.FollowRepo = repository.NewFollowRepository(d.Store)
	c.GateRepo = repository.NewGateRepository(d.Store)
	c.GroupRepo = repository.NewGroupRepository(d.Store)
	c.InviteRepo = repository.NewInviteRepository(d.Store)
	c.LikeRepo = repository.NewLikeRepository(d.Store)
	c.MetaRepo = repository.NewMetaRepository(d.Store)
	c.ModerationRepo = repository.NewModerationRepository(d.Store)
	c.MotionRepo = repository.NewMotionRepository(d.Store)
	c.NotificationRepo = repository.NewNotificationRepository(d.Store)
	c.OAuthRepo = repository.NewOAuthRepository(d.Store)
	c.PluginRepo = repository.NewPluginRepository(d.Store)
	c.PostRepo = repository.NewPostRepository(d.Store)
	c.ReportRepo = repository.NewReportRepository(d.Store)
	c.RevisionRepo = repository.NewRevisionRepository(d.Store)
	c.RoleRepo = repository.NewRoleRepository(d.Store)
	c.SettingRepo = repository.NewSettingRepository(d.Store)
	c.SpaceRepo = repository.NewSpaceRepository(d.Store)
	c.TagRepo = repository.NewTagRepository(d.Store)
	c.TopicReadRepo = repository.NewTopicReadRepository(d.Store)
	c.UploadRepo = repository.NewUploadRepository(d.Store)
	c.UserRepo = repository.NewUserRepository(d.Store)
	c.UserBlockRepo = repository.NewUserBlockRepository(d.Store)
	c.UserMFARepo = repository.NewUserMFARepository(d.Store)
	c.UserPrefRepo = repository.NewUserPreferenceRepository(d.Store)
	c.TrustRepo = repository.NewTrustRepository(d.Store)
	c.WatchRepo = repository.NewWatchRepository(d.Store)
	c.WebhookRepo = repository.NewWebhookRepository(d.Store)

	// ── 2. Services (dependency order matters) ────────────────────

	// Leaf services: depend only on repositories or infrastructure.
	c.InviteSvc = service.NewInviteService(c.InviteRepo)
	c.RoleSvc = service.NewRoleService(c.RoleRepo)
	c.DraftSvc = service.NewDraftService(c.DraftRepo)
	c.PluginSvc = service.NewPluginService(c.PluginRepo)
	c.ThemeSvc = service.NewThemeService(c.UserPrefRepo)
	c.LeaderboardSvc = service.NewLeaderboardService(c.UserRepo)
	c.BadgeSvc = service.NewBadgeService(c.BadgeRepo, c.UserRepo)
	c.AuditLogSvc = service.NewAuditLogService(c.AuditLogRepo)
	c.EmailSvc = service.NewEmailService(c.SettingSvc, d.KV)
	c.MFASvc = service.NewMFAService(c.UserMFARepo, c.SettingSvc, d.KV)
	c.UserSvc = service.NewUserService(c.UserRepo, c.SessionSvc, c.PermissionSvc, c.Tx)
	c.GroupSvc = service.NewGroupService(c.GroupRepo, c.PermissionSvc)
	c.TrustSvc = service.NewTrustService(c.UserRepo, c.TrustRepo, c.SettingSvc, d.KV)
	c.GateSvc = service.NewGateService(c.GateRepo, c.UserRepo, c.TrustRepo, d.KV)
	c.UserBlockSvc = service.NewUserBlockService(c.UserBlockRepo, c.UserRepo)
	c.FavoriteSvc = service.NewFavoriteService(c.FavoriteRepo, c.PostRepo)
	c.FavoriteSvc.SetTrustService(c.TrustSvc)
	c.FollowSvc = service.NewFollowService(c.FollowRepo, c.UserRepo, c.EventBus)
	c.UploadSvc = service.NewUploadService(c.UploadRepo, d.Storage, c.SettingSvc)
	c.OAuthSvc = service.NewOAuthService(c.UserRepo, c.OAuthRepo, c.SettingSvc, d.KV)
	c.TagSvc = service.NewTagService(c.TagRepo, c.Tx, d.KV)
	c.AnnouncementSvc = service.NewAnnouncementService(c.AnnouncementRepo, c.Tx)
	c.WebhookSvc = service.NewWebhookService(c.WebhookRepo)
	c.BackupSvc = service.NewBackupService(c.BackupRepo, d.Storage, d.Store, c.SettingSvc)
	c.RevisionSvc = service.NewRevisionService(c.RevisionRepo, c.PostRepo, c.ContentRepo, c.Tx, c.PermissionSvc)

	// Auth needs most of the above. BindSystemGroupSync hooks group membership
	// onto role changes afterwards, so Auth stays ignorant of Group.
	c.AuthSvc = service.NewAuthService(service.AuthServiceDeps{
		Users:    c.UserRepo,
		Creds:    c.CredentialRepo,
		Meta:     c.MetaRepo,
		Settings: c.SettingSvc,

		Ban:    c.BannedSvc,
		Invite: c.InviteSvc,
		User:   c.UserSvc,
		MFA:    c.MFASvc,
		Email:  c.EmailSvc,

		Tx:       c.Tx,
		Sessions: c.SessionSvc,
		Security: d.Security,
		Bus:      c.EventBus,
	})
	service.BindSystemGroupSync(c.GroupSvc, c.UserSvc, c.AuthSvc)

	c.NotificationSvc = service.NewNotificationService(c.NotificationRepo, d.KV, c.EmailSvc, c.UserRepo, c.SettingSvc, c.UserBlockSvc)
	c.PMSvc = service.NewPMService(c.ConversationRepo, c.UserRepo, c.NotificationSvc, c.UserBlockSvc)

	// Moderation must exist before Post/Content, which consult it pre-persist.
	c.ModerationSvc = service.NewModerationService(c.ModerationRepo, c.PostRepo, c.ContentRepo, c.SettingSvc, c.EventBus)
	c.ModerationSvc.SetTrustService(c.TrustSvc)

	c.WatchSvc = service.NewWatchService(c.WatchRepo, c.TagRepo, c.EventBus)

	c.PostSvc = service.NewPostService(service.PostServiceDeps{
		Posts:      c.PostRepo,
		Contents:   c.ContentRepo,
		TopicReads: c.TopicReadRepo,
		Revisions:  c.RevisionRepo,
		Users:      c.UserRepo,
		SpaceRepo:  c.SpaceRepo,

		Ban:        c.BannedSvc,
		Moderation: c.ModerationSvc,
		Settings:   c.SettingSvc,
		Perms:      c.PermissionSvc,
		Blocks:     c.UserBlockSvc,
		Badges:     c.BadgeSvc,
		Gate:       c.GateSvc,

		Store: d.Store,
		KV:    d.KV,
		Bus:   c.EventBus,
	})
	// Trust level is read back by PostService; the setter breaks what would
	// otherwise be a construction cycle (Trust ← Post ← Trust).
	c.PostSvc.SetTrustService(c.TrustSvc)

	c.ContentSvc = service.NewContentService(service.ContentServiceDeps{
		Contents:  c.ContentRepo,
		Posts:     c.PostRepo,
		Users:     c.UserRepo,
		Revisions: c.RevisionRepo,

		Ban:        c.BannedSvc,
		Moderation: c.ModerationSvc,
		Settings:   c.SettingSvc,
		Watch:      c.WatchSvc,
		Perms:      c.PermissionSvc,
		Trust:      c.TrustSvc,
		Gate:       c.GateSvc,
		PostView:   c.PostSvc,

		Tx:  c.Tx,
		Bus: c.EventBus,
	})

	c.LikeSvc = service.NewLikeService(c.LikeRepo, c.ContentRepo, c.PostRepo, c.UserRepo, c.EventBus, c.BadgeSvc)
	c.LikeSvc.SetTrustService(c.TrustSvc)
	c.CheckInSvc = service.NewCheckInService(c.CheckInRepo, c.UserRepo, c.Tx, c.BadgeSvc)
	c.CheckInSvc.SetTrustService(c.TrustSvc)
	c.ReportSvc = service.NewReportService(c.ReportRepo, c.PostRepo, c.ContentRepo, c.BannedSvc, c.EventBus)
	c.MotionSvc = service.NewMotionService(
		c.MotionRepo, c.PostRepo, c.ContentRepo, c.UserRepo, c.SpaceRepo,
		c.ReportSvc, c.PermissionSvc, c.BannedSvc, c.EventBus,
	)
	c.AdminSvc = service.NewAdminService(c.AdminRepo, c.UserRepo, c.PostRepo, d.KV, c.SessionSvc, c.PermissionSvc, c.Tx)
	c.DigestSvc = service.NewDigestService(c.UserPrefRepo, c.UserRepo, c.PostRepo, c.WatchRepo, c.EmailSvc, c.SettingSvc)

	return c
}
