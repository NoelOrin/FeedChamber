package repository

import (
	"testing"

	"orinforum/internal/model"

	"github.com/stretchr/testify/require"
)

func TestBuildPostListWhere_StatusIn(t *testing.T) {
	q, args := buildPostListWhere(PostListFilter{StatusIn: []string{model.PostStatusApproved, model.PostStatusClosed}})
	require.Contains(t, q, "p.status IN (?,?)")
	require.Equal(t, []interface{}{model.PostStatusApproved, model.PostStatusClosed}, args)
}

func TestBuildPostListWhere_ExplicitStatus(t *testing.T) {
	q, args := buildPostListWhere(PostListFilter{Status: model.PostStatusPending})
	require.Contains(t, q, "p.status = ?")
	require.Equal(t, []interface{}{model.PostStatusPending}, args)
}

func TestBuildPostListWhere_ListView(t *testing.T) {
	q, args := buildPostListWhere(PostListFilter{ListView: "unread", ViewerID: "u1"})
	require.Contains(t, q, "topic_read_states")
	require.Contains(t, q, "p.last_active_at > trs.last_read_at")
	require.Equal(t, []interface{}{"u1"}, args)
}

// 哨兵 "__all__" 表示「不过滤状态」，绝不能作为字面状态拼进 WHERE
// （曾经 WHERE p.status = '__all__' 让后台列表恒 0 行）。
func TestBuildPostListWhere_AllSentinelIsNotALiteralStatus(t *testing.T) {
	q, args := buildPostListWhere(PostListFilter{Status: StatusFilterAll})
	require.NotContains(t, q, "p.status")
	require.Empty(t, args)
}

// 主推荐流需排除暗区软隔离内容；按版面精确浏览（进入暗区）时不排除。
func TestBuildPostListWhere_ExcludeExiled(t *testing.T) {
	q, args := buildPostListWhere(PostListFilter{ExcludeExiled: true})
	require.Contains(t, q, "p.exiled = ?")
	require.Equal(t, []interface{}{false}, args)

	q2, _ := buildPostListWhere(PostListFilter{})
	require.NotContains(t, q2, "p.exiled")

	// 版面精确浏览保留 exiled（暗区自身可见）
	q3, _ := buildPostListWhere(PostListFilter{SpaceID: "dz-1"})
	require.NotContains(t, q3, "p.exiled")
}
