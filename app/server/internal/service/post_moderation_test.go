package service

import (
	"context"
	"errors"
	"testing"

	"feedchamber/internal/mock"
	"feedchamber/internal/model"
	"feedchamber/internal/pkg"
	"feedchamber/internal/pkg/eventbus"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type moderationRecord struct {
	targetType string
	targetID   string
	action     model.ModerationAction
	strategy   string
}

type stubModerationService struct {
	mode          string
	action        model.ModerationAction
	checkCalls    int
	external      model.ModerationAction
	externalErr   error
	externalCalls int
	pending       []moderationRecord
}

func (s *stubModerationService) ListPending(ctx context.Context, page, pageSize int) ([]model.ContentModeration, int64, error) {
	return nil, 0, nil
}
func (s *stubModerationService) Approve(ctx context.Context, id, reviewerID string) error { return nil }
func (s *stubModerationService) Reject(ctx context.Context, id, reviewerID, reason string) error {
	return nil
}
func (s *stubModerationService) CreateRule(ctx context.Context, rule *model.ModerationRule) error {
	return nil
}
func (s *stubModerationService) UpdateRule(ctx context.Context, rule *model.ModerationRule) error {
	return nil
}
func (s *stubModerationService) DeleteRule(ctx context.Context, id string) error { return nil }
func (s *stubModerationService) ListRules(ctx context.Context, page, pageSize int) ([]model.ModerationRule, int64, error) {
	return nil, 0, nil
}
func (s *stubModerationService) SetTrustService(svc TrustService) {}

// SetMCPServerService 是 P3 新增的接缝（外部审核优先走 MCP）。
// 测试替身不涉及该路径，空实现即可。
func (s *stubModerationService) SetMCPServerService(svc MCPServerService) {}

func (s *stubModerationService) Mode(ctx context.Context) string {
	if s.mode == "" {
		return model.DefaultModerationMode
	}
	return s.mode
}

func (s *stubModerationService) GetConfigs(ctx context.Context) (map[string]string, error) {
	return map[string]string{}, nil
}
func (s *stubModerationService) SetConfig(ctx context.Context, key, value string) error { return nil }
func (s *stubModerationService) CheckContent(ctx context.Context, targetType, targetID, body string, scene model.ModerationScene, authorRole string) (model.ModerationAction, error) {
	s.checkCalls++
	return s.action, nil
}
func (s *stubModerationService) CheckExternal(ctx context.Context, targetType, targetID, title, body string, scene model.ModerationScene) (model.ModerationAction, error) {
	s.externalCalls++
	return s.external, s.externalErr
}
func (s *stubModerationService) RecordPending(ctx context.Context, targetType, targetID string, scene model.ModerationScene, action model.ModerationAction, body string) error {
	return s.RecordPendingFrom(ctx, targetType, targetID, scene, action, body, "auto")
}
func (s *stubModerationService) RecordPendingFrom(ctx context.Context, targetType, targetID string, scene model.ModerationScene, action model.ModerationAction, body, strategy string) error {
	s.pending = append(s.pending, moderationRecord{
		targetType: targetType,
		targetID:   targetID,
		action:     action,
		strategy:   strategy,
	})
	return nil
}
func (s *stubModerationService) SubscribeAsync(bus *eventbus.EventBus) {}

// newModeFixture 组装一个只关心审核模式分支的 postService。
func newModeFixture(mod ModerationService, settings *mock.SettingService) (*postService, *mock.PostRepository) {
	posts := mock.NewPostRepository()
	contents := mock.NewContentRepository()
	users := mock.NewUserRepository()
	_ = users.Create(context.Background(), &model.User{ID: "u1", Username: "alice", Role: "user"})
	if settings == nil {
		settings = mock.NewSettingService()
	}
	return &postService{
		posts:         posts,
		contents:      contents,
		users:         users,
		moderationSvc: mod,
		settingSvc:    settings,
	}, posts
}

// 模式 1「不审核」：直接公开，规则引擎不跑，也不写审核记录。
func TestPostCreate_ModeOffBypassesModeration(t *testing.T) {
	mod := &stubModerationService{mode: model.ModerationModeOff, action: model.ModerationActionBlock}
	svc, posts := newModeFixture(mod, nil)

	post, err := svc.Create(context.Background(), "u1", "hello", "bad word body", "")
	require.NoError(t, err)
	assert.Equal(t, model.PostStatusApproved, post.Status)
	assert.Zero(t, mod.checkCalls, "不审核模式 MUST NOT 调用规则引擎")
	assert.Empty(t, mod.pending)

	got, err := posts.FindByID(context.Background(), post.ID)
	require.NoError(t, err)
	assert.Equal(t, model.PostStatusApproved, got.Status)
}

// 模式 2「发布后异步审核」（默认）：创建时立刻公开，同步路径不跑规则。
func TestPostCreate_ModeAsyncPublishesImmediately(t *testing.T) {
	mod := &stubModerationService{mode: model.ModerationModeAsync, action: model.ModerationActionBlock}
	svc, newPosts := newModeFixture(mod, nil)
	_ = newPosts

	post, err := svc.Create(context.Background(), "u1", "hello", "flagged body", "")
	require.NoError(t, err)
	assert.Equal(t, model.PostStatusApproved, post.Status)
	assert.Zero(t, mod.checkCalls, "异步模式 MUST NOT 在发帖请求里同步跑规则")
	assert.Empty(t, mod.pending)
}

// 模式 4「全部人工」：一律待审，且必留痕供审核队列处理。
func TestPostCreate_ModeManualLeavesPendingWithRecord(t *testing.T) {
	mod := &stubModerationService{mode: model.ModerationModeManual}
	svc, _ := newModeFixture(mod, nil)

	post, err := svc.Create(context.Background(), "u1", "hello", "clean body", "")
	require.NoError(t, err)
	assert.Equal(t, model.PostStatusPending, post.Status)
	require.Len(t, mod.pending, 1)
	assert.Equal(t, post.ID, mod.pending[0].targetID)
	assert.Equal(t, "manual", mod.pending[0].strategy)
}

// 模式 3「外部自动化审核」：发布前调用一次外部服务，flag 结论落待审 + 留痕。
func TestPostCreate_ModeExternalFlagLeavesPending(t *testing.T) {
	mod := &stubModerationService{mode: model.ModerationModeExternal, external: model.ModerationActionFlag}
	svc, _ := newModeFixture(mod, nil)

	post, err := svc.Create(context.Background(), "u1", "hello", "needs review", "")
	require.NoError(t, err)
	assert.Equal(t, 1, mod.externalCalls, "external 模式 MUST 在发布前调用外部审核服务")
	assert.Equal(t, model.PostStatusPending, post.Status)
	require.Len(t, mod.pending, 1)
	assert.Equal(t, "external", mod.pending[0].strategy)
	assert.Equal(t, model.ModerationActionFlag, mod.pending[0].action)
}

// external 判 block：内容落库为待审并留痕，同时给作者返回「含违规内容」。
func TestPostCreate_ModeExternalBlockReturnsError(t *testing.T) {
	mod := &stubModerationService{mode: model.ModerationModeExternal, external: model.ModerationActionBlock}
	svc, posts := newModeFixture(mod, nil)

	post, err := svc.Create(context.Background(), "u1", "hello", "blocked body", "")
	require.Error(t, err)
	assert.Nil(t, post)
	var appErr *pkg.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, pkg.INVALID_PARAMS, appErr.Code)
	require.Len(t, mod.pending, 1)
	assert.Equal(t, "external", mod.pending[0].strategy)

	// 帖子仍在库里（待审），作者能在自己的视角看到并等待人工结论。
	got, err := posts.FindByID(context.Background(), mod.pending[0].targetID)
	require.NoError(t, err)
	assert.Equal(t, model.PostStatusPending, got.Status)
}

// external 服务不可用：不阻塞发帖，回退为「发布后异步审核」，不发待审单。
func TestPostCreate_ModeExternalUnavailableFallsBackToAsync(t *testing.T) {
	mod := &stubModerationService{
		mode:        model.ModerationModeExternal,
		externalErr: errExternalModerationUnavailable,
	}
	svc, _ := newModeFixture(mod, nil)

	post, err := svc.Create(context.Background(), "u1", "hello", "clean body", "")
	require.NoError(t, err)
	assert.Equal(t, model.PostStatusApproved, post.Status)
	assert.Empty(t, mod.pending)
}

// 旧布尔开关 post_approval=true 继续等价于「全部人工」，升级后行为不变。
func TestPostCreate_LegacyPostApprovalForcesManual(t *testing.T) {
	mod := &stubModerationService{mode: model.ModerationModeAsync}
	svc, _ := newModeFixture(mod, mock.NewSettingService().WithSetting("post_approval", "true"))

	post, err := svc.Create(context.Background(), "u1", "hello", "needs approval", "")
	require.NoError(t, err)
	assert.Equal(t, model.PostStatusPending, post.Status)
	require.Len(t, mod.pending, 1)
	assert.Equal(t, "manual", mod.pending[0].strategy)
}

// 一旦 moderation_mode 落库，旧开关立刻失效：否则一条历史 post_approval=true
// 会把站点永久锁在「全部人工」，而新界面里没有能关掉它的开关。
func TestPostCreate_LegacyPostApprovalIgnoredOnceModeConfigured(t *testing.T) {
	mod := &stubModerationService{mode: model.ModerationModeAsync}
	settings := mock.NewSettingService().
		WithSetting("post_approval", "true").
		WithSetting(model.SettingModerationMode, model.ModerationModeAsync)
	svc, _ := newModeFixture(mod, settings)

	post, err := svc.Create(context.Background(), "u1", "hello", "clean body", "")
	require.NoError(t, err)
	assert.Equal(t, model.PostStatusApproved, post.Status)
	assert.Empty(t, mod.pending)
}

func TestPostList_DefaultStatusIncludesClosed(t *testing.T) {
	posts := mock.NewPostRepository()
	svc := &postService{posts: posts}
	_, _, err := svc.List(context.Background(), 1, 10, "", "", "", nil, nil, "", "", "", false, "")
	require.NoError(t, err)
	require.Equal(t, []string{model.PostStatusApproved, model.PostStatusClosed}, posts.LastListFilter.StatusIn)
	require.Empty(t, posts.LastListFilter.Status)
}

func TestPostList_ExplicitStatusIgnoredForRegularViewer(t *testing.T) {
	posts := mock.NewPostRepository()
	svc := &postService{posts: posts}
	ctx := WithViewerRole(context.Background(), "user")
	_, _, err := svc.List(ctx, 1, 10, "", "", model.PostStatusPending, nil, nil, "", "", "", false, "")
	require.NoError(t, err)
	require.Empty(t, posts.LastListFilter.Status)
	require.Equal(t, []string{model.PostStatusApproved, model.PostStatusClosed}, posts.LastListFilter.StatusIn)
}

func TestPostList_ExplicitStatusPreservedForModerator(t *testing.T) {
	posts := mock.NewPostRepository()
	svc := &postService{posts: posts}
	ctx := WithViewerRole(context.Background(), "moderator")
	_, _, err := svc.List(ctx, 1, 10, "", "", model.PostStatusPending, nil, nil, "", "", "", false, "")
	require.NoError(t, err)
	require.Equal(t, model.PostStatusPending, posts.LastListFilter.Status)
	require.Empty(t, posts.LastListFilter.StatusIn)
}

func TestModerationApplyTargetStatus_PostAndContent(t *testing.T) {
	posts := mock.NewPostRepository()
	contents := mock.NewContentRepository()
	_ = posts.Create(context.Background(), &model.Post{ID: "p1", ContentID: "croot", Status: model.PostStatusPending})
	_ = contents.Create(context.Background(), &model.Content{ID: "c1", Status: model.PostStatusPending})

	svc := &moderationService{postRepo: posts, contentRepo: contents}
	require.NoError(t, svc.applyTargetStatus(context.Background(), "post", "p1", model.PostStatusApproved))
	require.NoError(t, svc.applyTargetStatus(context.Background(), "content", "c1", model.PostStatusRejected))

	p, err := posts.FindByID(context.Background(), "p1")
	require.NoError(t, err)
	assert.Equal(t, model.PostStatusApproved, p.Status)

	c, err := contents.FindByID(context.Background(), "c1")
	require.NoError(t, err)
	assert.Equal(t, model.PostStatusRejected, c.Status)
}
