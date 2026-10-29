package handler

import (
	"io"

	"orinforum/internal/pkg"
	"orinforum/internal/service"

	"github.com/kataras/iris/v12"
	"github.com/sirupsen/logrus"
)

type UploadHandler interface {
	Upload(ctx iris.Context)
	Presign(ctx iris.Context)
	Confirm(ctx iris.Context)
	GetFile(ctx iris.Context)
	GetFileByKey(ctx iris.Context)
	List(ctx iris.Context)
	Delete(ctx iris.Context)
}

type uploadHandler struct{ svc service.UploadService }

func newUploadHandler(svc service.UploadService) UploadHandler {
	return &uploadHandler{svc: svc}
}

// Upload 上传文件（服务端代理模式，本地存储使用）
// @Summary      上传文件
// @Description  通过服务端代理上传图片文件（jpg/png/gif/webp），最大 10MB
// @Tags         文件上传
// @Accept       multipart/form-data
// @Produce      json
// @Security     BearerAuth
// @Param        file  formData  file  true  "上传的文件"
// @Success      201   {object}  pkg.Response{data=object{id=string,url=string,fileName=string,size=int}}
// @Failure      400   {object}  pkg.Response
// @Failure      401   {object}  pkg.Response
// @Router       /upload [post]
func (h *uploadHandler) Upload(ctx iris.Context) {
	u, ok := requireUser(ctx)
	if !ok {
		return
	}

	_, fileHeader, err := ctx.FormFile("file")
	if err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "请选择文件")
		return
	}

	upload, err := h.svc.Save(ctx.Request().Context(), u.ID, fileHeader)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}

	ctx.StatusCode(iris.StatusCreated)
	pkg.Success(ctx, iris.Map{
		"id":       upload.ID,
		"url":      upload.URL,
		"fileName": upload.FileName,
		"size":     upload.Size,
	})
}

// Presign 获取预签名 POST 上传信息（前端直传模式）
// @Summary      获取预签名上传信息
// @Description  返回预签名 POST URL 和字段，前端直接上传文件到对象存储
// @Tags         文件上传
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  object{fileName=string,contentType=string}  true  "文件信息"
// @Success      200   {object}  pkg.Response{data=object{uploadId=string,url=string,urlMap=object,fileName=string}}
// @Failure      400   {object}  pkg.Response
// @Failure      401   {object}  pkg.Response
// @Router       /upload/presign [post]
func (h *uploadHandler) Presign(ctx iris.Context) {
	u, ok := requireUser(ctx)
	if !ok {
		return
	}

	var body struct {
		FileName    string `json:"file_name"`
		ContentType string `json:"content_type"`
	}
	if err := ctx.ReadJSON(&body); err != nil || body.FileName == "" || body.ContentType == "" {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "文件名和内容类型不能为空")
		return
	}

	post, upload, err := h.svc.Presign(ctx.Request().Context(), u.ID, body.FileName, body.ContentType)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}

	pkg.Success(ctx, iris.Map{
		"uploadId": upload.ID,
		"url":      post.URL,
		"fields":   post.Fields,
		"fileUrl":  upload.URL,
	})
}

// Confirm 确认直传完成，返回文件元数据
// @Summary      确认上传完成
// @Description  前端直传完成后回调，返回文件信息
// @Tags         文件上传
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  object{uploadId=string}  true  "上传 ID"
// @Success      200   {object}  pkg.Response{data=object{id=string,url=string,fileName=string,size=int}}
// @Failure      400   {object}  pkg.Response
// @Failure      401   {object}  pkg.Response
// @Router       /upload/confirm [post]
func (h *uploadHandler) Confirm(ctx iris.Context) {
	u, ok := requireUser(ctx)
	if !ok {
		return
	}

	var body struct {
		UploadID string `json:"upload_id"`
	}
	if err := ctx.ReadJSON(&body); err != nil || body.UploadID == "" {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "上传ID不能为空")
		return
	}

	upload, err := h.svc.GetByID(ctx.Request().Context(), body.UploadID)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}

	// 所有权校验在 service 操作之前
	if upload.UploaderID != u.ID {
		pkg.Fail(ctx, pkg.FORBIDDEN)
		return
	}

	pkg.Success(ctx, iris.Map{
		"id":       upload.ID,
		"url":      upload.URL,
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
	if id == "" {
		pkg.Fail(ctx, pkg.NOT_FOUND, "文件 ID 不能为空")
		return
	}
	upload, err := h.svc.GetByID(ctx.Request().Context(), id)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	if upload.URL != "" {
		ctx.Redirect(upload.URL, iris.StatusFound)
		return
	}
	pkg.Fail(ctx, pkg.NOT_FOUND, "文件不可访问")
}

// GetFileByKey 根据存储 key 获取上传文件，供 PublicURL 使用。
// @Summary      根据存储 key 获取上传文件
// @Tags         文件上传
// @Produce      application/octet-stream
// @Param        key  path  string  true  "文件存储 key"
// @Success      200  {file}  binary
// @Failure      404  {object}  pkg.Response
// @Router       /upload/file/{key} [get]
func (h *uploadHandler) GetFileByKey(ctx iris.Context) {
	key := ctx.Params().Get("key")
	if key == "" {
		pkg.Fail(ctx, pkg.NOT_FOUND, "文件 key 不能为空")
		return
	}
	reader, upload, err := h.svc.OpenByKey(ctx.Request().Context(), key)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	defer reader.Close()
	if upload.MimeType != "" {
		ctx.ContentType(upload.MimeType)
	}
	if _, err := io.Copy(ctx.ResponseWriter(), reader); err != nil {
		logrus.Warnf("upload: stream file %s failed: %v", key, err)
	}
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
// @Router       /upload/list [post]
func (h *uploadHandler) List(ctx iris.Context) {
	u, ok := requireUser(ctx)
	if !ok {
		return
	}

	p, err := readListParams(ctx)
	if err != nil {
		return
	}

	uploads, err := h.svc.ListByUser(ctx.Request().Context(), u.ID, p.Page, p.PageSize)
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
// @Router       /upload/{id}/delete [post]
func (h *uploadHandler) Delete(ctx iris.Context) {
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	id := ctx.Params().Get("id")
	isAdmin := u.Role == "admin" || u.Role == "moderator"
	if err := h.svc.Delete(ctx.Request().Context(), id, u.ID, isAdmin); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}
