package handler

import (
	"feedchamber/internal/mcp"
	"feedchamber/internal/pkg"
	"feedchamber/internal/service"

	"github.com/kataras/iris/v12"
)

// MCPServerHandler 管理 MCP 相关能力：
//   - 外部 MCP Server 登记（P3，能力源方向）
//   - 本站 MCP Server 的工具启停（P2，对外暴露方向）
type MCPServerHandler interface {
	List(ctx iris.Context)
	Create(ctx iris.Context)
	Update(ctx iris.Context)
	Delete(ctx iris.Context)
	Test(ctx iris.Context)
	// 本站 MCP 工具管理
	ListTools(ctx iris.Context)
	SetToolEnabled(ctx iris.Context)
}

type mcpServerHandler struct {
	svc  service.MCPServerService
	host *mcp.Handler
}

func newMCPServerHandler(svc service.MCPServerService) MCPServerHandler {
	return &mcpServerHandler{svc: svc}
}

// SetHost 注入本站 MCP Server 的协议入口，用于读取/切换工具启停。
// 用 setter 而不是构造参数：构造顺序上 MCP 晚于 handler 完成装配。
func (h *mcpServerHandler) SetHost(host *mcp.Handler) { h.host = host }

// 外部 MCP 服务列表
// @Summary      外部 MCP 服务列表
// @Description  列出已登记的外部 MCP Server。鉴权头只返回值键名，不返回凭据。
// @Tags         外部 MCP
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Router       /admin/mcp-servers/list [post]
func (h *mcpServerHandler) List(ctx iris.Context) {
	items, err := h.svc.List(ctx.Request().Context())
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, items)
}

// 登记外部 MCP 服务
// @Summary      登记外部 MCP 服务
// @Tags         外部 MCP
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Router       /admin/mcp-servers/create [post]
func (h *mcpServerHandler) Create(ctx iris.Context) {
	var in service.MCPServerInput
	if err := ctx.ReadJSON(&in); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	view, err := h.svc.Create(ctx.Request().Context(), in)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, view)
}

// 更新外部 MCP 服务
// @Summary      更新外部 MCP 服务
// @Description  headers 留空表示保持原有凭据不变。
// @Tags         外部 MCP
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "服务 ID"
// @Success      200  {object}  pkg.Response
// @Router       /admin/mcp-servers/{id}/update [post]
func (h *mcpServerHandler) Update(ctx iris.Context) {
	var in service.MCPServerInput
	if err := ctx.ReadJSON(&in); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	view, err := h.svc.Update(ctx.Request().Context(), ctx.Params().Get("id"), in)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, view)
}

// 删除外部 MCP 服务
// @Summary      删除外部 MCP 服务
// @Tags         外部 MCP
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "服务 ID"
// @Success      200  {object}  pkg.Response
// @Router       /admin/mcp-servers/{id}/delete [post]
func (h *mcpServerHandler) Delete(ctx iris.Context) {
	if err := h.svc.Delete(ctx.Request().Context(), ctx.Params().Get("id")); err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, nil)
}

// 测试外部 MCP 服务连通性
// @Summary      测试连通性
// @Description  执行 initialize + tools/list，返回工具清单或失败原因。不改动已保存配置。
// @Tags         外部 MCP
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "服务 ID"
// @Success      200  {object}  pkg.Response
// @Router       /admin/mcp-servers/{id}/test [post]
func (h *mcpServerHandler) Test(ctx iris.Context) {
	result, err := h.svc.TestConnection(ctx.Request().Context(), ctx.Params().Get("id"))
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, result)
}

// 本站 MCP 工具清单
// @Summary      MCP 工具清单
// @Description  列出本站 MCP Server 暴露的全部工具及其启停状态（含未启用的写工具）。
// @Tags         MCP
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Router       /admin/mcp/tools [post]
func (h *mcpServerHandler) ListTools(ctx iris.Context) {
	if h.host == nil {
		pkg.Success(ctx, map[string]any{"tools": []any{}, "enabled": false})
		return
	}
	pkg.Success(ctx, map[string]any{"tools": h.host.ToolStates()})
}

// 启停 MCP 工具
// @Summary      启停 MCP 工具
// @Description  写类工具默认停用；启用后调用方即可通过 /mcp 调用。
// @Tags         MCP
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Router       /admin/mcp/tools/status [post]
func (h *mcpServerHandler) SetToolEnabled(ctx iris.Context) {
	if h.host == nil {
		pkg.Fail(ctx, pkg.INTERNAL_ERROR, "MCP 服务未装配")
		return
	}
	var in struct {
		Name    string `json:"name"`
		Enabled bool   `json:"enabled"`
	}
	if err := ctx.ReadJSON(&in); err != nil {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if in.Name == "" {
		pkg.Fail(ctx, pkg.INVALID_PARAMS, "name 不能为空")
		return
	}
	if !h.host.SetToolEnabled(in.Name, in.Enabled) {
		pkg.Fail(ctx, pkg.NOT_FOUND, "工具不存在")
		return
	}
	pkg.Success(ctx, map[string]any{"name": in.Name, "enabled": in.Enabled})
}
