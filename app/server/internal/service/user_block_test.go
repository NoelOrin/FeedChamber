package service

import (
	"context"
	"testing"

	"feedchamber/internal/model"
	"feedchamber/internal/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubBlockRepo struct {
	pairs map[string]struct{}
}

func (r *stubBlockRepo) key(a, b string) string { return a + "->" + b }
func (r *stubBlockRepo) Create(_ context.Context, b *model.UserBlock) error {
	if r.pairs == nil {
		r.pairs = map[string]struct{}{}
	}
	r.pairs[r.key(b.BlockerID, b.BlockedID)] = struct{}{}
	return nil
}
func (r *stubBlockRepo) Delete(_ context.Context, blockerID, blockedID string) error {
	delete(r.pairs, r.key(blockerID, blockedID))
	return nil
}
func (r *stubBlockRepo) Exists(_ context.Context, blockerID, blockedID string) (bool, error) {
	_, ok := r.pairs[r.key(blockerID, blockedID)]
	return ok, nil
}
func (r *stubBlockRepo) EitherBlocked(_ context.Context, a, b string) (bool, error) {
	_, ab := r.pairs[r.key(a, b)]
	_, ba := r.pairs[r.key(b, a)]
	return ab || ba, nil
}
func (r *stubBlockRepo) ListBlockedIDs(_ context.Context, blockerID string, offset, limit int) ([]string, error) {
	var out []string
	prefix := blockerID + "->"
	for k := range r.pairs {
		if len(k) > len(prefix) && k[:len(prefix)] == prefix {
			out = append(out, k[len(prefix):])
		}
	}
	return out, nil
}
func (r *stubBlockRepo) CountBlocked(_ context.Context, blockerID string) (int64, error) {
	ids, _ := r.ListBlockedIDs(context.Background(), blockerID, 0, 1000)
	return int64(len(ids)), nil
}

type stubBlockUserRepo struct {
	users map[string]*model.User
}

func (r *stubBlockUserRepo) FindByID(_ context.Context, id string) (*model.User, error) {
	if u, ok := r.users[id]; ok {
		return u, nil
	}
	return nil, repository.ErrNotFound
}
func (r *stubBlockUserRepo) FindByEmail(context.Context, string) (*model.User, error) {
	return nil, repository.ErrNotFound
}
func (r *stubBlockUserRepo) FindByUsername(context.Context, string) (*model.User, error) {
	return nil, repository.ErrNotFound
}
func (r *stubBlockUserRepo) FindByIdentifier(context.Context, string) (*model.User, error) {
	return nil, repository.ErrNotFound
}
func (r *stubBlockUserRepo) List(context.Context, int, int) ([]model.User, error) { return nil, nil }
func (r *stubBlockUserRepo) Search(context.Context, string, string, int, int) ([]model.User, error) {
	return nil, nil
}
func (r *stubBlockUserRepo) CountUsersByPermission(context.Context, string) (int64, error) {
	return 0, nil
}
func (r *stubBlockUserRepo) GetActivityStats(context.Context, string, int) (map[string]int, error) {
	return map[string]int{}, nil
}
func (r *stubBlockUserRepo) CountPosts(context.Context, string) (int64, error) { return 0, nil }
func (r *stubBlockUserRepo) CountCommentsByUser(context.Context, string) (int64, error) {
	return 0, nil
}
func (r *stubBlockUserRepo) CountLikesByUser(context.Context, string) (int64, error)   { return 0, nil }
func (r *stubBlockUserRepo) CountLikesReceived(context.Context, string) (int64, error) { return 0, nil }
func (r *stubBlockUserRepo) IncrPoints(context.Context, string, int) error             { return nil }
func (r *stubBlockUserRepo) ListByPoints(context.Context, int, int) ([]model.User, error) {
	return nil, nil
}
func (r *stubBlockUserRepo) ListByCheckIns(context.Context, int, int) ([]model.User, error) {
	return nil, nil
}
func (r *stubBlockUserRepo) ListByPostCount(context.Context, int, int) ([]model.User, error) {
	return nil, nil
}
func (r *stubBlockUserRepo) Count(_ context.Context) (int64, error)                    { return 0, nil }
func (r *stubBlockUserRepo) CountDistinctPostAuthors(_ context.Context) (int64, error) { return 0, nil }
func (r *stubBlockUserRepo) Create(context.Context, *model.User) error                 { return nil }
func (r *stubBlockUserRepo) Update(context.Context, *model.User) error                 { return nil }
func (r *stubBlockUserRepo) Delete(context.Context, string) error                      { return nil }

func TestUserBlock_BlockAndEither(t *testing.T) {
	users := &stubBlockUserRepo{users: map[string]*model.User{
		"u1": {ID: "u1", Username: "a"},
		"u2": {ID: "u2", Username: "b"},
	}}
	repo := &stubBlockRepo{}
	svc := NewUserBlockService(repo, users)

	require.NoError(t, svc.Block(context.Background(), "u1", "u2"))
	ok, err := svc.IsBlocked(context.Background(), "u1", "u2")
	require.NoError(t, err)
	assert.True(t, ok)

	either, err := svc.EitherBlocked(context.Background(), "u2", "u1")
	require.NoError(t, err)
	assert.True(t, either)

	err = svc.Block(context.Background(), "u1", "u1")
	assert.Error(t, err)
}
