package handler

import (
	"orinforum/internal/config"
	"orinforum/internal/db"
	"orinforum/internal/repository"
	"orinforum/internal/service"
	"orinforum/internal/storage"

	"github.com/kataras/iris/v12"
)

type Handler struct {
	Auth       AuthHandler
	Post       PostHandler
	Category   CategoryHandler
	Content    ContentHandler
	Like       LikeHandler
	Favorite   FavoriteHandler
	Follow     FollowHandler
	User       UserHandler
	Role       RoleHandler
	Permission PermissionHandler
	Tag          TagHandler
	Upload       UploadHandler
	Notification NotificationHandler
	Report       ReportHandler
	Admin        AdminHandler
	Setting      SettingHandler
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

func New(store db.Store, cfg config.Config, permSvc service.PermissionService, st storage.Provider) *Handler {
	userRepo := repository.NewUserRepository(store)
	roleRepo := repository.NewRoleRepository(store)
	postRepo := repository.NewPostRepository(store)
	catRepo := repository.NewCategoryRepository(store)
	contentRepo := repository.NewContentRepository(store)
	tagRepo := repository.NewTagRepository(store)
	likeRepo := repository.NewLikeRepository(store)
	favRepo := repository.NewFavoriteRepository(store)
	followRepo := repository.NewFollowRepository(store)

	authSvc := service.NewAuthService(userRepo)
	userSvc := service.NewUserService(userRepo)
	postSvc := service.NewPostService(postRepo, contentRepo, store)
	catSvc := service.NewCategoryService(catRepo)
	contentSvc := service.NewContentService(contentRepo, postRepo)
	tagSvc := service.NewTagService(tagRepo)
	likeSvc := service.NewLikeService(likeRepo)
	favSvc := service.NewFavoriteService(favRepo, postRepo)
	followSvc := service.NewFollowService(followRepo, userRepo)

	uploadRepo := repository.NewUploadRepository(store)
	uploadSvc := service.NewUploadService(uploadRepo, st)

	notifRepo := repository.NewNotificationRepository(store)
	notifSvc := service.NewNotificationService(notifRepo)

	adminRepo := repository.NewAdminRepository(store)
	adminSvc := service.NewAdminService(adminRepo)

	settingRepo := repository.NewSettingRepository(store)
	settingSvc := service.NewSettingService(settingRepo)

	reportRepo := repository.NewReportRepository(store)
	reportSvc := service.NewReportService(reportRepo)

	return &Handler{
		Auth:       newAuthHandler(authSvc),
		User:       newUserHandler(userSvc),
		Role:       newRoleHandler(roleRepo),
		Post:       newPostHandler(postSvc),
		Category:   newCategoryHandler(catSvc),
		Content:    newContentHandler(contentSvc),
		Like:       newLikeHandler(likeSvc),
		Favorite:   newFavoriteHandler(favSvc),
		Follow:     newFollowHandler(followSvc),
		Permission: newPermissionHandler(permSvc),
		Tag:          newTagHandler(tagSvc),
		Upload:       newUploadHandler(uploadSvc),
		Notification: newNotificationHandler(notifSvc),
		Report:       newReportHandler(reportSvc),
		Admin:        newAdminHandler(adminSvc),
		Setting:      newSettingHandler(settingSvc),
	}
}
