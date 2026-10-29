package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"orinforum/internal/mock"
	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"orinforum/internal/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newPostCreateFixture 组装一个最小的 postService：TL0 用户 + 可注入的设置/审核替身。
func newPostCreateFixture(t *testing.T, settings *mock.SettingService, mod ModerationService) (*postService, *mock.PostRepository, *mock.UserRepository) {
	t.Helper()
	posts := mock.NewPostRepository()
	contents := mock.NewContentRepository()
	users := mock.NewUserRepository()
	require.NoError(t, users.Create(context.Background(), &model.User{
		ID: "u1", Username: "alice", Role: "user", TrustLevel: 0,
	}))
	if settings == nil {
		settings = mock.NewSettingService()
	}
	return &postService{
		posts:         posts,
		contents:      contents,
		users:         users,
		settingSvc:    settings,
		trustSvc:      NewTrustService(users, mock.NewTrustRepository().WithDefaultConfig(), settings, nil),
		moderationSvc: mod,
	}, posts, users
}

// 契约 5 + 4：默认配置（trust_tl0_require_approval 未配置）下 TL0 用户发帖 MUST 落 approved，
// 且创建响应 data 里 MUST 带 status。
func TestPostCreate_TL0DefaultsToApproved(t *testing.T) {
	ctx := context.Background()
	svc, posts, _ := newPostCreateFixture(t, nil, nil)

	post, err := svc.Create(ctx, "u1", "hello", "clean body", "")
	require.NoError(t, err)
	require.NotNil(t, post)
	assert.Equal(t, model.PostStatusApproved, post.Status)

	raw, err := json.Marshal(post)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"status":"approved"`)

	got, err := posts.FindByID(ctx, post.ID)
	require.NoError(t, err)
	assert.Equal(t, model.PostStatusApproved, got.Status)
}

// 契约 5 场景二（P3 起由门控规则引擎表达）：post_create 规则 require_review
// 时发帖 = pending，且 MUST 留下审核记录。
func TestPostCreate_GateRequireReviewRecordsPending(t *testing.T) {
	ctx := context.Background()
	mod := &stubModerationService{action: model.ModerationActionAllow}
	svc, posts, _ := newPostCreateFixture(t, nil, mod)
	svc.gateSvc = &stubGateService{decision: GateDecision{
		Domain: model.GateDomainPostCreate, Action: model.GateActionRequireReview, RuleID: "gate-review",
	}}

	post, err := svc.Create(ctx, "u1", "hello", "needs review", "")
	require.NoError(t, err)
	require.NotNil(t, post)
	assert.Equal(t, model.PostStatusPending, post.Status)
	require.Len(t, mod.pending, 1, "pending 帖 MUST 有审核记录")
	assert.Equal(t, "post", mod.pending[0].targetType)
	assert.Equal(t, post.ID, mod.pending[0].targetID)

	got, err := posts.FindByID(ctx, post.ID)
	require.NoError(t, err)
	assert.Equal(t, model.PostStatusPending, got.Status)
}

// 留痕回归：审核规则全部放行、但 post_approval=true 时，pending 帖同样 MUST 有审核记录
// （此前的实现只在 moderation 命中时才写，approval 门驱动的 pending 会漏）。
func TestPostCreate_ApprovalGateWithoutRuleHitStillRecords(t *testing.T) {
	ctx := context.Background()
	settings := mock.NewSettingService().WithSetting("post_approval", "true")
	mod := &stubModerationService{action: model.ModerationActionAllow}
	svc, _, _ := newPostCreateFixture(t, settings, mod)

	post, err := svc.Create(ctx, "u1", "hello", "clean body", "")
	require.NoError(t, err)
	assert.Equal(t, model.PostStatusPending, post.Status)
	require.Len(t, mod.pending, 1)
}

// 契约 1：SetStatus 接受四个合法值、拒绝非法值（2xxx）且不改状态、未知 id 返回 404。
func TestPostSetStatus_AppliesValidStatusAndRejectsInvalid(t *testing.T) {
	ctx := context.Background()
	posts := mock.NewPostRepository()
	require.NoError(t, posts.Create(ctx, &model.Post{ID: "p1", AuthorID: "u1", Status: model.PostStatusPending}))
	svc := &postService{posts: posts}

	for _, status := range []string{
		model.PostStatusApproved, model.PostStatusPending, model.PostStatusRejected, model.PostStatusClosed,
	} {
		require.NoError(t, svc.SetStatus(ctx, "p1", status), "合法状态 %q 应被接受", status)
		got, err := posts.FindByID(ctx, "p1")
		require.NoError(t, err)
		assert.Equal(t, status, got.Status)
	}

	// 非法状态值：返回参数错误（2xxx）且状态不变
	before, err := posts.FindByID(ctx, "p1")
	require.NoError(t, err)
	err = svc.SetStatus(ctx, "p1", "whatever")
	require.Error(t, err)
	var appErr *pkg.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, pkg.INVALID_PARAMS, appErr.Code)
	after, err := posts.FindByID(ctx, "p1")
	require.NoError(t, err)
	assert.Equal(t, before.Status, after.Status, "非法状态值 MUST NOT 改变帖子状态")

	// 未知 id
	err = svc.SetStatus(ctx, "missing", model.PostStatusApproved)
	require.Error(t, err)
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, pkg.NOT_FOUND, appErr.Code)
}

// 契约 1 的救回路径：无任何审核记录的存量 pending 帖，SetStatus 直接放行后
// 对匿名列表可见（approve 只接受审核记录 id，救不回这类帖子）。
func TestPostSetStatus_RescuesPostWithoutModerationRecord(t *testing.T) {
	ctx := context.Background()
	posts := mock.NewPostRepository()
	require.NoError(t, posts.Create(ctx, &model.Post{ID: "p1", AuthorID: "u1", Status: model.PostStatusPending}))
	svc := &postService{posts: posts}

	require.NoError(t, svc.SetStatus(ctx, "p1", model.PostStatusApproved))

	// 匿名列表请求只能看到 approved/closed —— 该帖现在应落在可见集合内。
	_, _, err := svc.List(ctx, 1, 10, "", "", "", nil, nil, "", "", "", false, "")
	require.NoError(t, err)
	assert.Equal(t, []string{model.PostStatusApproved, model.PostStatusClosed}, posts.LastListFilter.StatusIn)

	got, err := posts.FindByID(ctx, "p1")
	require.NoError(t, err)
	assert.Equal(t, model.PostStatusApproved, got.Status)
}

// 契约 2 的举报列表同源缺陷：哨兵 "__all__" MUST 返回全部状态而不是恒 0 行。
func TestReportList_AllSentinelReturnsAllStatuses(t *testing.T) {
	ctx := context.Background()
	repo := mock.NewReportRepository()
	svc := NewReportService(repo, nil, nil, nil, nil)
	require.NoError(t, repo.Create(ctx, &model.Report{ID: "r1", Status: "pending", ReporterID: "u1"}))
	require.NoError(t, repo.Create(ctx, &model.Report{ID: "r2", Status: "resolved", ReporterID: "u2"}))

	items, total, err := svc.List(ctx, repository.StatusFilterAll, 1, 10)
	require.NoError(t, err)
	assert.Len(t, items, 2)
	assert.Equal(t, int64(2), total)
}
