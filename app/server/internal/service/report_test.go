package service

import (
	"context"
	"errors"
	"testing"

	"feedchamber/internal/mock"
	"feedchamber/internal/model"
	"feedchamber/internal/pkg"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReportGetByID_Found(t *testing.T) {
	repo := mock.NewReportRepository()
	svc := NewReportService(repo, nil, nil, nil, nil)

	report := &model.Report{
		ID:         "r1",
		ReporterID: "user-1",
		TargetType: "post",
		TargetID:   "post-1",
		Reason:     "spam",
		Status:     "pending",
	}
	_ = repo.Create(context.Background(), report)

	result, err := svc.GetByID(context.Background(), "r1")
	assert.NoError(t, err)
	assert.Equal(t, "r1", result.ID)
	assert.Equal(t, "pending", result.Status)
}

func TestReportGetByID_NotFound(t *testing.T) {
	repo := mock.NewReportRepository()
	svc := NewReportService(repo, nil, nil, nil, nil)

	_, err := svc.GetByID(context.Background(), "nonexistent")
	assert.Error(t, err)

	var appErr *pkg.AppError
	assert.True(t, errors.As(err, &appErr))
	assert.Equal(t, pkg.NOT_FOUND, appErr.Code)
}

func TestReportList_ReturnsFilteredTotal(t *testing.T) {
	repo := mock.NewReportRepository()
	svc := NewReportService(repo, nil, nil, nil, nil)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, &model.Report{ID: "r1", Status: "pending", ReporterID: "u1"}))
	require.NoError(t, repo.Create(ctx, &model.Report{ID: "r2", Status: "pending", ReporterID: "u2"}))
	require.NoError(t, repo.Create(ctx, &model.Report{ID: "r3", Status: "resolved", ReporterID: "u3"}))

	items, total, err := svc.List(ctx, "pending", 1, 1)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, int64(2), total)

	all, allTotal, err := svc.List(ctx, "", 1, 10)
	require.NoError(t, err)
	require.Len(t, all, 3)
	require.Equal(t, int64(3), allTotal)
}

func TestReportCreate_DeduplicatesAndSnapshots(t *testing.T) {
	ctx := context.Background()
	repo := mock.NewReportRepository()
	posts := mock.NewPostRepository()
	contents := mock.NewContentRepository()
	svc := NewReportService(repo, posts, contents, nil, nil)

	require.NoError(t, posts.Create(ctx, &model.Post{
		ID:        "post-1",
		Title:     "spam post",
		ContentID: "content-1",
		AuthorID:  "user-1",
		Status:    model.PostStatusApproved,
	}))
	require.NoError(t, contents.Create(ctx, &model.Content{
		ID:       "content-1",
		RootID:   "content-1",
		Body:     "spam body",
		AuthorID: "user-1",
	}))

	first, err := svc.Create(ctx, "reporter-1", "post", "post-1", "spam", "contains spam")
	require.NoError(t, err)
	require.Equal(t, "spam", first.FlagType)
	require.Equal(t, "spam post", first.TargetTitle)
	require.Equal(t, "spam body", first.TargetBody)
	require.Equal(t, 1, first.Count)

	second, err := svc.Create(ctx, "reporter-2", "post", "post-1", "spam", "another spam")
	require.NoError(t, err)
	require.Equal(t, first.ID, second.ID)
	require.Equal(t, 2, second.Count)
}

func TestReportUpdateStatus_PersistsHandler(t *testing.T) {
	ctx := context.Background()
	repo := mock.NewReportRepository()
	svc := NewReportService(repo, nil, nil, nil, nil)
	report := &model.Report{ID: "r1", Status: "pending", TargetType: "post", TargetID: "p1", Reason: "test"}
	require.NoError(t, repo.Create(ctx, report))

	require.NoError(t, svc.UpdateStatus(ctx, "r1", "resolved", "none", "admin-1", "handled"))
	got, err := svc.GetByID(ctx, "r1")
	require.NoError(t, err)
	require.Equal(t, "resolved", got.Status)
	require.Equal(t, "admin-1", got.HandledBy)
	require.Equal(t, "none", got.Action)
	require.Equal(t, "handled", got.Note)
	require.NotNil(t, got.HandledAt)
}
