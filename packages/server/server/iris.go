package server

import (
	"context"
	"fmt"
	"orinforum/internal/config"
	"orinforum/internal/db"
	"orinforum/internal/handler"
	"orinforum/internal/middleware"
	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"orinforum/internal/repository"
	"orinforum/internal/router"
	"orinforum/internal/service"
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

	store, err := db.New(cfg.DB)
	if err != nil {
		panic(fmt.Sprintf("failed to initialize database: %v", err))
	}
	defer store.Close()

	if err := store.AutoMigrate(
		&model.Role{},
		&model.User{},
		&model.Content{},
		&model.Post{},
		&model.Category{},
		&model.Permission{},
		&model.RolePermission{},
		&model.Tag{},
		&model.PostTag{},
		&model.ContentLike{},
		&model.PostFavorite{},
		&model.UserFollow{},
		&model.Upload{},
		&model.Notification{},
		&model.Report{},
		&model.Setting{},
	); err != nil {
		logrus.Warnf("AutoMigrate: %v", err)
	}

	seedRoles(store)
	seedAdmin(store)
	permSvc := seedPermissions(store)
	seedSettings(store)
	ensureMockData(store)

	// inject setting reader into middleware for dynamic auth gates
	settingRepo := repository.NewSettingRepository(store)
	middleware.SetSettingReader(service.NewSettingService(settingRepo))

	app := iris.New()

	if env == Dev {
		app.Logger().SetLevel("debug")
	}

	app.Use(middleware.CORS())
	app.Use(middleware.Logger())

	// Swagger UI
	swaggerUI := irisSwagger.Handler(swaggerFiles.Handler,
		irisSwagger.URL("/swagger/doc.json"),
		irisSwagger.DeepLinking(true),
		irisSwagger.Prefix("/swagger"),
	)
	app.Get("/swagger/{any:path}", swaggerUI)

	h := handler.New(store, cfg, permSvc)
	router.Register(app, h)

	addr := fmt.Sprintf(":%s", cfg.Port)
	logrus.Infof("OrinForum listening on %s", addr)
	app.Listen(addr)
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
		_ = roleRepo.CreateIfNotExists(ctx, &r)
	}

	// Load role cache from DB
	if roles, err := roleRepo.List(ctx); err == nil {
		model.LoadRoleCache(roles)
		logrus.Infof("loaded %d roles into cache", len(roles))
	}
}

func seedAdmin(store db.Store) {
	userRepo := repository.NewUserRepository(store)
	ctx := context.Background()

	existing, err := userRepo.FindByUsername(ctx, "admin")
	if err == nil && existing != nil {
		return
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	admin := &model.User{
		ID:        pkg.NewID(),
		Username:  "admin",
		Email:     "admin@orinforum.local",
		Password:  string(hash),
		Role:      "admin",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_ = userRepo.Create(ctx, admin)
	logrus.Info("default admin created (admin / admin123)")
}

func seedPermissions(store db.Store) service.PermissionService {
	permRepo := repository.NewPermissionRepository(store)
	ctx := context.Background()

	// Seed permission definitions
	for _, perm := range model.DefaultPermissions {
		p := perm
		_ = permRepo.CreateIfNotExists(ctx, &p)
	}
	logrus.Infof("seeded %d permissions", len(model.DefaultPermissions))

	// Seed default role-permission mappings
	for roleName, codes := range model.DefaultRolePermissions {
		if err := permRepo.SyncRolePermissions(ctx, roleName, codes); err != nil {
			logrus.Warnf("sync role permissions for %s: %v", roleName, err)
		}
	}
	logrus.Info("seeded default role-permission mappings")

	// Build permission service and load cache
	permSvc := service.NewPermissionService(permRepo)
	if err := permSvc.LoadCache(ctx); err != nil {
		logrus.Warnf("load permission cache: %v", err)
	}
	logrus.Info("permission cache loaded")

	// Inject permission checker into middleware
	middleware.SetPermissionChecker(permSvc)

	return permSvc
}

func seedSettings(store db.Store) {
	settingRepo := repository.NewSettingRepository(store)
	ctx := context.Background()

	for key, value := range model.DefaultSettings {
		if err := settingRepo.Set(ctx, key, value); err != nil {
			logrus.Warnf("seed setting %s: %v", key, err)
		}
	}
	logrus.Info("seeded default settings")
}
