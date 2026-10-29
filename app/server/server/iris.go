package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"orinforum/internal/config"
	"orinforum/internal/container"
	"orinforum/internal/db"
	"orinforum/internal/gateway"
	"orinforum/internal/metrics"
	"orinforum/internal/middleware"
	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"orinforum/internal/pkg/eventbus"
	"orinforum/internal/pkg/kvstore"
	"orinforum/internal/pkg/logger"
	"orinforum/internal/pkg/ratelimit"
	"orinforum/internal/repository"
	"orinforum/internal/router"
	"orinforum/internal/service"
	"orinforum/internal/storage"
	"time"

	_ "orinforum/docs"

	irisSwagger "github.com/iris-contrib/swagger"
	"github.com/iris-contrib/swagger/swaggerFiles"
	"github.com/joho/godotenv"
	"github.com/kataras/iris/v12"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
)

type Env string

const (
	Dev  Env = "dev"
	Prod Env = "prod"
)

func Start(env Env) {
	loadEnv(env)

	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		logrus.Fatalf("invalid config: %v", err)
	}

	// 配置必须先于任何业务日志生效：同时接管直接调用 logrus 的存量代码。
	logger.SetGlobal(logger.New(cfg.Log))

	store, err := db.New(cfg.DB)
	if err != nil {
		logrus.Fatalf("failed to initialize database: %v", err)
	}

	if err := store.AutoMigrate(
		&model.Role{},
		&model.User{},
		&model.Content{},
		&model.Post{},
		&model.Space{},
		&model.Permission{},
		&model.RolePermission{},
		&model.SpaceRolePermission{},
		&model.Tag{},
		&model.PostTag{},
		&model.ContentLike{},
		&model.PostFavorite{},
		&model.TopicReadState{},
		&model.UserFollow{},
		&model.Upload{},
		&model.Notification{},
		&model.Report{},
		&model.Motion{},
		&model.MotionVote{},
		&model.Setting{},
		&model.Banned{},
		&model.Announcement{},
		&model.OAuthAccount{},
		&model.CheckIn{},
		&model.Credential{},
		&model.Meta{},
		&model.AuditLog{},
		&model.ContentModeration{},
		&model.ModerationRule{},
		&model.ModerationConfig{},
		&model.OAuthClient{},
		&model.OAuthAuthorize{},
		&model.OAuthToken{},
		&model.APIKey{},
		&model.UserSession{},
		&model.SecurityEvent{},
		&model.LoginHistory{},
		&model.Conversation{},
		&model.ConversationMember{},
		&model.Message{},
		&model.Invite{},
		&model.Watch{},
		&model.UserBlock{},
		&model.UserMFA{},
		&model.UserPreference{},
		&model.WebhookDelivery{},
		&model.Webhook{},
		&model.UserBadge{},
		&model.Badge{},
		&model.Draft{},
		&model.Revision{},
		&model.Backup{},
		&model.Plugin{},
		&model.Group{},
		&model.GroupMember{},
		&model.GroupPermission{},
		&model.ImportMap{},
		&model.XPRule{},
		&model.XPStreakBonus{},
		&model.TrustLevel{},
		&model.XPAdjustment{},
		&model.GateRule{},
		&model.GateDomainDefault{},
	); err != nil {
		logrus.Warnf("AutoMigrate: %v", err)
	}

	// 存量库的 trust_level 是字符串（basic/member/…），total_points 尚未改名。
	// 必须在任何读取 users 的种子步骤之前完成转换，否则 GORM 把 'basic' 扫进
	// int 字段会直接失败。
	migrateTrustData(store)
	migrateSpaceVisibility(store)

	backfillDisplayName(store) // one-time: display_name = username for legacy rows

	seedRoles(store)
	seedAdmin(store)
	seedGroups(store)
	// 必须早于 syncDefaultSettings：它只在 moderation_mode 缺行时迁移旧开关。
	migrateModerationMode(store)
	syncDefaultSettings(store) // idempotent — ensures all default setting keys exist
	syncModerationConfigs(store)
	seedTrustDefaults(store) // 仅当配置表为空时写入，不覆盖后台自定义
	seedGateDefaults(store)

	// Determine whether the one-time seed has already run. The destructive seed
	// steps below (role-permission sync, post/content reseed) run only on the
	// first boot — re-running them every restart would clobber an admin's
	// customizations (R-04). Each step still re-seeds when its own table is
	// empty, so a wiped table recovers.
	metaRepo := repository.NewMetaRepository(store)
	seedCtx := context.Background()
	seeded, _, err := metaRepo.Get(seedCtx, model.MetaSeeded)
	alreadySeeded := seeded == "true"
	if err != nil {
		logrus.Warnf("check seed flag: %v", err)
	}

	// NATS 必须是外部服务（见 deploy/docker-compose.yml 的 nats 服务）。
	//
	// 此前 NATS_URL 留空会启动进程内 nats-server，号称「单二进制零依赖」；实际代价是
	// 生产环境里 KV / EventBus 的生命周期绑在应用进程上——进程重启即丢 JetStream
	// 队列、多实例无法共享限流与会话缓存。改为强制外部 NATS：配错立刻失败，
	// 而不是静默降级成一个只有本进程可见的存储。
	//
	// natsembedded 包保留（仅 internal/pkg/testnats 使用）：单元测试仍需一个
	// 自包含的 NATS，否则 CI 要多起一个外部服务。
	natsURL := strings.TrimSpace(cfg.KV.NATSURL)
	if natsURL == "" {
		logrus.Fatal("NATS_URL 未配置：KV Store 与 EventBus 依赖外部 NATS（JetStream 已启用）。" +
			"请参考 deploy/docker-compose.yml 的 nats 服务，或本地执行 `nats-server -js`。")
	}
	logrus.Infof("EventBus/KV Store: external NATS at %s", natsURL)

	// create KV store (NATS JetStream) — must be created
	// before seedPermissions so services receive the kv dependency.
	kvStore, err := kvstore.New(kvstore.Config{
		NATSURL:    natsURL,
		NATSBucket: cfg.KV.NATSBucket,
	})
	if err != nil {
		logrus.Fatalf("初始化 KV Store 失败: %v", err)
	}

	eventBus, err := eventbus.New(natsURL)
	if err != nil {
		logrus.Fatalf("初始化 EventBus 失败: %v", err)
	}

	permSvc, settingSvc, bannedSvc := seedPermissions(store, kvStore, alreadySeeded)

	// 从 settings 表加载运行时配置（OAuth token TTL、分页参数等）
	initCtx := context.Background()
	pkg.InitOAuthTTL(initCtx, settingSvc)
	pkg.InitPagination(initCtx, settingSvc)
	logrus.Infof("配置加载: OAuth access token TTL=%v, page max=%d, default=%d",
		pkg.OAuthAccessTokenTTL, pkg.PageMaxSize, pkg.PageDefaultSize)
	// 基础版面与标签：首次启动或对应表被清空时补齐，所有环境生效。
	// 已存在的行不会被覆盖（管理员改名/排序调整在重启后保留）。
	seedBaselineSpaces(store, alreadySeeded)
	seedBaselineTags(store, alreadySeeded)
	// 暗区为系统版面，无条件幂等确保：老站点升级后也必须存在，否则动议 exile 无处落地。
	spaceRepoForSeed := repository.NewSpaceRepository(store)
	ensureSeedDarkZone(initCtx, spaceRepoForSeed)
	// 存量版面补 recommendable=true：该列默认 false（为暗区而设），老库里的
	// 普通版面需要一次性补齐，否则整站主推荐会把它们一起排除掉。
	if err := spaceRepoForSeed.BackfillRecommendable(initCtx); err != nil {
		logrus.Warnf("backfill space recommendable: %v", err)
	}

	// mock 用户和帖子种子仅 dev 环境运行，线上不需要
	if env == Dev {
		ensureMockData(store)
		reseedPostsAndContents(store, alreadySeeded)
	}

	// Record that the one-time seed has completed so destructive steps are
	// skipped on the next restart.
	if !alreadySeeded {
		if err := metaRepo.Set(seedCtx, model.MetaSeeded, "true"); err != nil {
			logrus.Warnf("set seed flag: %v", err)
		}
	}

	// create storage provider
	// 存储配置以 settings 表为唯一可信来源（缺省回退到环境变量，见
	// pkg.BuildStorageConfig）。存储为启动期单例，修改 settings 后需重启生效。
	// 将解析后的配置写回 cfg.Storage，使健康检查等下游消费者与实际使用的配置一致。
	logrus.Info(">>> before BuildStorageConfig")
	cfg.Storage = pkg.BuildStorageConfig(initCtx, settingSvc)
	logrus.Info(">>> after BuildStorageConfig")

	// site_url 缺失会让 sitemap / robots / canonical 退化成 http://localhost:8080，
	// 爬虫拿到的是不可达地址。这是「SEO 看着正常、收录却为零」的典型静默故障，
	// 所以在启动时显式告警，而不是等运维去爬虫日志里发现。
	warnIfSiteURLMissing(initCtx, settingSvc)

	st, err := storage.New(cfg.Storage)
	if err != nil {
		logrus.Fatalf("failed to initialize storage: %v", err)
	}

	// Start permission sync subscriber so this instance reloads its cache
	// when another instance calls SyncRolePermissions.
	permSyncCtx, cancelPermSync := context.WithCancel(context.Background())
	logrus.Info(">>> before StartSyncSubscriber")
	if err := permSvc.StartSyncSubscriber(permSyncCtx); err != nil {
		logrus.Warnf("权限缓存同步订阅启动失败: %v", err)
	}
	logrus.Info(">>> after StartSyncSubscriber")

	app := iris.New()

	if env == Dev {
		app.Logger().SetLevel("debug")
	}

	// Trust reverse proxy headers for client IP extraction.
	// When deployed behind nginx/cloudflare etc., ctx.RemoteAddr() will
	// return the real client IP instead of the proxy's address.
	// Only enable if the proxy is trusted — an untrusted network can
	// spoof these headers to bypass rate limiting.
	app.Configure(iris.WithRemoteAddrHeader("X-Forwarded-For"),
		iris.WithRemoteAddrHeader("X-Real-IP"))

	// Trace MUST precede Logger: Iris runs app-level middleware before
	// per-party ones, so the trace has to exist before the request log is built.
	app.Use(middleware.CORS())
	app.Use(middleware.Trace(cfg.TracingEnabled))
	app.Use(middleware.Logger())
	// After Logger so a panic still produces a request log line with status 500.
	app.Use(middleware.Recover())

	var metricsSrv *http.Server
	if cfg.MetricsEnabled {
		app.Use(middleware.Prometheus())
		metrics.RegisterDBPoolCollector(store)

		metricsAddr := fmt.Sprintf(":%d", cfg.MetricsPort)
		srv, err := metrics.Serve(metricsAddr)
		if err != nil {
			logrus.WithError(err).Warnf("metrics listener on %s unavailable", metricsAddr)
		} else {
			metricsSrv = srv
			logrus.Infof("Prometheus metrics listening on %s/metrics (internal only)", metricsAddr)
		}
	}

	// Swagger UI
	swaggerUI := irisSwagger.Handler(swaggerFiles.Handler,
		irisSwagger.URL("/swagger/doc.json"),
		irisSwagger.DeepLinking(true),
		irisSwagger.Prefix("/swagger"),
	)
	app.Get("/swagger/{any:path}", swaggerUI)

	// Build services shared by handler and middleware. Each is constructed once
	// here and handed to BOTH layers so they share a single authority (M-25).
	spaceRepo := repository.NewSpaceRepository(store)
	postRepo := repository.NewPostRepository(store)
	spaceSvc := service.NewSpaceService(spaceRepo, postRepo, db.NewTransactionManager(store), kvStore)
	// NGA: wire space hierarchy into permission cache for inheritance
	// compile-time check: PermissionService must implement hierarchy methods
	if ps, ok := permSvc.(interface {
		SetSpaceRepository(repository.SpaceRepository)
		RefreshSpaceParents(context.Context) error
	}); ok {
		ps.SetSpaceRepository(spaceRepo)
		refreshCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := ps.RefreshSpaceParents(refreshCtx)
		cancel()
		if err != nil {
			logrus.Warnf("refresh space parents for permission cache: %v", err)
		} else {
			logrus.Info("space permission hierarchy cache loaded")
		}
	} else {
		logrus.Warn("permission service does not implement hierarchy cache — space inheritance disabled")
	}
	if ss, ok := spaceSvc.(interface {
		SetPermissionCacheUpdater(service.SpacePermissionCacheUpdater)
	}); ok {
		if updater, ok := permSvc.(service.SpacePermissionCacheUpdater); ok {
			ss.SetPermissionCacheUpdater(updater)
		} else {
			logrus.Warn("permission service does not implement SpacePermissionCacheUpdater")
		}
	}
	// The gateway's dependency set is built once here and reused by the three
	// gateway components the startup also needs. Typing the fields (rather than
	// asserting service interfaces at the call site) means a signature drift is a
	// compile error in the composition root instead of a startup panic.
	gwDeps := gateway.Deps{
		SettingService: settingSvc,
		Perms:          permSvc,
		Ban:            bannedSvc,
		KV:             kvStore,
	}
	if sqlStore := store.SQL(); sqlStore != nil {
		gwDeps.DB = sqlStore.DB()
	}

	// Server-side session authority: user_sessions rows + KV read-through cache.
	// It replaces the old JWT + blacklist pair — issuing a credential creates a
	// row, revoking deletes it, and an unresolved credential is not
	// authenticated. One instance backs the services, the handlers and the
	// gateway middleware.
	var sessions service.SessionStore
	if ss := gateway.NewSessionStore(gwDeps); ss != nil {
		sessions = ss
	}

	// SecManager provides brute-force protection for the auth login flow.
	var secSvc service.SecManager
	if gwDeps.DB != nil {
		secSvc = gateway.NewSecurityManager(gwDeps)
	}

	// Build middleware deps (replaces global setter injection)
	deps := middleware.Deps{
		Settings: settingSvc,
		Perms:    permSvc,
		Ban:      bannedSvc,
		Limiter:  ratelimit.NewLimiter(kvStore),
	}

	// Assemble the object graph, then start its background workers. The two
	// steps are separate so construction stays side-effect free (see container).
	c := container.New(container.Deps{
		Config:     cfg,
		Store:      store,
		KV:         kvStore,
		EventBus:   eventBus,
		Storage:    st,
		Permission: permSvc,
		Setting:    settingSvc,
		Banned:     bannedSvc,
		Space:      spaceSvc,
		Sessions:   sessions,
		Security:   secSvc,
	})

	runtime, err := c.Start(context.Background())
	if err != nil {
		logrus.WithError(err).Fatal("failed to start background workers")
	}

	h := newHandler(c)
	router.Register(app, h, deps, gateway.New(gwDeps))

	addr := fmt.Sprintf(":%s", cfg.Port)

	// Check port availability before listening
	if ln, err := net.Listen("tcp", addr); err != nil {
		logrus.Fatalf("端口 %s 已被占用，请先释放该端口后再启动", cfg.Port)
	} else {
		_ = ln.Close()
	}

	logrus.Infof("OrinForum listening on %s", addr)

	// Run server in background goroutine
	go func() {
		if err := app.Listen(addr); err != nil && err != iris.ErrServerClosed {
			logrus.Fatalf("server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	logrus.Infof("received signal %v, shutting down gracefully...", sig)

	// Graceful shutdown with 10s timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := app.Shutdown(ctx); err != nil {
		logrus.Fatalf("server forced to shutdown: %v", err)
	}

	if metricsSrv != nil {
		if err := metricsSrv.Shutdown(ctx); err != nil {
			logrus.WithError(err).Warn("metrics listener shutdown")
		}
	}

	// Drain buffered view counts and stop workers before closing KV store
	runtime.Shutdown()

	// Stop permission sync subscriber
	cancelPermSync()

	// Close KV store and DB store (defers also run, but close before logging exit)
	if err := kvStore.Close(); err != nil {
		logrus.Warnf("kvstore close error: %v", err)
	}
	if err := store.Close(); err != nil {
		logrus.Warnf("db store close error: %v", err)
	}

	logrus.Info("server exited gracefully")
}

func loadEnv(env Env) {
	files := []string{".env"}
	switch env {
	case Dev:
		files = append(files, ".env.dev")
	case Prod:
		files = append(files, ".env.prod")
	}
	for _, f := range files {
		_ = godotenv.Overload(f)
	}
}

func seedRoles(store db.Store) {
	roleRepo := repository.NewRoleRepository(store)
	ctx := context.Background()

	for _, role := range model.DefaultRoles {
		r := role
		r.ID = pkg.NewID()
		r.CreatedAt = time.Now()
		if err := roleRepo.CreateIfNotExists(ctx, &r); err != nil {
			logrus.Fatalf("seed role %s: %v", r.Name, err)
		}
	}

	// Load role cache from DB
	if roles, err := roleRepo.List(ctx); err == nil {
		model.LoadRoleCache(roles)
		logrus.Infof("loaded %d roles into cache", len(roles))
	}
}

func seedAdmin(store db.Store) {
	userRepo := repository.NewUserRepository(store)
	credRepo := repository.NewCredentialRepository(store)
	metaRepo := repository.NewMetaRepository(store)
	ctx := context.Background()

	existing, err := userRepo.FindByUsername(ctx, "admin")
	if err == nil && existing != nil {
		return
	}

	adminPassword := os.Getenv("ADMIN_PASSWORD")
	if adminPassword == "" {
		buf := make([]byte, 16)
		if _, err := rand.Read(buf); err != nil {
			logrus.Fatalf("generate admin password: %v", err)
		}
		adminPassword = hex.EncodeToString(buf)
		logrus.Warnf("ADMIN_PASSWORD 未设置，已生成随机管理员密码: %s", adminPassword)
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost)
	admin := &model.User{
		ID:          pkg.NewID(),
		Username:    "admin",
		DisplayName: "admin",
		Email:       "admin@orinforum.local",
		Role:        "admin",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := userRepo.Create(ctx, admin); err != nil {
		logrus.Fatalf("seed admin user: %v", err)
	}
	// Save password hash as separate credential
	cred := &model.Credential{
		UserID:       admin.ID,
		PasswordHash: string(hash),
	}
	if err := credRepo.Upsert(ctx, cred); err != nil {
		logrus.Fatalf("seed admin credential: %v", err)
	}
	// Flag the seeded admin to force a password change on first login (R-05).
	// The key's presence alone means "must change"; it is deleted after the
	// password is changed.
	if err := metaRepo.Set(ctx, model.MustChangePasswordKey(admin.ID), "1"); err != nil {
		logrus.Fatalf("seed admin must-change-password flag: %v", err)
	}
	logrus.Info("default admin created (admin / admin123) — must change password on first login")
}

// roleMappingsEmpty reports whether no role-permission mappings exist, in which
// case the destructive seed should run even after the first boot (recovery).

func seedGroups(store db.Store) {
	groupRepo := repository.NewGroupRepository(store)
	ctx := context.Background()
	for _, g := range model.DefaultSystemGroups {
		gg := g
		gg.ID = pkg.NewID()
		gg.CreatedAt = time.Now()
		if err := groupRepo.CreateIfNotExistsByName(ctx, &gg); err != nil {
			logrus.Fatalf("seed group %s: %v", gg.Name, err)
		}
	}
	// best-effort: sync existing users into their system groups
	users, err := repository.NewUserRepository(store).List(ctx, 0, 10000)
	if err != nil {
		logrus.Warnf("seed groups: list users: %v", err)
		return
	}
	for _, u := range users {
		g, err := groupRepo.FindByName(ctx, u.Role)
		if err != nil {
			continue
		}
		if !g.IsSystem {
			continue
		}
		_ = groupRepo.AddMember(ctx, model.NewGroupMember(g.ID, u.ID, model.GroupMemberRoleMember))
	}
	logrus.Infof("seeded %d system groups", len(model.DefaultSystemGroups))
}

// ensureRolePermission 幂等地把某个权限码补进角色映射。
func ensureRolePermission(ctx context.Context, permRepo repository.PermissionRepository, role, code string) {
	codes, err := permRepo.GetRolePermissions(ctx, role)
	if err != nil {
		logrus.Warnf("read role permissions for %s: %v", role, err)
		return
	}
	for _, existing := range codes {
		if existing == code {
			return
		}
	}
	codes = append(codes, code)
	if err := permRepo.SyncRolePermissions(ctx, role, codes); err != nil {
		logrus.Warnf("add %s to role %s: %v", code, role, err)
	} else {
		logrus.Infof("granted %s to role %s", code, role)
	}
}

func roleMappingsEmpty(ctx context.Context, permRepo repository.PermissionRepository) bool {
	all, err := permRepo.GetAllRolePermissions(ctx)
	if err != nil {
		logrus.Warnf("check role-permission mappings: %v", err)
		return false // on error, assume non-empty so we don't clobber existing data
	}
	return len(all) == 0
}

func seedPermissions(store db.Store, kv kvstore.Store, alreadySeeded bool) (service.PermissionService, service.SettingService, service.BannedService) {
	permRepo := repository.NewPermissionRepository(store)
	ctx := context.Background()

	// Seed permission definitions
	for _, perm := range model.DefaultPermissions {
		p := perm
		if err := permRepo.CreateIfNotExists(ctx, &p); err != nil {
			logrus.Fatalf("seed permission %s: %v", perm.Code, err)
		}
	}
	logrus.Infof("seeded %d permissions", len(model.DefaultPermissions))

	// Seed default role-permission mappings — only on first run, or when the
	// mapping table is empty (recovery). This sync is a destructive
	// delete-then-insert, so re-running it on every restart would clobber any
	// role-permission customizations an administrator has made (R-04).
	if alreadySeeded && !roleMappingsEmpty(ctx, permRepo) {
		logrus.Info("default role-permission mappings already seeded, skipping")
		// 增量补齐 pending_user（审批注册）角色权限：仅当该角色尚无任何映射时写入，不覆盖自定义。
		if codes, ok := model.DefaultRolePermissions["pending_user"]; ok {
			all, err := permRepo.GetAllRolePermissions(ctx)
			if err == nil {
				if _, exists := all["pending_user"]; !exists {
					if err := permRepo.SyncRolePermissions(ctx, "pending_user", codes); err != nil {
						logrus.Warnf("sync role permissions for pending_user: %v", err)
					} else {
						logrus.Info("seeded pending_user role-permission mappings")
					}
				}
			}
		}
	} else {
		for roleName, codes := range model.DefaultRolePermissions {
			if err := permRepo.SyncRolePermissions(ctx, roleName, codes); err != nil {
				logrus.Warnf("sync role permissions for %s: %v", roleName, err)
			}
		}
		logrus.Info("seeded default role-permission mappings")
	}

	// 新增权限码在升级库上不会自动进入 admin 映射（上面的同步只跑首启）。
	// 逐个补齐：只追加缺失项，不重置管理员已有的自定义映射。
	for _, code := range []string{model.PermLevelManage, model.PermGateManage} {
		ensureRolePermission(ctx, permRepo, "admin", code)
	}

	// Build permission service and load cache
	spacePermRepo := repository.NewSpaceRolePermissionRepository(store)
	groupRepo := repository.NewGroupRepository(store)
	permSvc := service.NewPermissionService(permRepo, spacePermRepo, groupRepo, kv)
	if err := permSvc.LoadCache(ctx); err != nil {
		logrus.Warnf("load permission cache: %v", err)
	}
	logrus.Info("permission cache loaded")

	// Build setting service
	settingRepo := repository.NewSettingRepository(store)
	settingSvc := service.NewSettingService(settingRepo, kv)

	// Build banned service
	bannedRepo := repository.NewBannedRepository(store)
	bannedSvc := service.NewBannedService(bannedRepo, kv, db.NewTransactionManager(store))

	return permSvc, settingSvc, bannedSvc
}

// warnIfSiteURLMissing 在 site_url 未配置时于启动日志里给出明确告警。
//
// site_url 是 sitemap、robots.txt、canonical 与 og:url 的基准地址。缺失时这些出口
// 全部退化成 http://localhost:8080：站点本身功能正常，但爬虫拿到的是不可达 URL，
// 表现为「一切正常、收录为零」。这种静默故障很难从现象倒推，因此在启动时点名。
//
// 只告警不阻断：本地开发与内网部署本就不需要 site_url，硬性 Fatal 会让这些场景无法启动。
func warnIfSiteURLMissing(ctx context.Context, settings service.SettingService) {
	fromSettings := pkg.GetSettingOrDefault(ctx, settings, "site_url", "", "")
	if fromSettings != "" {
		return
	}
	if os.Getenv("SITE_URL") != "" {
		return
	}
	logrus.Warn(
		"site_url 未配置：sitemap / robots.txt / canonical 将输出 http://localhost:8080，" +
			"搜索引擎会因 URL 不可达而拒绝收录。请在后台「站点信息 → 网站 URL」填写对外域名。",
	)
}
