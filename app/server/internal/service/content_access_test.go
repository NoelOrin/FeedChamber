package service

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"orinforum/internal/config"
	"orinforum/internal/db"
	"orinforum/internal/mock"
	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"orinforum/internal/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newVisibilityContentFixture 造一个「TL3 版面 + 受限帖 + 一条评论」的库，
// 当前访问者是 TL1 用户（作者另有其人）。
func newVisibilityContentFixture(t *testing.T) (ContentService, context.Context) {
	t.Helper()
	store, err := db.New(config.DBConfig{Type: "sqlite", DSN: filepath.Join(t.TempDir(), "access.db")})
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	require.NoError(t, store.AutoMigrate(
		&model.User{}, &model.Post{}, &model.Content{}, &model.Space{},
	))

	space := &model.Space{
		ID: "sp-level3", Name: "vip", Slug: "vip",
		Visibility: model.SpaceVisibilityLevel, MinLevel: 3, IsActive: true,
	}
	require.NoError(t, store.SQL().DB().Create(space).Error)
	require.NoError(t, store.SQL().DB().Create(&model.User{
		ID: "u-author", Username: "author", Email: "a@x.com", TrustLevel: 2,
	}).Error)
	require.NoError(t, store.SQL().DB().Create(&model.User{
		ID: "u-low", Username: "low", Email: "l@x.com", TrustLevel: 1,
	}).Error)

	require.NoError(t, store.SQL().DB().Create(&model.Post{
		ID: "p-restricted", Title: "restricted", Slug: "restricted",
		ContentID: "root-r", AuthorID: "u-author",
		Status: model.PostStatusApproved, SpaceID: "sp-level3",
	}).Error)
	root := &model.Content{
		ID: "root-r", AuthorID: "u-author", RootID: "root-r",
		Status: model.PostStatusApproved, Body: "root",
	}
	comment := &model.Content{
		ID: "c-top", AuthorID: "u-author", RootID: "root-r", ParentID: "",
		Status: model.PostStatusApproved, Body: "comment",
	}
	require.NoError(t, store.SQL().DB().Create(root).Error)
	require.NoError(t, store.SQL().DB().Create(comment).Error)

	postRepo := repository.NewPostRepository(store)
	contentRepo := repository.NewContentRepository(store)
	userRepo := repository.NewUserRepository(store)
	spaceRepo := repository.NewSpaceRepository(store)

	postSvc := NewPostService(PostServiceDeps{
		Posts:     postRepo,
		Users:     userRepo,
		SpaceRepo: spaceRepo,
		Store:     store,
	})
	svc := NewContentService(ContentServiceDeps{
		Contents: contentRepo,
		Posts:    postRepo,
		Users:    userRepo,
		PostView: postSvc,
		Tx:       db.NewTransactionManager(store),
	})

	viewerCtx := context.WithValue(context.Background(), viewerIDCtxKey{}, "u-low")
	return svc, viewerCtx
}

// 评论是从属内容：它不能绕过帖子本身的等级门槛。帖子详情返回 403 时，
// 评论列表与回复列表必须同样拒绝，否则受限内容会从评论出口泄露。
func TestContentListByPost_RestrictedPostIsForbidden(t *testing.T) {
	svc, viewerCtx := newVisibilityContentFixture(t)

	_, _, err := svc.ListByPost(viewerCtx, "p-restricted", 1, 20, nil, nil)
	require.Error(t, err)

	var appErr *pkg.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, pkg.FORBIDDEN, appErr.Code)
	assert.Contains(t, appErr.Message, "需要等级 3")
}

func TestContentListReplies_RestrictedPostIsForbidden(t *testing.T) {
	svc, viewerCtx := newVisibilityContentFixture(t)

	_, _, err := svc.ListReplies(viewerCtx, "c-top", 1, 20, nil, nil)
	require.Error(t, err)

	var appErr *pkg.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, pkg.FORBIDDEN, appErr.Code)
	assert.Contains(t, appErr.Message, "需要等级 3")
}

func TestContentListByPost_AuthorReadsOwnRestrictedPost(t *testing.T) {
	svc, _ := newVisibilityContentFixture(t)
	authorCtx := context.WithValue(context.Background(), viewerIDCtxKey{}, "u-author")

	items, total, err := svc.ListByPost(authorCtx, "p-restricted", 1, 20, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	require.Len(t, items, 1)
	assert.Equal(t, "c-top", items[0].ID)
}

// recordingGate 记录被求值的能力域，并按域返回预设动作。
type recordingGate struct {
	actions map[string]GateDecision
	domains []string
}

func (g *recordingGate) Evaluate(_ context.Context, domain string, _ GateInput) (GateDecision, error) {
	g.domains = append(g.domains, domain)
	if d, ok := g.actions[domain]; ok {
		return d, nil
	}
	return GateDecision{Action: model.GateActionAllow}, nil
}
func (g *recordingGate) EnforceRateLimit(context.Context, GateDecision, GateInput) error { return nil }
func (g *recordingGate) ListRules(context.Context) ([]model.GateRule, error)             { return nil, nil }
func (g *recordingGate) SaveRule(context.Context, *model.GateRule, string) error         { return nil }
func (g *recordingGate) DeleteRule(context.Context, string) error                        { return nil }
func (g *recordingGate) ListDomainDefaults(context.Context) ([]model.GateDomainDefault, error) {
	return nil, nil
}
func (g *recordingGate) SaveDomainDefault(context.Context, string, string) error { return nil }
func (g *recordingGate) Simulate(context.Context, GateSimulateInput) (GateDecision, error) {
	return GateDecision{Action: model.GateActionAllow}, nil
}
func (g *recordingGate) StartSyncSubscriber(context.Context) error { return nil }

func newGateContentFixture(t *testing.T, gate GateService) ContentService {
	t.Helper()
	contents := mock.NewContentRepository()
	posts := mock.NewPostRepository()
	users := mock.NewUserRepository()

	require.NoError(t, users.Create(context.Background(), &model.User{ID: "u1", Username: "alice", Role: "user"}))
	require.NoError(t, posts.Create(context.Background(), &model.Post{
		ID: "p1", ContentID: "root-1", Title: "t", AuthorID: "u1",
		Status: model.PostStatusApproved,
	}))

	return NewContentService(ContentServiceDeps{
		Contents: contents,
		Posts:    posts,
		Users:    users,
		Gate:     gate,
		Tx:       noopTxManager{},
	})
}

func TestContentCreate_DeniesExternalLinkByGate(t *testing.T) {
	gate := &recordingGate{actions: map[string]GateDecision{
		model.GateDomainExternalLink: {Action: model.GateActionDeny, Note: "等级不足"},
	}}
	svc := newGateContentFixture(t, gate)

	_, err := svc.Create(context.Background(), "p1", "u1", "看这里 https://example.com", "")
	require.Error(t, err)

	var appErr *pkg.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, pkg.FORBIDDEN, appErr.Code)
	assert.Contains(t, gate.domains, model.GateDomainExternalLink)
}

func TestContentCreate_PlainTextSkipsExternalLinkDomain(t *testing.T) {
	gate := &recordingGate{actions: map[string]GateDecision{
		model.GateDomainExternalLink: {Action: model.GateActionDeny},
	}}
	svc := newGateContentFixture(t, gate)

	_, err := svc.Create(context.Background(), "p1", "u1", "纯文本评论", "")
	require.NoError(t, err)
	assert.NotContains(t, gate.domains, model.GateDomainExternalLink)
}

func TestContentCreate_ExternalLinkRequireReviewMarksPending(t *testing.T) {
	gate := &recordingGate{actions: map[string]GateDecision{
		model.GateDomainExternalLink: {Action: model.GateActionRequireReview},
	}}
	svc := newGateContentFixture(t, gate)

	c, err := svc.Create(context.Background(), "p1", "u1", "看这里 https://example.com", "")
	require.NoError(t, err)
	assert.Equal(t, model.PostStatusPending, c.Status)
}

// 写作侧同样不能绕过：看不见的帖子不应接受评论（否则低等级用户可盲写受限主题）。
func TestContentCreate_RejectsCommentOnRestrictedPost(t *testing.T) {
	svc, viewerCtx := newVisibilityContentFixture(t)

	_, err := svc.Create(viewerCtx, "p-restricted", "u-low", "盲写一条", "")
	require.Error(t, err)

	var appErr *pkg.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, pkg.FORBIDDEN, appErr.Code)
	assert.Contains(t, appErr.Message, "需要等级 3")
}

func TestContentCreate_AuthorCanCommentOnOwnRestrictedPost(t *testing.T) {
	svc, _ := newVisibilityContentFixture(t)
	authorCtx := context.WithValue(context.Background(), viewerIDCtxKey{}, "u-author")

	c, err := svc.Create(authorCtx, "p-restricted", "u-author", "作者自己的回复", "")
	require.NoError(t, err)
	assert.Equal(t, "u-author", c.AuthorID)
}
