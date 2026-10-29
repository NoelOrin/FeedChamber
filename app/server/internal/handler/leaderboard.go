package handler

import (
	"orinforum/internal/pkg"
	"orinforum/internal/service"

	"github.com/kataras/iris/v12"
)

type LeaderboardHandler interface {
	List(ctx iris.Context)
}

type leaderboardHandler struct{ svc service.LeaderboardService }

func newLeaderboardHandler(svc service.LeaderboardService) LeaderboardHandler {
	return &leaderboardHandler{svc: svc}
}

// 排行榜
// @Summary      排行榜
// @Description  按积分等维度分页返回用户排行（公开接口）。
// @Tags         排行榜
// @Accept       json
// @Produce      json
// @Param        body body object{page=int,page_size=int,type=string} false "分页与榜单类型"
// @Success      200  {object}  pkg.Response
// @Router       /leaderboard/list [post]
func (h *leaderboardHandler) List(ctx iris.Context) {
	var body struct {
		ListParams
		Type string `json:"type"`
	}
	_ = ctx.ReadJSON(&body)
	body.Normalize()
	if body.Type == "" {
		body.Type = "points"
	}
	items, total, err := h.svc.List(ctx.Request().Context(), body.Type, body.Page, body.PageSize)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	out := make([]iris.Map, 0, len(items))
	for _, u := range items {
		name := u.DisplayName
		if name == "" {
			name = u.Username
		}
		out = append(out, iris.Map{
			"id":             u.ID,
			"username":       u.Username,
			"display_name":   name,
			"avatar":         u.Avatar,
			"total_xp":       u.TotalXP,
			"total_checkins": u.TotalCheckIns,
			"post_count":     u.PostCount,
			"trust_level":    u.TrustLevel,
			"bio":            u.Bio,
		})
	}
	pkg.SuccessPaginated(ctx, out, total)
}
