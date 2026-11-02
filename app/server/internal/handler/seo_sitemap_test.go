package handler

import (
	"context"
	"testing"

	"feedchamber/internal/model"
	"feedchamber/internal/service"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/httptest"
	"github.com/stretchr/testify/assert"
)

// stubSpaceService 让 sitemap 测试只关心「哪些版面可见」，不触达业务层。
type stubSpaceService struct {
	service.SpaceService
	flat  []model.Space
	index service.VisibilityIndex
}

func (s *stubSpaceService) Flat(context.Context) ([]model.Space, error) { return s.flat, nil }

func (s *stubSpaceService) VisibilityIndex(context.Context) (service.VisibilityIndex, error) {
	return s.index, nil
}

// 爬虫面固定匿名（TL0）：login / level 版面的落地页对爬虫是 403，
// 不能出现在 /sitemap-spaces.xml 里，否则 sitemap 与「可见性一致」冲突。
func TestSitemapSpaces_ExcludesSpacesAnonymousCannotRead(t *testing.T) {
	svc := &stubSpaceService{
		flat: []model.Space{
			{ID: "sp-public", Name: "public", Slug: "public", IsActive: true},
			{ID: "sp-login", Name: "login", Slug: "login", IsActive: true},
			{ID: "sp-level", Name: "vip", Slug: "vip", IsActive: true},
			{ID: "sp-off", Name: "off", Slug: "off", IsActive: false},
		},
		index: service.VisibilityIndex{
			"sp-public": {Visibility: model.SpaceVisibilityPublic},
			"sp-login":  {Visibility: model.SpaceVisibilityLogin},
			"sp-level":  {Visibility: model.SpaceVisibilityLevel, MinLevel: 3},
			"sp-off":    {Visibility: model.SpaceVisibilityPublic},
		},
	}

	app := iris.New()
	app.Get("/sitemap-spaces.xml", newSEOHandler(nil, svc, nil, nil).SitemapSpaces)
	e := httptest.New(t, app)

	body := e.GET("/sitemap-spaces.xml").Expect().Status(200).Body().Raw()
	assert.Contains(t, body, "/spaces/public")
	assert.NotContains(t, body, "/spaces/login")
	assert.NotContains(t, body, "/spaces/vip")
	assert.NotContains(t, body, "/spaces/off")
}
