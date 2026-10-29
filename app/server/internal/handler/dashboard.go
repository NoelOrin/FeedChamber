package handler

import (
	"context"
	"sync"
	"time"

	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"orinforum/internal/service"

	"github.com/kataras/iris/v12"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

const dashboardTimeout = 5 * time.Second

// HomeHandler 把首页需要的三份数据（用户资料、关注计数、已发布公告）合成一次请求。
type HomeHandler interface {
	Dashboard(ctx iris.Context)
}

type homeHandler struct {
	users        service.UserService
	follow       service.FollowService
	announcement service.AnnouncementService
}

func newHomeHandler(users service.UserService, follow service.FollowService, announcement service.AnnouncementService) HomeHandler {
	return &homeHandler{users: users, follow: follow, announcement: announcement}
}

type dashboardResponse struct {
	User          *model.UserResponse `json:"user"`
	FollowCounts  *followCountsResp   `json:"follow_counts"`
	Announcements *announcementsResp  `json:"announcements"`
}

type followCountsResp struct {
	Followers int64 `json:"followers"`
	Following int64 `json:"following"`
}

type announcementsResp struct {
	Items []model.Announcement `json:"items"`
}

// Dashboard 聚合首页所需数据，1 次请求替代前端 3 次（profile + counts + announcements）。
// @Summary      首页聚合数据
// @Description  聚合当前用户的个人信息、关注计数和已发布公告
// @Tags         用户
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response{data=object{user=model.UserResponse,follow_counts=object{followers=int,following=int},announcements=object{items=[]model.Announcement}}}
// @Failure      401  {object}  pkg.Response
// @Router       /dashboard [post]
func (h *homeHandler) Dashboard(ctx iris.Context) {
	u, ok := requireUser(ctx)
	if !ok {
		return
	}

	reqCtx := ctx.Request().Context()
	dashCtx, cancel := context.WithTimeout(reqCtx, dashboardTimeout)
	defer cancel()

	resp := &dashboardResponse{
		Announcements: &announcementsResp{Items: []model.Announcement{}},
	}

	g, gCtx := errgroup.WithContext(dashCtx)

	// User profile
	g.Go(func() error {
		user, err := h.users.GetByID(gCtx, u.ID)
		if err != nil {
			logrus.Warnf("dashboard: user fetch failed: %v", err)
			return nil // non-fatal
		}
		r := user.ToResponse()
		resp.User = &r
		return nil
	})

	// Follow counts — run CountFollowers and CountFollowing in parallel
	// to avoid one slow count starving the other of the shared timeout budget.
	var followers, following int64
	g.Go(func() error {
		var inner sync.WaitGroup
		inner.Add(2)
		go func() {
			defer inner.Done()
			if v, err := h.follow.CountFollowers(gCtx, u.ID); err == nil {
				followers = v
			} else {
				logrus.Warnf("dashboard: count followers failed: %v", err)
			}
		}()
		go func() {
			defer inner.Done()
			if v, err := h.follow.CountFollowing(gCtx, u.ID); err == nil {
				following = v
			} else {
				logrus.Warnf("dashboard: count following failed: %v", err)
			}
		}()
		inner.Wait()
		resp.FollowCounts = &followCountsResp{Followers: followers, Following: following}
		return nil
	})

	// Published announcements
	g.Go(func() error {
		items, err := h.announcement.ListPublished(gCtx, 1, 20)
		if err != nil {
			logrus.Warnf("dashboard: announcements failed: %v", err)
			return nil // non-fatal
		}
		if items == nil {
			items = []model.Announcement{}
		}
		resp.Announcements = &announcementsResp{Items: items}
		return nil
	})

	if err := g.Wait(); err != nil {
		logrus.Errorf("dashboard: unexpected errgroup error: %v", err)
	}

	pkg.Success(ctx, resp)
}
