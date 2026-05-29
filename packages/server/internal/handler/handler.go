package handler

import (
	"orinforum/internal/config"
	"orinforum/internal/db"
	"orinforum/internal/repository"
	"orinforum/internal/service"

	"github.com/kataras/iris/v12"
)

type Handler struct {
	Auth       AuthHandler
	Post       PostHandler
	Category   CategoryHandler
	Comment    CommentHandler
	Like       LikeHandler
	Favorite   FavoriteHandler
	Follow     FollowHandler
	User       UserHandler
	Role       RoleHandler
	Permission PermissionHandler
	Tag        TagHandler
}

// Health 健康检查
// @Summary      健康检查
// @Description  检查服务是否正常运行
// @Tags         系统
// @Accept       json
// @Produce      json
// @Success      200  {object}  object{status=string}
// @Router       /health [get]
func Health(ctx iris.Context) {
	ctx.JSON(iris.Map{"status": "ok"})
}

func New(store db.Store, cfg config.Config, permSvc service.PermissionService) *Handler {
	userRepo := repository.NewUserRepository(store)
	roleRepo := repository.NewRoleRepository(store)
	postRepo := repository.NewPostRepository(store)
	catRepo := repository.NewCategoryRepository(store)
	commentRepo := repository.NewCommentRepository(store)
	tagRepo := repository.NewTagRepository(store)
	likeRepo := repository.NewLikeRepository(store)
	favRepo := repository.NewFavoriteRepository(store)
	followRepo := repository.NewFollowRepository(store)

	authSvc := service.NewAuthService(userRepo)
	userSvc := service.NewUserService(userRepo)
	postSvc := service.NewPostService(postRepo)
	catSvc := service.NewCategoryService(catRepo)
	commentSvc := service.NewCommentService(commentRepo)
	tagSvc := service.NewTagService(tagRepo)
	likeSvc := service.NewLikeService(likeRepo)
	favSvc := service.NewFavoriteService(favRepo)
	followSvc := service.NewFollowService(followRepo)

	return &Handler{
		Auth:       newAuthHandler(authSvc),
		User:       newUserHandler(userSvc),
		Role:       newRoleHandler(roleRepo),
		Post:       newPostHandler(postSvc),
		Category:   newCategoryHandler(catSvc),
		Comment:    newCommentHandler(commentSvc),
		Like:       newLikeHandler(likeSvc),
		Favorite:   newFavoriteHandler(favSvc),
		Follow:     newFollowHandler(followSvc),
		Permission: newPermissionHandler(permSvc),
		Tag:        newTagHandler(tagSvc),
	}
}
