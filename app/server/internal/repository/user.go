package repository

import (
	"context"
	"feedchamber/internal/db"
	"feedchamber/internal/model"
)

type UserRepository interface {
	FindByID(ctx context.Context, id string) (*model.User, error)
	FindByEmail(ctx context.Context, email string) (*model.User, error)
	FindByUsername(ctx context.Context, username string) (*model.User, error)
	FindByIdentifier(ctx context.Context, identifier string) (*model.User, error)
	List(ctx context.Context, offset, limit int) ([]model.User, error)
	Search(ctx context.Context, keyword, role string, offset, limit int) ([]model.User, error)
	CountUsersByPermission(ctx context.Context, permCode string) (int64, error)
	GetActivityStats(ctx context.Context, userID string, year int) (map[string]int, error)
	CountPosts(ctx context.Context, userID string) (int64, error)
	CountCommentsByUser(ctx context.Context, userID string) (int64, error)
	CountLikesByUser(ctx context.Context, userID string) (int64, error)
	CountLikesReceived(ctx context.Context, userID string) (int64, error)
	Create(ctx context.Context, user *model.User) error
	Update(ctx context.Context, user *model.User) error
	IncrPoints(ctx context.Context, userID string, delta int) error
	ListByPoints(ctx context.Context, offset, limit int) ([]model.User, error)
	ListByCheckIns(ctx context.Context, offset, limit int) ([]model.User, error)
	ListByPostCount(ctx context.Context, offset, limit int) ([]model.User, error)
	Count(ctx context.Context) (int64, error)
	CountDistinctPostAuthors(ctx context.Context) (int64, error)
	Delete(ctx context.Context, id string) error
}

func NewUserRepository(store db.Store) UserRepository {
	return &sqlUserRepo{db: store.SQL()}
}
