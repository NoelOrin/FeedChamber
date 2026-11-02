package service

import (
	"context"
	"testing"
	"time"

	"feedchamber/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubAuditLogRepo struct {
	items []model.AuditLog
}

func (s *stubAuditLogRepo) Create(ctx context.Context, log *model.AuditLog) error {
	s.items = append(s.items, *log)
	return nil
}

func (s *stubAuditLogRepo) List(ctx context.Context, offset, limit int, actorID, action string) ([]model.AuditLog, int64, error) {
	var out []model.AuditLog
	for _, it := range s.items {
		if actorID != "" && it.ActorID != actorID {
			continue
		}
		if action != "" && it.Action != action {
			continue
		}
		out = append(out, it)
	}
	total := int64(len(out))
	if offset >= len(out) {
		return []model.AuditLog{}, total, nil
	}
	end := offset + limit
	if end > len(out) {
		end = len(out)
	}
	return out[offset:end], total, nil
}

func TestAuditLogService_RecordAndList(t *testing.T) {
	repo := &stubAuditLogRepo{}
	svc := NewAuditLogService(repo)
	ctx := context.Background()

	require.NoError(t, svc.Record(ctx, "actor-1", "settings.update", "site_name", "Orin"))
	require.NoError(t, svc.Log(ctx, "actor-2", "user.ban", "user-9", `{"reason":"spam"}`))

	// allow time fields to differ
	assert.Len(t, repo.items, 2)
	assert.NotEmpty(t, repo.items[0].ID)
	assert.False(t, repo.items[0].CreatedAt.IsZero())
	assert.WithinDuration(t, time.Now(), repo.items[0].CreatedAt, 2*time.Second)

	items, total, err := svc.List(ctx, 1, 10, "actor-1", "settings.update")
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	assert.Equal(t, "site_name", items[0].Target)
}
