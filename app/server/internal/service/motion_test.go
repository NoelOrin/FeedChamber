package service

import (
	"context"
	"testing"
	"time"

	"feedchamber/internal/mock"
	"feedchamber/internal/model"
	"feedchamber/internal/pkg"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newMotionFixture 组装一个最小可用的 motion 服务，并把默认法定人数降到 2，
// 免得每个用例都要造 20 个投票人。
func newMotionFixture(t *testing.T) (*motionService, *mock.MotionRepository, *mock.UserRepository, *mock.SpaceRepository, *mock.PostRepository) {
	t.Helper()
	repo := mock.NewMotionRepository()
	users := mock.NewUserRepository()
	spaces := mock.NewSpaceRepository()
	posts := mock.NewPostRepository()

	svc := NewMotionService(repo, posts, mock.NewContentRepository(), users, spaces, nil, nil, nil, nil)
	impl := svc.(*motionService)
	impl.cfg.Quorum = 2
	impl.cfg.UserQuorumFactor = 1 // 单测里 user 类不额外放大

	require.NoError(t, spaces.Create(context.Background(), &model.Space{
		ID: "dz-1", Slug: DarkZoneSlug, Name: "暗区", System: true, Recommendable: false,
	}))
	return impl, repo, users, spaces, posts
}

func TestMotionCreate_RejectsInvalidTargetAndType(t *testing.T) {
	svc, _, _, _, _ := newMotionFixture(t)
	ctx := context.Background()

	_, err := svc.Create(ctx, "u1", "spaceship", "t1", model.MotionTypeFold, "r", "")
	var appErr *pkg.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, pkg.INVALID_PARAMS, appErr.Code)

	_, err = svc.Create(ctx, "u1", model.MotionTargetPost, "t1", "nuke", "r", "")
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, pkg.INVALID_PARAMS, appErr.Code)
}

func TestMotionCreate_RespectsMotionBan(t *testing.T) {
	svc, _, users, _, _ := newMotionFixture(t)
	ctx := context.Background()
	until := timeNowAdd(24)
	require.NoError(t, users.Create(ctx, &model.User{ID: "u1", Username: "u1", MotionBannedUntil: &until}))

	_, err := svc.Create(ctx, "u1", model.MotionTargetPost, "p1", model.MotionTypeFold, "r", "")
	var appErr *pkg.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, pkg.FORBIDDEN, appErr.Code)
}

func TestMotionCreate_DedupesPendingSameTarget(t *testing.T) {
	svc, _, _, _, _ := newMotionFixture(t)
	ctx := context.Background()

	_, err := svc.Create(ctx, "u1", model.MotionTargetPost, "p1", model.MotionTypeFold, "r", "")
	require.NoError(t, err)

	_, err = svc.Create(ctx, "u2", model.MotionTargetPost, "p1", model.MotionTypeFold, "r", "")
	var appErr *pkg.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, pkg.INVALID_PARAMS, appErr.Code)
}

// 票权按等级：TL0–1 → 1 票 / TL2–3 → 2 票 / TL4+ → 3 票。
func TestMotionVote_LevelWeighted(t *testing.T) {
	svc, repo, users, _, _ := newMotionFixture(t)
	ctx := context.Background()
	svc.cfg.Quorum = 100 // 抬高门槛，避免中途自动结算，逐个校验票权

	require.NoError(t, users.Create(ctx, &model.User{ID: "tl0", Username: "tl0", TrustLevel: 0}))
	require.NoError(t, users.Create(ctx, &model.User{ID: "tl2", Username: "tl2", TrustLevel: 2}))
	require.NoError(t, users.Create(ctx, &model.User{ID: "tl4", Username: "tl4", TrustLevel: 4}))

	m, err := svc.Create(ctx, "initiator", model.MotionTargetPost, "p1", model.MotionTypeWatch, "r", "")
	require.NoError(t, err)

	_, err = svc.Vote(ctx, m.ID, "tl0", model.MotionVoteApprove)
	require.NoError(t, err)
	v, err := repo.FindVote(ctx, m.ID, "tl0")
	require.NoError(t, err)
	assert.Equal(t, 1.0, v.Weight)

	_, err = svc.Vote(ctx, m.ID, "tl2", model.MotionVoteApprove)
	require.NoError(t, err)
	v, err = repo.FindVote(ctx, m.ID, "tl2")
	require.NoError(t, err)
	assert.Equal(t, 2.0, v.Weight)

	_, err = svc.Vote(ctx, m.ID, "tl4", model.MotionVoteApprove)
	require.NoError(t, err)
	v, err = repo.FindVote(ctx, m.ID, "tl4")
	require.NoError(t, err)
	assert.Equal(t, 3.0, v.Weight)
}

func TestMotionVote_RejectsDoubleVoteAndClosedMotion(t *testing.T) {
	svc, _, users, _, _ := newMotionFixture(t)
	ctx := context.Background()
	require.NoError(t, users.Create(ctx, &model.User{ID: "v1", Username: "v1"}))

	m, err := svc.Create(ctx, "initiator", model.MotionTargetPost, "p1", model.MotionTypeWatch, "r", "")
	require.NoError(t, err)
	_, err = svc.Vote(ctx, m.ID, "v1", model.MotionVoteApprove)
	require.NoError(t, err)

	_, err = svc.Vote(ctx, m.ID, "v1", model.MotionVoteReject)
	var appErr *pkg.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, pkg.INVALID_PARAMS, appErr.Code)

	// 结算后不可再投
	_, err = svc.Resolve(ctx, m.ID, "admin")
	require.NoError(t, err)
	_, err = svc.Vote(ctx, m.ID, "v2", model.MotionVoteApprove)
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, pkg.INVALID_PARAMS, appErr.Code)
}

// 达法定人数且赞成 > 反对 → passed。
func TestMotionResolve_PassesOnQuorumMajority(t *testing.T) {
	svc, _, users, _, _ := newMotionFixture(t)
	ctx := context.Background()
	for _, id := range []string{"a", "b", "c"} {
		require.NoError(t, users.Create(ctx, &model.User{ID: id, Username: id}))
	}

	m, err := svc.Create(ctx, "initiator", model.MotionTargetPost, "p1", model.MotionTypeWatch, "r", "")
	require.NoError(t, err)
	_, _ = svc.Vote(ctx, m.ID, "a", model.MotionVoteApprove)
	_, _ = svc.Vote(ctx, m.ID, "b", model.MotionVoteApprove)

	got, err := svc.GetByID(ctx, m.ID)
	require.NoError(t, err)
	assert.Equal(t, model.MotionStatusPassed, got.Status, "2 票赞成达法定人数即自动结算")
}

// 未达法定人数 → rejected。
func TestMotionResolve_RejectsBelowQuorum(t *testing.T) {
	svc, _, users, _, _ := newMotionFixture(t)
	ctx := context.Background()
	require.NoError(t, users.Create(ctx, &model.User{ID: "a", Username: "a"}))

	m, err := svc.Create(ctx, "initiator", model.MotionTargetPost, "p1", model.MotionTypeWatch, "r", "")
	require.NoError(t, err)
	_, err = svc.Vote(ctx, m.ID, "a", model.MotionVoteApprove)
	require.NoError(t, err)

	got, err := svc.Resolve(ctx, m.ID, "admin")
	require.NoError(t, err)
	assert.Equal(t, model.MotionStatusRejected, got.Status)
}

// exile 帖子：路由到 dark-zone 并置 exiled=true。
func TestMotionExile_RoutesPostToDarkZone(t *testing.T) {
	svc, _, users, _, posts := newMotionFixture(t)
	ctx := context.Background()
	for _, id := range []string{"a", "b"} {
		require.NoError(t, users.Create(ctx, &model.User{ID: id, Username: id}))
	}
	require.NoError(t, posts.Create(ctx, &model.Post{
		ID: "p1", Title: "t", ContentID: "c1", AuthorID: "author", SpaceID: "tech", Status: "approved",
	}))

	m, err := svc.Create(ctx, "initiator", model.MotionTargetPost, "p1", model.MotionTypeExile, "toxic", "")
	require.NoError(t, err)
	_, err = svc.Vote(ctx, m.ID, "a", model.MotionVoteApprove)
	require.NoError(t, err)
	_, err = svc.Vote(ctx, m.ID, "b", model.MotionVoteApprove)
	require.NoError(t, err)

	got, err := svc.GetByID(ctx, m.ID)
	require.NoError(t, err)
	assert.Equal(t, model.MotionStatusPassed, got.Status)

	p, err := posts.FindByID(ctx, "p1")
	require.NoError(t, err)
	assert.Equal(t, "dz-1", p.SpaceID, "帖子被路由到暗区")
	assert.True(t, p.Exiled, "帖子置 exiled 标记，主推荐流据此过滤")
}

// 反猎巫：标记滥诉累计达阈值后限制发起。
func TestMotionAbuse_ThresholdBansInitiator(t *testing.T) {
	svc, repo, users, _, _ := newMotionFixture(t)
	ctx := context.Background()
	require.NoError(t, users.Create(ctx, &model.User{ID: "initiator", Username: "initiator"}))

	svc.cfg.AbuseThreshold = 1

	m, err := svc.Create(ctx, "initiator", model.MotionTargetPost, "p1", model.MotionTypeFold, "r", "")
	require.NoError(t, err)
	_ = repo.UpdateStatus(ctx, m.ID, model.MotionStatusRejected, nil)

	abuse := true
	_, err = svc.UpdateStatus(ctx, m.ID, "moderator", "", &abuse, false)
	require.NoError(t, err)

	u, err := users.FindByID(ctx, "initiator")
	require.NoError(t, err)
	assert.Equal(t, 1, u.AbuseCount)
	require.NotNil(t, u.MotionBannedUntil, "达阈值后置限权截止")

	_, err = svc.Create(ctx, "initiator", model.MotionTargetPost, "p2", model.MotionTypeFold, "r", "")
	var appErr *pkg.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, pkg.FORBIDDEN, appErr.Code)
}

// 翻案解除软隔离。
func TestMotionOverturn_LiftsExile(t *testing.T) {
	svc, _, users, _, posts := newMotionFixture(t)
	ctx := context.Background()
	for _, id := range []string{"a", "b"} {
		require.NoError(t, users.Create(ctx, &model.User{ID: id, Username: id}))
	}
	require.NoError(t, posts.Create(ctx, &model.Post{
		ID: "p1", Title: "t", ContentID: "c1", AuthorID: "author", SpaceID: "tech", Status: "approved",
	}))

	m, err := svc.Create(ctx, "initiator", model.MotionTargetPost, "p1", model.MotionTypeExile, "toxic", "")
	require.NoError(t, err)
	_, _ = svc.Vote(ctx, m.ID, "a", model.MotionVoteApprove)
	_, _ = svc.Vote(ctx, m.ID, "b", model.MotionVoteApprove)

	p, _ := posts.FindByID(ctx, "p1")
	require.True(t, p.Exiled)

	_, err = svc.UpdateStatus(ctx, m.ID, "moderator", model.MotionStatusOverturned, nil, false)
	require.NoError(t, err)

	p, _ = posts.FindByID(ctx, "p1")
	assert.False(t, p.Exiled, "翻案后解除 exiled 标记")
}

func TestMotionDelete_OnlyInitiatorAndPending(t *testing.T) {
	svc, _, _, _, _ := newMotionFixture(t)
	ctx := context.Background()

	m, err := svc.Create(ctx, "initiator", model.MotionTargetPost, "p1", model.MotionTypeFold, "r", "")
	require.NoError(t, err)

	var appErr *pkg.AppError
	err = svc.Delete(ctx, m.ID, "someone-else")
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, pkg.FORBIDDEN, appErr.Code)

	require.NoError(t, svc.Delete(ctx, m.ID, "initiator"))
	_, err = svc.GetByID(ctx, m.ID)
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, pkg.NOT_FOUND, appErr.Code)
}

// timeNowAdd 返回 n 小时后的时间，供限权/隔离用例复用。
func timeNowAdd(hours int) (t time.Time) {
	return time.Now().Add(time.Duration(hours) * time.Hour)
}

// 开放期满后由后台 sweep 结算：未达法定人数按当前票数判 rejected。
func TestMotionResolveDue_SettlesExpiredPending(t *testing.T) {
	svc, repo, users, _, _ := newMotionFixture(t)
	ctx := context.Background()
	require.NoError(t, users.Create(ctx, &model.User{ID: "a", Username: "a"}))

	m, err := svc.Create(ctx, "initiator", model.MotionTargetPost, "p1", model.MotionTypeWatch, "r", "")
	require.NoError(t, err)
	_, err = svc.Vote(ctx, m.ID, "a", model.MotionVoteApprove)
	require.NoError(t, err)

	// 未过期：sweep 不应动它
	svc.resolveDue(ctx)
	got, _ := svc.GetByID(ctx, m.ID)
	require.Equal(t, model.MotionStatusPending, got.Status)

	// 把创建时间推到开放期之前，sweep 应结算
	stored, _ := repo.FindByID(ctx, m.ID)
	stored.CreatedAt = time.Now().Add(-25 * time.Hour)
	svc.resolveDue(ctx)

	got, _ = svc.GetByID(ctx, m.ID)
	assert.Equal(t, model.MotionStatusRejected, got.Status, "1 票未达法定人数(2)，期满应判未通过")
	assert.NotNil(t, got.ResolvedAt)
}

// 期满且票数已过门槛时应判 passed 并执行结果。
func TestMotionResolveDue_PassesWhenQuorumMet(t *testing.T) {
	svc, repo, users, _, posts := newMotionFixture(t)
	ctx := context.Background()
	for _, id := range []string{"a", "b"} {
		require.NoError(t, users.Create(ctx, &model.User{ID: id, Username: id}))
	}
	require.NoError(t, posts.Create(ctx, &model.Post{
		ID: "p1", Title: "t", ContentID: "c1", AuthorID: "author", SpaceID: "tech", Status: "approved",
	}))

	m, err := svc.Create(ctx, "initiator", model.MotionTargetPost, "p1", model.MotionTypeExile, "tox", "")
	require.NoError(t, err)
	stored, _ := repo.FindByID(ctx, m.ID)
	stored.CreatedAt = time.Now().Add(-25 * time.Hour)

	// 票数刚好达法定人数（2）
	_, _ = svc.Vote(ctx, m.ID, "a", model.MotionVoteApprove)
	_, _ = svc.Vote(ctx, m.ID, "b", model.MotionVoteApprove)

	got, _ := svc.GetByID(ctx, m.ID)
	require.Equal(t, model.MotionStatusPassed, got.Status)

	p, _ := posts.FindByID(ctx, "p1")
	assert.Equal(t, "dz-1", p.SpaceID, "passed 的 exile 必须真的把帖子移入暗区")
	assert.True(t, p.Exiled)
}
