package server

import (
	"context"
	"fmt"
	"orinforum/internal/config"
	"orinforum/internal/db"
	"orinforum/internal/handler"
	"orinforum/internal/middleware"
	"orinforum/internal/model"
	"orinforum/internal/repository"
	"orinforum/internal/router"
	"time"

	"github.com/google/uuid"
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
		&model.Post{},
		&model.Category{},
		&model.Comment{},
	); err != nil {
		logrus.Warnf("AutoMigrate: %v", err)
	}

	seedRoles(store)
	seedAdmin(store)

	app := iris.New()

	if env == Dev {
		app.Logger().SetLevel("debug")
	}

	app.Use(middleware.CORS())
	app.Use(middleware.Logger())

	h := handler.New(store, cfg)
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
		r.ID = uuid.New().String()
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
		ID:        uuid.New().String(),
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
