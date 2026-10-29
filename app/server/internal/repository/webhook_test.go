package repository

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"feedchamber/internal/config"
	"feedchamber/internal/db"
	"feedchamber/internal/model"

	"github.com/stretchr/testify/require"
)

func newWebhookTestRepo(t *testing.T) WebhookRepository {
	t.Helper()
	store, err := db.New(config.DBConfig{Type: "sqlite", DSN: filepath.Join(t.TempDir(), "webhook.db")})
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	require.NoError(t, store.AutoMigrate(&model.Webhook{}, &model.WebhookDelivery{}))
	return NewWebhookRepository(store)
}

func mkDelivery(id, status string, attempts int, nextRetry *time.Time) model.WebhookDelivery {
	return model.WebhookDelivery{
		ID:          id,
		WebhookID:   "wh1",
		Event:       "post.created",
		Payload:     "{}",
		Status:      status,
		Attempts:    attempts,
		NextRetryAt: nextRetry,
		CreatedAt:   time.Now(),
	}
}

// TestCountPendingDeliveries_IsNotCappedByBatchSize is the regression test for
// the backlog gauge bug: the gauge used to be fed len(ListPendingDeliveries(20)),
// so a 35-deep backlog was reported as 20 — indistinguishable from exactly 20.
//
// The two queries must agree on the predicate but not on the limit.
func TestCountPendingDeliveries_IsNotCappedByBatchSize(t *testing.T) {
	repo := newWebhookTestRepo(t)
	ctx := context.Background()

	const total = 35
	for i := 0; i < total; i++ {
		require.NoError(t, repo.CreateDelivery(ctx, ptrDelivery(mkDelivery(fmt.Sprintf("d-%02d", i), "pending", 0, nil))))
	}

	count, err := repo.CountPendingDeliveries(ctx)
	require.NoError(t, err)
	require.EqualValues(t, total, count, "count must report the true backlog, not the batch size")

	batch, err := repo.ListPendingDeliveries(ctx, 20)
	require.NoError(t, err)
	require.Len(t, batch, 20, "the worker batch stays capped at its limit")

	// Guard the exact bug: had the gauge used this, it would read 20.
	require.NotEqualValues(t, count, int64(len(batch)))
}

// TestCountPendingDeliveries_PredicateMatchesList pins that the count and the
// list select the same rows. If they drift, the gauge reports work the worker
// will never pick up (or misses work it will).
func TestCountPendingDeliveries_PredicateMatchesList(t *testing.T) {
	repo := newWebhookTestRepo(t)
	ctx := context.Background()

	future := time.Now().Add(time.Hour)
	rows := []model.WebhookDelivery{
		mkDelivery("pending-fresh", "pending", 0, nil),
		mkDelivery("pending-retry-waiting", "pending", 2, &future),
		mkDelivery("pending-max-attempts", "pending", 5, nil), // attempts >= 5: excluded
		mkDelivery("success", "success", 1, nil),
		mkDelivery("failed", "failed", 5, nil),
	}
	for _, d := range rows {
		require.NoError(t, repo.CreateDelivery(ctx, ptrDelivery(d)))
	}

	count, err := repo.CountPendingDeliveries(ctx)
	require.NoError(t, err)

	// A future NextRetryAt is still undelivered work: including it is what makes
	// a retry storm visible instead of looking like an empty queue.
	require.EqualValues(t, 2, count)

	listed, err := repo.ListPendingDeliveries(ctx, 100)
	require.NoError(t, err)
	require.Len(t, listed, int(count), "count and list must select the same rows")
}

func ptrDelivery(d model.WebhookDelivery) *model.WebhookDelivery { return &d }
