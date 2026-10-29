package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"feedchamber/internal/mock"
	"feedchamber/internal/model"
	"feedchamber/internal/pkg/eventbus"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseModerationMode(t *testing.T) {
	assert.Equal(t, model.ModerationModeOff, model.ParseModerationMode(" OFF "))
	assert.Equal(t, model.ModerationModeManual, model.ParseModerationMode("manual"))
	assert.Equal(t, model.ModerationModeExternal, model.ParseModerationMode("external"))
	// 未配置 / 非法值一律回落默认的「发布后异步审核」，不会静默变成免审或全员待审。
	assert.Equal(t, model.DefaultModerationMode, model.ParseModerationMode(""))
	assert.Equal(t, model.DefaultModerationMode, model.ParseModerationMode("bogus"))

	assert.True(t, model.ValidModerationMode("async"))
	assert.False(t, model.ValidModerationMode("bogus"))
}

func TestModerationMode_ReadsSettingWithFallback(t *testing.T) {
	ctx := context.Background()
	newSvc := func(settings *mock.SettingService) ModerationService {
		return NewModerationService(&memModRepo{configs: map[string]string{}}, nil, nil, settings, nil)
	}

	assert.Equal(t, model.ModerationModeManual,
		newSvc(mock.NewSettingService().WithSetting(model.SettingModerationMode, "manual")).Mode(ctx))
	assert.Equal(t, model.ModerationModeOff,
		newSvc(mock.NewSettingService().WithSetting(model.SettingModerationMode, "off")).Mode(ctx))
	// 未配置 → 默认 async；非法值 → 同样回落 async。
	assert.Equal(t, model.DefaultModerationMode, newSvc(mock.NewSettingService()).Mode(ctx))
	assert.Equal(t, model.DefaultModerationMode,
		newSvc(mock.NewSettingService().WithSetting(model.SettingModerationMode, "nonsense")).Mode(ctx))
}

// 模式 2 的异步侧：发帖事件到达后跑规则，命中即转待审并建单（不直接删帖/拒稿）。
func TestHandlePostCreated_FlagsApprovedPostToPending(t *testing.T) {
	ctx := context.Background()
	posts := mock.NewPostRepository()
	contents := mock.NewContentRepository()
	require.NoError(t, posts.Create(ctx, &model.Post{ID: "p1", ContentID: "c1", Status: model.PostStatusApproved}))
	require.NoError(t, contents.Create(ctx, &model.Content{ID: "c1", Body: "hello spamword", AuthorID: "u1"}))

	repo := &memModRepo{
		configs: map[string]string{
			model.ModerationConfigEnabled:     "true",
			model.ModerationConfigPostEnabled: "true",
		},
		rules: []model.ModerationRule{{
			Name:    "badword",
			Scene:   model.ModerationScenePost,
			Type:    "keyword",
			Pattern: "spamword",
			Action:  model.ModerationActionFlag,
			Enabled: true,
		}},
	}
	svc := NewModerationService(repo, posts, contents,
		mock.NewSettingService().WithSetting(model.SettingModerationMode, model.ModerationModeAsync), nil)
	impl := svc.(*moderationService)

	impl.handlePostCreated(eventbus.Event{
		Ctx:     ctx,
		Payload: map[string]any{"post_id": "p1", "moderation_async": "true", "author_role": "user"},
	})

	got, err := posts.FindByID(ctx, "p1")
	require.NoError(t, err)
	assert.Equal(t, model.PostStatusPending, got.Status)
	require.Len(t, repo.created, 1)
	assert.Equal(t, "async", repo.created[0].Strategy)
	assert.Equal(t, model.ModerationActionFlag, repo.created[0].Action)
}

// 没有显式要求异步复查的帖子（manual / off / 明文通过）不进入异步管线。
func TestHandlePostCreated_IgnoresUnrequestedEvents(t *testing.T) {
	ctx := context.Background()
	posts := mock.NewPostRepository()
	contents := mock.NewContentRepository()
	require.NoError(t, posts.Create(ctx, &model.Post{ID: "p1", ContentID: "c1", Status: model.PostStatusApproved}))
	require.NoError(t, contents.Create(ctx, &model.Content{ID: "c1", Body: "hello spamword"}))

	repo := &memModRepo{
		configs: map[string]string{
			model.ModerationConfigEnabled:     "true",
			model.ModerationConfigPostEnabled: "true",
		},
		rules: []model.ModerationRule{{
			Name: "badword", Scene: model.ModerationScenePost, Type: "keyword",
			Pattern: "spamword", Action: model.ModerationActionFlag, Enabled: true,
		}},
	}
	svc := NewModerationService(repo, posts, contents,
		mock.NewSettingService().WithSetting(model.SettingModerationMode, model.ModerationModeAsync), nil)
	impl := svc.(*moderationService)

	impl.handlePostCreated(eventbus.Event{Ctx: ctx, Payload: map[string]any{"post_id": "p1"}})

	got, _ := posts.FindByID(ctx, "p1")
	assert.Equal(t, model.PostStatusApproved, got.Status)
	assert.Empty(t, repo.created)
}

// 已被人工/TL0 置为待审的帖子不重复建单（否则队列里会出现两条同样的记录）。
func TestHandlePostCreated_SkipsAlreadyPendingPost(t *testing.T) {
	ctx := context.Background()
	posts := mock.NewPostRepository()
	contents := mock.NewContentRepository()
	require.NoError(t, posts.Create(ctx, &model.Post{ID: "p1", ContentID: "c1", Status: model.PostStatusPending}))
	require.NoError(t, contents.Create(ctx, &model.Content{ID: "c1", Body: "hello spamword"}))

	repo := &memModRepo{
		configs: map[string]string{
			model.ModerationConfigEnabled:     "true",
			model.ModerationConfigPostEnabled: "true",
		},
		rules: []model.ModerationRule{{
			Name: "badword", Scene: model.ModerationScenePost, Type: "keyword",
			Pattern: "spamword", Action: model.ModerationActionFlag, Enabled: true,
		}},
	}
	svc := NewModerationService(repo, posts, contents,
		mock.NewSettingService().WithSetting(model.SettingModerationMode, model.ModerationModeAsync), nil)
	impl := svc.(*moderationService)

	impl.handlePostCreated(eventbus.Event{
		Ctx:     ctx,
		Payload: map[string]any{"post_id": "p1", "moderation_async": "true"},
	})

	assert.Empty(t, repo.created)
}

// 模式 3：外部服务返回结论时按结论放行/待审。
func TestCheckExternal_ReturnsVerdict(t *testing.T) {
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "Bearer secret", r.Header.Get("Authorization"))
		require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
		_ = json.NewEncoder(w).Encode(map[string]string{"action": "flag", "reason": "politics"})
	}))
	defer srv.Close()

	repo := &memModRepo{configs: map[string]string{
		model.ModerationConfigAIProvider: "http",
		model.ModerationConfigAIEndpoint: srv.URL,
		model.ModerationConfigAIAPIKey:   "secret",
	}}
	svc := NewModerationService(repo, nil, nil, nil, nil)

	action, err := svc.CheckExternal(context.Background(), "post", "p1", "title", "body text", model.ModerationScenePost)
	require.NoError(t, err)
	assert.Equal(t, model.ModerationActionFlag, action)
	assert.Equal(t, "post", gotBody["target_type"])
	assert.Equal(t, "p1", gotBody["target_id"])
	assert.Equal(t, "body text", gotBody["body"])
}

func TestCheckExternal_AllowPasses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"action":"allow"}`))
	}))
	defer srv.Close()

	repo := &memModRepo{configs: map[string]string{
		model.ModerationConfigAIProvider: "http",
		model.ModerationConfigAIEndpoint: srv.URL,
	}}
	svc := NewModerationService(repo, nil, nil, nil, nil)
	action, err := svc.CheckExternal(context.Background(), "post", "p1", "t", "b", model.ModerationScenePost)
	require.NoError(t, err)
	assert.Equal(t, model.ModerationActionAllow, action)
}

// 未配置 / 非 2xx / 未知结论都必须返回「不可用」，让发帖回退异步而不是被卡住。
func TestCheckExternal_UnavailableCases(t *testing.T) {
	ctx := context.Background()

	// 未配置 endpoint
	svc := NewModerationService(&memModRepo{configs: map[string]string{}}, nil, nil, nil, nil)
	_, err := svc.CheckExternal(ctx, "post", "p1", "t", "b", model.ModerationScenePost)
	require.ErrorIs(t, err, errExternalModerationUnavailable)

	// 5xx
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	svcErr := NewModerationService(&memModRepo{configs: map[string]string{
		model.ModerationConfigAIProvider: "http",
		model.ModerationConfigAIEndpoint: srv.URL,
	}}, nil, nil, nil, nil)
	_, err = svcErr.CheckExternal(ctx, "post", "p1", "t", "b", model.ModerationScenePost)
	require.ErrorIs(t, err, errExternalModerationUnavailable)

	// 未知 action
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"action":"whatever"}`))
	}))
	defer srv2.Close()
	svcUnknown := NewModerationService(&memModRepo{configs: map[string]string{
		model.ModerationConfigAIProvider: "http",
		model.ModerationConfigAIEndpoint: srv2.URL,
	}}, nil, nil, nil, nil)
	_, err = svcUnknown.CheckExternal(ctx, "post", "p1", "t", "b", model.ModerationScenePost)
	require.ErrorIs(t, err, errExternalModerationUnavailable)
}
