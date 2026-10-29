package handler

import (
	"feedchamber/internal/pkg"
	"feedchamber/internal/service"
	"strconv"
	"time"

	"github.com/kataras/iris/v12"
)

type CheckInHandler interface {
	CheckIn(ctx iris.Context)
	GetStatus(ctx iris.Context)
	GetHistory(ctx iris.Context)
}

type checkInHandler struct {
	svc service.CheckInService
}

func newCheckInHandler(svc service.CheckInService) CheckInHandler {
	return &checkInHandler{svc: svc}
}

// CheckIn 每日签到
// @Summary      每日签到
// @Description  用户每日签到，获取积分
// @Tags         签到
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response{data=object{points_earned=int,streak=int}}
// @Failure      401  {object}  pkg.Response
// @Failure      409  {object}  pkg.Response  "今日已签到"
// @Router       /checkin [post]
func (h *checkInHandler) CheckIn(ctx iris.Context) {
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	ci, err := h.svc.CheckIn(ctx.Request().Context(), u.ID)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, iris.Map{
		"points_earned": ci.PointsEarned,
		"streak":        ci.Streak,
	})
}

// GetStatus 获取签到状态
// @Summary      获取签到状态
// @Description  获取今日签到状态和用户签到统计
// @Tags         签到
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response{data=object{checked_in_today=bool,current_streak=int,max_streak=int,total_checkins=int,total_xp=int}}
// @Failure      401  {object}  pkg.Response
// @Router       /checkin/status [get]
func (h *checkInHandler) GetStatus(ctx iris.Context) {
	u, ok := requireUser(ctx)
	if !ok {
		return
	}
	status, err := h.svc.GetStatus(ctx.Request().Context(), u.ID)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, status)
}

// GetHistory 获取签到历史
// @Summary      获取签到历史
// @Description  按月获取签到历史记录
// @Tags         签到
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        year    query     int  false  "年份"
// @Param        month   query     int  false  "月份"
// @Success      200  {object}  pkg.Response{data=object{list=[]model.CheckIn}}
// @Failure      401  {object}  pkg.Response
// @Router       /checkin/history [get]
func (h *checkInHandler) GetHistory(ctx iris.Context) {
	u, ok := requireUser(ctx)
	if !ok {
		return
	}

	now := time.Now()
	year, _ := strconv.Atoi(ctx.URLParamDefault("year", strconv.Itoa(now.Year())))
	month, _ := strconv.Atoi(ctx.URLParamDefault("month", strconv.Itoa(int(now.Month()))))

	if year < now.Year()-10 || year > now.Year()+1 {
		year = now.Year()
	}
	if month < 1 || month > 12 {
		month = int(now.Month())
	}

	records, err := h.svc.GetHistory(ctx.Request().Context(), u.ID, year, month)
	if err != nil {
		pkg.HandleError(ctx, err)
		return
	}
	pkg.Success(ctx, iris.Map{"items": records})
}
