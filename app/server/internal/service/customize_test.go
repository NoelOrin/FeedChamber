package service

import (
	"context"
	"testing"

	"feedchamber/internal/model"
	"feedchamber/internal/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── 测试替身 ──────────────────────────────────────────────────

type memSnippetRepo struct{ items map[string]*model.Snippet }

func newMemSnippetRepo() *memSnippetRepo {
	return &memSnippetRepo{items: map[string]*model.Snippet{}}
}
func (r *memSnippetRepo) Create(_ context.Context, s *model.Snippet) error {
	r.items[s.ID] = s
	return nil
}
func (r *memSnippetRepo) Update(_ context.Context, s *model.Snippet) error {
	if _, ok := r.items[s.ID]; !ok {
		return repository.ErrNotFound
	}
	r.items[s.ID] = s
	return nil
}
func (r *memSnippetRepo) Get(_ context.Context, id string) (*model.Snippet, error) {
	s, ok := r.items[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return s, nil
}
func (r *memSnippetRepo) List(_ context.Context) ([]model.Snippet, error) {
	out := make([]model.Snippet, 0, len(r.items))
	for _, s := range r.items {
		out = append(out, *s)
	}
	return out, nil
}
func (r *memSnippetRepo) ListEnabled(_ context.Context) ([]model.Snippet, error) {
	out := make([]model.Snippet, 0)
	for _, s := range r.items {
		if s.Enabled {
			out = append(out, *s)
		}
	}
	return out, nil
}
func (r *memSnippetRepo) Delete(_ context.Context, id string) error {
	delete(r.items, id)
	return nil
}
func (r *memSnippetRepo) SetEnabled(_ context.Context, id string, enabled bool, updatedBy string) error {
	s, ok := r.items[id]
	if !ok {
		return repository.ErrNotFound
	}
	s.Enabled = enabled
	s.UpdatedBy = updatedBy
	return nil
}

// memTokenRepo 按 (token, mode) 建键，与真实仓储的复合身份一致。
type memTokenRepo struct{ items map[string]*model.ThemeToken }

func tokenKey(token, mode string) string { return token + "|" + mode }

func newMemTokenRepo() *memTokenRepo { return &memTokenRepo{items: map[string]*model.ThemeToken{}} }
func (r *memTokenRepo) Upsert(_ context.Context, t *model.ThemeToken) error {
	r.items[tokenKey(t.Token, t.Mode)] = t
	return nil
}
func (r *memTokenRepo) GetByToken(_ context.Context, token string) (*model.ThemeToken, error) {
	for _, t := range r.items {
		if t.Token == token {
			return t, nil
		}
	}
	return nil, repository.ErrNotFound
}
func (r *memTokenRepo) List(_ context.Context) ([]model.ThemeToken, error) {
	out := make([]model.ThemeToken, 0, len(r.items))
	for _, t := range r.items {
		out = append(out, *t)
	}
	return out, nil
}
func (r *memTokenRepo) DeleteByToken(_ context.Context, token string) error {
	return r.DeleteByTokenMode(context.Background(), token, "")
}

func (r *memTokenRepo) DeleteByTokenMode(_ context.Context, token, mode string) error {
	removed := 0
	for k, t := range r.items {
		if t.Token != token {
			continue
		}
		if mode != "" && t.Mode != mode {
			continue
		}
		delete(r.items, k)
		removed++
	}
	if removed == 0 {
		return repository.ErrNotFound
	}
	return nil
}
func (r *memTokenRepo) DeleteAll(_ context.Context) error {
	r.items = map[string]*model.ThemeToken{}
	return nil
}

type memNavRepo struct{ items map[string]*model.NavItem }

func newMemNavRepo() *memNavRepo                                       { return &memNavRepo{items: map[string]*model.NavItem{}} }
func (r *memNavRepo) Create(_ context.Context, n *model.NavItem) error { r.items[n.ID] = n; return nil }
func (r *memNavRepo) Update(_ context.Context, n *model.NavItem) error {
	if _, ok := r.items[n.ID]; !ok {
		return repository.ErrNotFound
	}
	r.items[n.ID] = n
	return nil
}
func (r *memNavRepo) Get(_ context.Context, id string) (*model.NavItem, error) {
	n, ok := r.items[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return n, nil
}
func (r *memNavRepo) List(_ context.Context) ([]model.NavItem, error) {
	out := make([]model.NavItem, 0, len(r.items))
	for _, n := range r.items {
		out = append(out, *n)
	}
	return out, nil
}
func (r *memNavRepo) ListEnabled(_ context.Context) ([]model.NavItem, error) {
	out := make([]model.NavItem, 0)
	for _, n := range r.items {
		if n.Enabled {
			out = append(out, *n)
		}
	}
	return out, nil
}
func (r *memNavRepo) Delete(_ context.Context, id string) error { delete(r.items, id); return nil }

type memPageRepo struct{ items map[string]*model.CustomPage }

func newMemPageRepo() *memPageRepo { return &memPageRepo{items: map[string]*model.CustomPage{}} }
func (r *memPageRepo) Create(_ context.Context, p *model.CustomPage) error {
	r.items[p.ID] = p
	return nil
}
func (r *memPageRepo) Update(_ context.Context, p *model.CustomPage) error {
	if _, ok := r.items[p.ID]; !ok {
		return repository.ErrNotFound
	}
	r.items[p.ID] = p
	return nil
}
func (r *memPageRepo) Get(_ context.Context, id string) (*model.CustomPage, error) {
	p, ok := r.items[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return p, nil
}
func (r *memPageRepo) GetBySlug(_ context.Context, slug string) (*model.CustomPage, error) {
	for _, p := range r.items {
		if p.Slug == slug {
			return p, nil
		}
	}
	return nil, repository.ErrNotFound
}
func (r *memPageRepo) List(_ context.Context, offset, limit int) ([]model.CustomPage, error) {
	out := make([]model.CustomPage, 0, len(r.items))
	for _, p := range r.items {
		out = append(out, *p)
	}
	return out, nil
}
func (r *memPageRepo) Count(_ context.Context) (int64, error) { return int64(len(r.items)), nil }
func (r *memPageRepo) ListPublished(_ context.Context) ([]model.CustomPage, error) {
	out := make([]model.CustomPage, 0)
	for _, p := range r.items {
		if p.IsPublished() {
			out = append(out, *p)
		}
	}
	return out, nil
}
func (r *memPageRepo) Delete(_ context.Context, id string) error { delete(r.items, id); return nil }
func (r *memPageRepo) SetStatus(_ context.Context, id, status, updatedBy string) error {
	p, ok := r.items[id]
	if !ok {
		return repository.ErrNotFound
	}
	p.Status = status
	p.UpdatedBy = updatedBy
	return nil
}

type memModuleRepo struct{ items map[string]*model.HomeModule }

func newMemModuleRepo() *memModuleRepo { return &memModuleRepo{items: map[string]*model.HomeModule{}} }
func (r *memModuleRepo) Create(_ context.Context, m *model.HomeModule) error {
	r.items[m.ID] = m
	return nil
}
func (r *memModuleRepo) Update(_ context.Context, m *model.HomeModule) error {
	if _, ok := r.items[m.ID]; !ok {
		return repository.ErrNotFound
	}
	r.items[m.ID] = m
	return nil
}
func (r *memModuleRepo) Get(_ context.Context, id string) (*model.HomeModule, error) {
	m, ok := r.items[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return m, nil
}
func (r *memModuleRepo) List(_ context.Context) ([]model.HomeModule, error) {
	out := make([]model.HomeModule, 0, len(r.items))
	for _, m := range r.items {
		out = append(out, *m)
	}
	return out, nil
}
func (r *memModuleRepo) ListEnabled(_ context.Context) ([]model.HomeModule, error) {
	out := make([]model.HomeModule, 0)
	for _, m := range r.items {
		if m.Enabled {
			out = append(out, *m)
		}
	}
	return out, nil
}
func (r *memModuleRepo) Delete(_ context.Context, id string) error { delete(r.items, id); return nil }

// newTestCustomizeService 装配一套内存实现。
func newTestCustomizeService() (CustomizeService, *memSnippetRepo, *memNavRepo, *memPageRepo) {
	snips := newMemSnippetRepo()
	nav := newMemNavRepo()
	pages := newMemPageRepo()
	svc := NewCustomizeService(snips, newMemTokenRepo(), nav, newMemModuleRepo(), pages, nil, nil)
	return svc, snips, nav, pages
}

// 两个 actor：人工后台操作，与机器凭证（可带 / 不带发布权限）。
var (
	uiActor            = WriteActor{UserID: "admin1", Source: WriteSourceUI, CanPublish: true}
	machineNoPublish   = WriteActor{UserID: "u1", Source: WriteSourceMachine, CanPublish: false}
	machineWithPublish = WriteActor{UserID: "u1", Source: WriteSourceMachine, CanPublish: true}
)

// ── 9.4 分级生效 ──────────────────────────────────────────────

// TestCreateSnippet_MachineJSForcedDraft 是分级策略的核心断言：
// 机器凭证写 JS 必须落草稿，且响应必须如实说明「未生效」。
func TestCreateSnippet_MachineJSForcedDraft(t *testing.T) {
	svc, _, _, _ := newTestCustomizeService()
	snippet, res, err := svc.CreateSnippet(context.Background(), SnippetInput{
		Kind: model.SnippetKindJS, Content: "alert(1)", Enabled: true,
	}, machineNoPublish)
	require.NoError(t, err)
	assert.False(t, snippet.Enabled, "机器写入的 JS 必须存为未启用")
	assert.False(t, res.Applied, "响应必须如实说明未生效")
	assert.NotEmpty(t, res.Reason, "未生效时必须给出原因")
}

// TestCreateSnippet_MachineCSSAppliesDirectly CSS 是低风险，直接生效。
func TestCreateSnippet_MachineCSSAppliesDirectly(t *testing.T) {
	svc, _, _, _ := newTestCustomizeService()
	snippet, res, err := svc.CreateSnippet(context.Background(), SnippetInput{
		Kind: model.SnippetKindCSS, Content: "body{color:red}", Enabled: true,
	}, machineNoPublish)
	require.NoError(t, err)
	assert.True(t, snippet.Enabled, "CSS 片段应直接生效")
	assert.True(t, res.Applied)
}

// TestCreateSnippet_MachineWithPublishApplies 持发布权限的机器凭证可直接生效。
func TestCreateSnippet_MachineWithPublishApplies(t *testing.T) {
	svc, _, _, _ := newTestCustomizeService()
	snippet, res, err := svc.CreateSnippet(context.Background(), SnippetInput{
		Kind: model.SnippetKindJS, Content: "alert(1)", Enabled: true,
	}, machineWithPublish)
	require.NoError(t, err)
	assert.True(t, snippet.Enabled, "持 customize:publish 时应直接生效")
	assert.True(t, res.Applied)
}

// TestCreateSnippet_UIAppliesDirectly 后台人工操作不受限制。
func TestCreateSnippet_UIAppliesDirectly(t *testing.T) {
	svc, _, _, _ := newTestCustomizeService()
	snippet, res, err := svc.CreateSnippet(context.Background(), SnippetInput{
		Kind: model.SnippetKindJS, Content: "alert(1)", Enabled: true,
	}, uiActor)
	require.NoError(t, err)
	assert.True(t, snippet.Enabled)
	assert.True(t, res.Applied)
}

// TestSetSnippetEnabled_MachineCannotEnableHighRisk
// 「启用」动作本身等于让 JS 上线，必须与写入同一判据。
func TestSetSnippetEnabled_MachineCannotEnableHighRisk(t *testing.T) {
	svc, snips, _, _ := newTestCustomizeService()
	// 先由人工建一个停用的 JS 片段。
	snippet, _, err := svc.CreateSnippet(context.Background(), SnippetInput{
		Kind: model.SnippetKindJS, Content: "alert(1)", Enabled: false,
	}, uiActor)
	require.NoError(t, err)

	// 机器凭证试图启用它 → 必须被拒。
	err = svc.SetSnippetEnabled(context.Background(), snippet.ID, true, machineNoPublish)
	require.Error(t, err, "机器凭证不得启用高风险片段")
	stored := snips.items[snippet.ID]
	assert.False(t, stored.Enabled, "片段状态不应改变")
}

// TestNavItem_MachineForcedDraft 导航项同属高风险（会让 AI 改站点信息架构）。
func TestNavItem_MachineForcedDraft(t *testing.T) {
	svc, _, nav, _ := newTestCustomizeService()
	item, res, err := svc.CreateNavItem(context.Background(), NavItemInput{
		Label: "恶意链接", Href: "https://evil.example", Visibility: model.NavVisibilityPublic, Enabled: true,
	}, machineNoPublish)
	require.NoError(t, err)
	assert.False(t, item.Enabled)
	assert.False(t, res.Applied)
	assert.False(t, nav.items[item.ID].Enabled)
}

// TestCustomPage_MachineForcedDraft 机器建页必须落草稿；草稿对访客是 404。
func TestCustomPage_MachineForcedDraft(t *testing.T) {
	svc, _, _, pages := newTestCustomizeService()
	page, res, err := svc.CreateCustomPage(context.Background(), CustomPageInput{
		Slug: "evil", Title: "钓鱼页", Content: "# hi", Status: model.CustomPagePublished,
	}, machineNoPublish)
	require.NoError(t, err)
	assert.Equal(t, model.CustomPageDraft, page.Status)
	assert.False(t, res.Applied)
	assert.False(t, pages.items[page.ID].IsPublished())

	// 草稿对外表现为「不存在」。
	_, err = svc.GetPublishedPage(context.Background(), "evil")
	require.Error(t, err, "草稿页不应可公开访问")
}

// TestReservedSlugRejected slug 与既有路由冲突时必须拒绝。
func TestReservedSlugRejected(t *testing.T) {
	svc, _, _, _ := newTestCustomizeService()
	for _, slug := range []string{"admin", "timeline", "spaces", "mcp", "login"} {
		_, _, err := svc.CreateCustomPage(context.Background(), CustomPageInput{
			Slug: slug, Title: "x", Status: model.CustomPageDraft,
		}, uiActor)
		require.Error(t, err, "slug %q 与既有路由冲突，必须拒绝", slug)
	}
}

// TestHomeModuleHTML_MachineForcedDraft html 模块等价于可执行标记。
func TestHomeModuleHTML_MachineForcedDraft(t *testing.T) {
	svc, _, _, _ := newTestCustomizeService()
	m, res, err := svc.CreateHomeModule(context.Background(), HomeModuleInput{
		// config 必须是合法 JSON（HTML 内容作为 JSON 字符串承载）。
		Kind: model.HomeModuleHTML, Config: `{"html":"<script>alert(1)</script>"}`,
		Visibility: model.NavVisibilityPublic, Enabled: true,
	}, machineNoPublish)
	require.NoError(t, err)
	assert.False(t, m.Enabled)
	assert.False(t, res.Applied)
}

// TestNavVisibilityFilter 可见性过滤：游客看不到「仅登录」项。
func TestNavVisibilityFilter(t *testing.T) {
	svc, _, _, _ := newTestCustomizeService()
	ctx := context.Background()
	_, _, err := svc.CreateNavItem(ctx, NavItemInput{
		Label: "公开项", Href: "/a", Visibility: model.NavVisibilityPublic, Enabled: true,
	}, uiActor)
	require.NoError(t, err)
	_, _, err = svc.CreateNavItem(ctx, NavItemInput{
		Label: "登录项", Href: "/b", Visibility: model.NavVisibilityLogin, Enabled: true,
	}, uiActor)
	require.NoError(t, err)

	guest, err := svc.PublicNav(ctx, false, 0)
	require.NoError(t, err)
	assert.Len(t, guest, 1, "游客只应看到公开项")

	loggedIn, err := svc.PublicNav(ctx, true, 0)
	require.NoError(t, err)
	assert.Len(t, loggedIn, 2, "登录用户应看到两项")
}
