package handler

import (
	"io"
	"strconv"

	"feedchamber/internal/pkg"
	"feedchamber/internal/service"

	"github.com/kataras/iris/v12"
)

type BackupHandler interface {
	Create(ctx iris.Context)
	Cleanup(ctx iris.Context)
	Restore(ctx iris.Context)
	List(ctx iris.Context)
	Download(ctx iris.Context)
	Delete(ctx iris.Context)
}

type backupHandler struct{ svc service.BackupService }

func newBackupHandler(svc service.BackupService) BackupHandler {
	return &backupHandler{svc: svc}
}

// 创建备份
// @Summary      创建备份
// @Description  生成数据库与配置的快照，可在备份列表中下载或恢复。
// @Tags         备份
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object{name=string} false "备份名称"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /admin/backups/create [post]
func (h *backupHandler) Create(ctx iris.Context) {
	var body struct {
		Name string `json:"name"`
	}
	_ = ctx.ReadJSON(&body)
	backup, err := h.svc.Create(ctx.Request().Context(), currentUserID(ctx), body.Name)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, backup)
}

// 备份列表
// @Summary      备份列表
// @Description  分页列出已生成的备份文件。
// @Tags         备份
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object{page=int,page_size=int} false "分页参数"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /admin/backups/list [post]
func (h *backupHandler) List(ctx iris.Context) {
	var body struct {
		Page     int `json:"page"`
		PageSize int `json:"page_size"`
	}
	_ = ctx.ReadJSON(&body)
	page, pageSize := pkg.NormalizePagination(body.Page, body.PageSize)
	items, total, err := h.svc.List(ctx.Request().Context(), page, pageSize)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.SuccessPaginated(ctx, items, total)
}

// 清理过期备份
// @Summary      清理过期备份
// @Description  按保留天数设置删除过期备份，返回删除数量。
// @Tags         备份
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /admin/backups/cleanup [post]
func (h *backupHandler) Cleanup(ctx iris.Context) {
	count, err := h.svc.Cleanup(ctx.Request().Context())
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, iris.Map{"removed": count})
}

// 恢复备份
// @Summary      恢复备份
// @Description  用指定备份覆盖当前数据（高危操作）。
// @Tags         备份
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "资源 ID"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /admin/backups/{id}/restore [post]
func (h *backupHandler) Restore(ctx iris.Context) {
	if err := h.svc.Restore(ctx.Request().Context(), ctx.Params().Get("id"), currentUserID(ctx)); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}

// 下载备份
// @Summary      下载备份
// @Description  以附件形式下载备份 JSON 文件。
// @Tags         备份
// @Accept       json
// @Produce      application/json
// @Security     BearerAuth
// @Param        id path string true "资源 ID"
// @Success      200  {file}  file
// @Failure      401  {object}  pkg.Response
// @Router       /admin/backups/{id}/download [post]
func (h *backupHandler) Download(ctx iris.Context) {
	backup, reader, err := h.svc.Download(ctx.Request().Context(), ctx.Params().Get("id"))
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	defer reader.Close()
	ctx.ContentType("application/json")
	ctx.Header("Content-Disposition", "attachment; filename="+strconv.Quote(backup.Name+".json"))
	_, _ = io.Copy(ctx.ResponseWriter(), reader)
}

// 删除备份
// @Summary      删除备份
// @Description  删除指定备份文件。
// @Tags         备份
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "资源 ID"
// @Success      200  {object}  pkg.Response
// @Failure      401  {object}  pkg.Response
// @Router       /admin/backups/{id}/delete [post]
func (h *backupHandler) Delete(ctx iris.Context) {
	if err := h.svc.Delete(ctx.Request().Context(), ctx.Params().Get("id")); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}
