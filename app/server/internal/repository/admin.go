package repository

import (
	"context"

	"feedchamber/internal/db"
	"feedchamber/internal/model"
)

// AdminRepository provides admin-level data access.
// Methods are grouped by functional area; the interface is intentionally wide
// because the admin service needs all groups. A future refactor (L-03) can
// compose narrower interfaces (StatsRepo, SearchRepo, etc.) if needed.
type AdminRepository interface {
	// ── Stats ─────────────────────────────────────────
	CountUsers(ctx context.Context) (int64, error)
	CountPosts(ctx context.Context) (int64, error)
	CountComments(ctx context.Context) (int64, error)
	CountTodayUsers(ctx context.Context) (int64, error)
	CountTodayPosts(ctx context.Context) (int64, error)
	CountTodayComments(ctx context.Context) (int64, error)

	// ── Global search ─────────────────────────────────
	SearchUsers(ctx context.Context, keyword string, offset, limit int) ([]model.User, int64, error)
	SearchPosts(ctx context.Context, keyword string, offset, limit int) ([]model.Post, int64, error)
	SearchComments(ctx context.Context, keyword string, offset, limit int) ([]model.Content, int64, error)

	// ── User activities ───────────────────────────────
	GetUserPosts(ctx context.Context, userID string, offset, limit int) ([]model.Post, int64, error)
	GetUserComments(ctx context.Context, userID string, offset, limit int) ([]model.Content, int64, error)
	GetUserReportsAbout(ctx context.Context, userID string, offset, limit int) ([]model.Report, int64, error)
	GetUserReportsBy(ctx context.Context, userID string, offset, limit int) ([]model.Report, int64, error)

	// ── Dashboard extras ──────────────────────────────
	CountPendingReports(ctx context.Context) (int64, error)
	ListRecentUsers(ctx context.Context, limit int) ([]model.User, error)
	ListRecentPosts(ctx context.Context, limit int) ([]model.Post, error)

	// ── Batch operations ──────────────────────────────
	BatchDeleteUsers(ctx context.Context, ids []string) error
	BatchUpdateUserRole(ctx context.Context, ids []string, role string) error
	BatchDeletePosts(ctx context.Context, ids []string) error
	BatchTogglePinPosts(ctx context.Context, ids []string, pinned bool) error
	BatchToggleLockPosts(ctx context.Context, ids []string, locked bool) error
	BatchDeleteComments(ctx context.Context, ids []string) error

	// ── Counter reconciliation (L-06) ─────────────────
	// ReconcileCounters recalculates denormalized counters on posts
	// (comment_count, like_count, favorite_count) from actual data.
	ReconcileCounters(ctx context.Context) (postsFixed int, err error)
}

func NewAdminRepository(store db.Store) AdminRepository {
	return &sqlAdminRepo{db: store.SQL()}
}
