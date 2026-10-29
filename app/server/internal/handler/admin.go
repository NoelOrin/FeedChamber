package handler

import (
	"encoding/json"
	"log/slog"
	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"orinforum/internal/service"
	"strconv"

	"github.com/kataras/iris/v12"
	"github.com/sirupsen/logrus"
)

type AdminHandler interface {
	GetStats(ctx iris.Context)
	Dashboard(ctx iris.Context)
	Search(ctx iris.Context)
	GetUserActivities(ctx iris.Context)
	BatchUsers(ctx iris.Context)
	BatchPosts(ctx iris.Context)
	BatchComments(ctx iris.Context)
	ReconcileCounters(ctx iris.Context)
	ListAuditLogs(ctx iris.Context)
}

type adminHandler struct {
	svc         service.AdminService
	auditLogSvc service.AuditLogService
}

func newAdminHandler(svc service.AdminService, auditLogSvc service.AuditLogService) AdminHandler {
	return &adminHandler{svc: svc, auditLogSvc: auditLogSvc}
}

// GetStats 获取论坛统计数据
// @Summary      获取论坛统计数据
// @Description  获取用户、帖子、评论、分类的总数及今日新增数量
// @Tags         管理后台
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response{data=service.AdminStats}
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Router       /admin/stats [post]
func (h *adminHandler) GetStats(ctx iris.Context) {
	stats, err := h.svc.GetStats(ctx.Request().Context())
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, stats)
}

// Dashboard 运营概览
// @Summary      运营概览
// @Description  获取聚合的运营概览数据，包括统计、待处理举报、近期动态
// @Tags         管理后台
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response{data=service.DashboardData}
// @Router       /admin/dashboard [post]
func (h *adminHandler) Dashboard(ctx iris.Context) {
	data, err := h.svc.GetDashboard(ctx.Request().Context())
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, data)
}

type searchRequest struct {
	Query            string `json:"q"`
	Page             int    `json:"page"`
	PageSize         int    `json:"page_size"`
	UsersPage        int    `json:"users_page"`
	UsersPageSize    int    `json:"users_page_size"`
	PostsPage        int    `json:"posts_page"`
	PostsPageSize    int    `json:"posts_page_size"`
	CommentsPage     int    `json:"comments_page"`
	CommentsPageSize int    `json:"comments_page_size"`
}

// Search 全局搜索
// @Summary      全局搜索
// @Description  跨资源搜索用户、帖子、评论，各实体独立分页
// @Tags         管理后台
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body searchRequest true "搜索参数"
// @Success      200  {object}  pkg.Response{data=service.SearchResults}
// @Router       /admin/search [post]
func (h *adminHandler) Search(ctx iris.Context) {
	var req searchRequest
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "请求参数无效")
		return
	}
	// 各实体用独立分页，不传则回退到通用 page/pageSize
	up, ups := req.UsersPage, req.UsersPageSize
	if up == 0 {
		up = req.Page
	}
	if ups == 0 {
		ups = req.PageSize
	}
	pp, pps := req.PostsPage, req.PostsPageSize
	if pp == 0 {
		pp = req.Page
	}
	if pps == 0 {
		pps = req.PageSize
	}
	cp, cps := req.CommentsPage, req.CommentsPageSize
	if cp == 0 {
		cp = req.Page
	}
	if cps == 0 {
		cps = req.PageSize
	}

	results := &service.SearchResults{}
	var err error

	var users []model.User
	var usersTotal int64
	users, usersTotal, err = h.svc.SearchUsers(ctx.Request().Context(), req.Query, up, ups)
	if err != nil {
		slog.Error("admin search users failed", "query", req.Query, "error", err)
	} else {
		results.Users = &service.EntitySearchResult[model.User]{Items: users, Total: usersTotal, Page: up, PageSize: ups}
	}

	var posts []model.Post
	var postsTotal int64
	posts, postsTotal, err = h.svc.SearchPosts(ctx.Request().Context(), req.Query, pp, pps)
	if err != nil {
		slog.Error("admin search posts failed", "query", req.Query, "error", err)
	} else {
		results.Posts = &service.EntitySearchResult[model.Post]{Items: posts, Total: postsTotal, Page: pp, PageSize: pps}
	}

	var comments []model.Content
	var commentsTotal int64
	comments, commentsTotal, err = h.svc.SearchComments(ctx.Request().Context(), req.Query, cp, cps)
	if err != nil {
		slog.Error("admin search comments failed", "query", req.Query, "error", err)
	} else {
		results.Comments = &service.EntitySearchResult[model.Content]{Items: comments, Total: commentsTotal, Page: cp, PageSize: cps}
	}
	pkg.Success(ctx, results)
}

// GetUserActivities 获取用户活动记录
// @Summary      获取用户活动记录
// @Description  获取指定用户的帖子、评论、被举报记录
// @Tags         管理后台
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path string true "用户 ID"
// @Param        page     query int    false "页码" default(1)
// @Param        pageSize query int    false "每页数量" default(10)
// @Success      200  {object}  pkg.Response{data=service.UserActivities}
// @Router       /admin/user/{id}/activities [post]
func (h *adminHandler) GetUserActivities(ctx iris.Context) {
	userID := ctx.Params().Get("id")
	if userID == "" {
		pkg.Fail(ctx, pkg.INVALID_PARAMS)
		return
	}
	page, _ := strconv.Atoi(ctx.URLParamDefault("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.URLParamDefault("pageSize", "10"))
	page, pageSize = pkg.NormalizePaginationCustom(page, pageSize, 50, 10)

	activities, err := h.svc.GetUserActivities(ctx.Request().Context(), userID, page, pageSize)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, activities)
}

type batchRequest struct {
	IDs    []string          `json:"ids"`
	Action string            `json:"action"`
	Params map[string]string `json:"params,omitempty"`
}

// BatchUsers 批量用户操作
// @Summary      批量用户操作
// @Description  批量删除用户或修改角色
// @Tags         管理后台
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body batchRequest true "操作参数"
// @Success      200  {object}  pkg.Response
// @Router       /admin/batch/users [post]
func (h *adminHandler) BatchUsers(ctx iris.Context) {
	var req batchRequest
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "请求参数无效")
		return
	}
	if len(req.IDs) > 100 {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "批量操作最多支持 100 条")
		return
	}
	allowedActions := map[string]bool{"delete": true, "set_role": true}
	if !allowedActions[req.Action] {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "不支持的操作类型")
		return
	}
	if err := h.svc.BatchUserAction(ctx.Request().Context(), req.Action, req.IDs, req.Params); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	detail, _ := json.Marshal(map[string]any{"ids": req.IDs, "params": req.Params})
	if err := h.auditLogSvc.Record(ctx.Request().Context(), currentUserID(ctx), "admin.batch_users."+req.Action, "users", string(detail)); err != nil {
		logrus.Warnf("audit log write failed: %v", err)
	}
	pkg.Success(ctx, nil)
}

// BatchPosts 批量帖子操作
// @Summary      批量帖子操作
// @Description  批量删除、置顶/取消置顶、锁定/解锁帖子
// @Tags         管理后台
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body batchRequest true "操作参数"
// @Success      200  {object}  pkg.Response
// @Router       /admin/batch/posts [post]
func (h *adminHandler) BatchPosts(ctx iris.Context) {
	var req batchRequest
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "请求参数无效")
		return
	}
	if err := h.svc.BatchPostAction(ctx.Request().Context(), req.Action, req.IDs, currentRole(ctx)); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	detail, _ := json.Marshal(map[string]any{"ids": req.IDs})
	if err := h.auditLogSvc.Record(ctx.Request().Context(), currentUserID(ctx), "admin.batch_posts."+req.Action, "posts", string(detail)); err != nil {
		logrus.Warnf("audit log write failed: %v", err)
	}
	pkg.Success(ctx, nil)
}

// BatchComments 批量评论操作
// @Summary      批量删除评论
// @Description  批量删除评论
// @Tags         管理后台
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body batchRequest true "操作参数"
// @Success      200  {object}  pkg.Response
// @Router       /admin/batch/comments [post]
func (h *adminHandler) BatchComments(ctx iris.Context) {
	var req batchRequest
	if err := ctx.ReadJSON(&req); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "请求参数无效")
		return
	}
	if err := h.svc.BatchDeleteComments(ctx.Request().Context(), req.IDs); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	detail, _ := json.Marshal(map[string]any{"ids": req.IDs})
	if err := h.auditLogSvc.Record(ctx.Request().Context(), currentUserID(ctx), "admin.batch_comments.delete", "comments", string(detail)); err != nil {
		logrus.Warnf("audit log write failed: %v", err)
	}
	pkg.Success(ctx, nil)
}

// ReconcileCounters 修复帖子计数器漂移
// @Summary      修复计数器
// @Description  重新计算所有帖子的评论计数，修复反规范化计数器漂移问题
// @Tags         管理后台
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response{data=object{fixed=int}}
// @Failure      401  {object}  pkg.Response
// @Failure      403  {object}  pkg.Response
// @Router       /admin/reconcile/counters [post]
func (h *adminHandler) ReconcileCounters(ctx iris.Context) {
	fixed, err := h.svc.ReconcileCounters(ctx.Request().Context())
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, iris.Map{"fixed": fixed})
}

// ListAuditLogs 审计日志列表
// @Summary      审计日志列表
// @Description  分页查询敏感操作审计日志，可按操作者与动作过滤
// @Tags         管理后台
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body object{page=int,pageSize=int,actor_id=string,action=string} false "查询参数"
// @Success      200  {object}  pkg.Response{data=pkg.PaginatedData}
// @Router       /admin/audit_logs/list [post]
func (h *adminHandler) ListAuditLogs(ctx iris.Context) {
	var req struct {
		Page      int    `json:"page"`
		PageSize  int    `json:"pageSize"`
		PageSize2 int    `json:"page_size"`
		ActorID   string `json:"actor_id"`
		Action    string `json:"action"`
	}
	// body 允许为空
	_ = ctx.ReadJSON(&req)
	pageSize := req.PageSize
	if pageSize == 0 {
		pageSize = req.PageSize2
	}
	logs, total, err := h.auditLogSvc.List(ctx.Request().Context(), req.Page, pageSize, req.ActorID, req.Action)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.SuccessPaginated(ctx, logs, int(total))
}
