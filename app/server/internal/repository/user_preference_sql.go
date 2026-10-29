package repository

import (
	"context"
	"orinforum/internal/db"
	"orinforum/internal/model"
)

type sqlUserPrefRepo struct{ db db.SQLStore }

func (r *sqlUserPrefRepo) Get(ctx context.Context, userID, key string) (string, error) {
	var p model.UserPreference
	err := WrapDBErr(r.db.DB().WithContext(ctx).First(&p, "user_id = ? AND key = ?", userID, key).Error)
	if err != nil {
		return "", err
	}
	return p.Value, nil
}

func (r *sqlUserPrefRepo) Set(ctx context.Context, userID, key, value string) error {
	p := model.UserPreference{UserID: userID, Key: key, Value: value}
	return r.db.DB().WithContext(ctx).Save(&p).Error
}

func (r *sqlUserPrefRepo) ListByKey(ctx context.Context, key, value string, offset, limit int) ([]model.UserPreference, error) {
	var items []model.UserPreference
	q := r.db.DB().WithContext(ctx).Where("key = ?", key)
	if value != "" {
		q = q.Where("value = ?", value)
	}
	err := q.Offset(offset).Limit(limit).Find(&items).Error
	return items, err
}

func (r *sqlUserPrefRepo) ListUsersWithKeyDefault(ctx context.Context, key, defaultValue string, offset, limit int) ([]string, error) {
	// Users with explicit preference OR no preference (default weekly).
	// For MVP: list all users and filter in service is expensive; return users with preference=weekly plus users without preference via raw query.
	var ids []string
	err := r.db.DB().WithContext(ctx).Raw(`
		SELECT u.id FROM users u
		LEFT JOIN user_preferences p ON p.user_id = u.id AND p.key = ?
		WHERE COALESCE(p.value, ?) = ?
		ORDER BY u.created_at DESC
		LIMIT ? OFFSET ?
	`, key, defaultValue, defaultValue, limit, offset).Scan(&ids).Error
	return ids, err
}
