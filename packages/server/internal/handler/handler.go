package handler

import (
	"orinforum/internal/config"
	"orinforum/internal/db"
	"orinforum/internal/repository"
	"orinforum/internal/service"
)

type Handler struct {
	Auth     AuthHandler
	Post     PostHandler
	Category CategoryHandler
	Comment  CommentHandler
	User     UserHandler
	Role     RoleHandler
}

func New(store db.Store, cfg config.Config) *Handler {
	userRepo := repository.NewUserRepository(store)
	roleRepo := repository.NewRoleRepository(store)
	postRepo := repository.NewPostRepository(store)
	catRepo := repository.NewCategoryRepository(store)
	commentRepo := repository.NewCommentRepository(store)

	authSvc := service.NewAuthService(userRepo)
	userSvc := service.NewUserService(userRepo)
	postSvc := service.NewPostService(postRepo)
	catSvc := service.NewCategoryService(catRepo)
	commentSvc := service.NewCommentService(commentRepo)

	return &Handler{
		Auth:     newAuthHandler(authSvc),
		User:     newUserHandler(userSvc),
		Role:     newRoleHandler(roleRepo),
		Post:     newPostHandler(postSvc),
		Category: newCategoryHandler(catSvc),
		Comment:  newCommentHandler(commentSvc),
	}
}
