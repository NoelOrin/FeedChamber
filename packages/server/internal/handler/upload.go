package handler

import (
	"fmt"
	"orinforum/internal/pkg"
	"orinforum/internal/service"

	"github.com/kataras/iris/v12"
)

type UploadHandler interface {
	Upload(ctx iris.Context)
	GetFile(ctx iris.Context)
	List(ctx iris.Context)
	Delete(ctx iris.Context)
}

type uploadHandler struct{ svc service.UploadService }

func newUploadHandler(svc service.UploadService) UploadHandler {
	return &uploadHandler{svc: svc}
}

// Upload 上传文件
// @Summary      上传文件
// @Description  上传图片文件（jpg/png/gif/webp），最大 10MB
// @Tags         文件上传
// @Accept       multipart/form-data
// @Produce      json
// @Security     BearerAuth
// @Param        file  formData  file  true  "上传的文件"
// @Success      201   {object}  pkg.Response{data=object{id=string,url=string,fileName=string,size=int}}
// @Failure      400   {object}  pkg.Response
// @Failure      401   {object}  pkg.Response
// @Router       /upload/ [post]
func (h *uploadHandler) Upload(ctx iris.Context) {
	userID := ctx.Values().GetString("user_uuid")
	if userID == "" {
		pkg.Fail(ctx, pkg.TOKEN_NOT_EXIST)
		return
	}

	_, fileHeader, err := ctx.FormFile("file")
	if err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "file is required")
		return
	}

	upload, err := h.svc.Save(ctx.Request().Context(), userID, fileHeader)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}

	ctx.StatusCode(iris.StatusCreated)
	pkg.Success(ctx, iris.Map{
		"id":       upload.ID,
		"url":      fmt.Sprintf("/api/v1/upload/%s", upload.ID),
		"fileName": upload.FileName,
		"size":     upload.Size,
	})
}

// GetFile 获取文件
// @Summary      获取文件
// @Description  根据 ID 获取上传的文件
// @Tags         文件上传
// @Produce      application/octet-stream
// @Security     BearerAuth
// @Param        id   path      string  true  "文件 ID"
// @Success      200  {file}    binary
// @Failure      401  {object}  pkg.Response
// @Failure      404  {object}  pkg.Response
// @Router       /upload/{id} [get]
func (h *uploadHandler) GetFile(ctx iris.Context) {
	id := ctx.Params().Get("id")
	upload, err := h.svc.GetByID(ctx.Request().Context(), id)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	ctx.ServeFile(upload.Path)
}

// List 上传列表
// @Summary      上传列表
// @Description  获取当前用户的上传文件列表
// @Tags         文件上传
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page     query  int  false  "页码"  default(1)
// @Param        pageSize query  int  false  "每页数量"  default(20)
// @Success      200  {object}  pkg.Response{data=[]model.Upload}
// @Failure      401  {object}  pkg.Response
// @Router       /upload/ [get]
func (h *uploadHandler) List(ctx iris.Context) {
	userID := ctx.Values().GetString("user_uuid")
	if userID == "" {
		pkg.Fail(ctx, pkg.TOKEN_NOT_EXIST)
		return
	}

	p := readListParams(ctx)

	uploads, err := h.svc.ListByUser(ctx.Request().Context(), userID, p.Page, p.PageSize)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, uploads)
}

// Delete 删除文件
// @Summary      删除文件
// @Description  根据 ID 删除上传的文件
// @Tags         文件上传
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "文件 ID"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Failure      404  {object}  pkg.Response
// @Router       /upload/{id} [delete]
func (h *uploadHandler) Delete(ctx iris.Context) {
	id := ctx.Params().Get("id")
	if err := h.svc.Delete(ctx.Request().Context(), id); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}
