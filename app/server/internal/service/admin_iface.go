package service

import (
	"context"

	"orinforum/internal/model"
)

// AdminStats holds aggregated forum statistics.
type AdminStats struct {
	TotalUsers       int64 `json:"total_users"`
	TotalPosts       int64 `json:"total_posts"`
	TotalComments    int64 `json:"total_comments"`
	TodayNewUsers    int64 `json:"today_new_users"`
	TodayNewPosts    int64 `json:"today_new_posts"`
	TodayNewComments int64 `json:"today_new_comments"`
}

// DashboardData is the aggregated data for the admin overview page.
type DashboardData struct {
	Stats          AdminStats   `json:"stats"`
	PendingReports int64        `json:"pending_reports"`
	RecentUsers    []model.User `json:"recent_users"`
	RecentPosts    []model.Post `json:"recent_posts"`
}

// SearchResults groups results from a global search.
type SearchResults struct {
	Users    *EntitySearchResult[model.User]    `json:"users"`
	Posts    *EntitySearchResult[model.Post]    `json:"posts"`
	Comments *EntitySearchResult[model.Content] `json:"comments"`
}

type EntitySearchResult[T any] struct {
	Items    []T   `json:"items"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
}

// UserActivities holds a user's related content.
type UserActivities struct {
	User           model.User      `json:"user"`
	Posts          []model.Post    `json:"posts"`
	PostsTotal     int64           `json:"posts_total"`
	Comments       []model.Content `json:"comments"`
	CommentsTotal  int64           `json:"comments_total"`
	ReportsAbout   []model.Report  `json:"reports_about"`
	ReportsTotal   int64           `json:"reports_total"`
	ReportsBy      []model.Report  `json:"reports_by"`
	ReportsByTotal int64           `json:"reports_by_total"`
}

type AdminService interface {
	GetStats(ctx context.Context) (*AdminStats, error)
	GetDashboard(ctx context.Context) (*DashboardData, error)
	SearchUsers(ctx context.Context, query string, page, pageSize int) ([]model.User, int64, error)
	SearchPosts(ctx context.Context, query string, page, pageSize int) ([]model.Post, int64, error)
	SearchComments(ctx context.Context, query string, page, pageSize int) ([]model.Content, int64, error)
	GetUserActivities(ctx context.Context, userID string, page, pageSize int) (*UserActivities, error)
	BatchUserAction(ctx context.Context, action string, ids []string, params map[string]string) error
	BatchPostAction(ctx context.Context, action string, ids []string, actorRole string) error
	BatchDeleteComments(ctx context.Context, ids []string) error
	ReconcileCounters(ctx context.Context) (int, error)
	InvalidateStatsCache(ctx context.Context) error
}
