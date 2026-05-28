package repository

import (
	"context"
	"orinforum/internal/db"
	"orinforum/internal/model"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type RoleRepository interface {
	List(ctx context.Context) ([]model.Role, error)
	GetByName(ctx context.Context, name string) (*model.Role, error)
	Create(ctx context.Context, role *model.Role) error
	CreateIfNotExists(ctx context.Context, role *model.Role) error
	Delete(ctx context.Context, id string) error
}

func NewRoleRepository(store db.Store) RoleRepository {
	if store.SQL() != nil {
		return &sqlRoleRepo{db: store.SQL()}
	}
	return &mongoRoleRepo{store: store.Mongo()}
}

// ── SQL ─────────────────────────────────────────────────────────────────────

type sqlRoleRepo struct{ db db.SQLStore }

func (r *sqlRoleRepo) List(ctx context.Context) ([]model.Role, error) {
	var roles []model.Role
	err := r.db.DB().WithContext(ctx).Find(&roles).Error
	return roles, err
}

func (r *sqlRoleRepo) GetByName(ctx context.Context, name string) (*model.Role, error) {
	var role model.Role
	err := r.db.DB().WithContext(ctx).Where("name = ?", name).First(&role).Error
	return &role, err
}

func (r *sqlRoleRepo) Create(ctx context.Context, role *model.Role) error {
	return r.db.DB().WithContext(ctx).Create(role).Error
}

func (r *sqlRoleRepo) CreateIfNotExists(ctx context.Context, role *model.Role) error {
	return r.db.DB().WithContext(ctx).Where("name = ?", role.Name).FirstOrCreate(role).Error
}

func (r *sqlRoleRepo) Delete(ctx context.Context, id string) error {
	result := r.db.DB().WithContext(ctx).Delete(&model.Role{}, "id = ?", id)
	return result.Error
}

// ── Mongo ───────────────────────────────────────────────────────────────────

type mongoRoleRepo struct{ store db.MongoStore }

func (r *mongoRoleRepo) List(ctx context.Context) ([]model.Role, error) {
	cursor, err := r.store.Collection("roles").Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	var roles []model.Role
	err = cursor.All(ctx, &roles)
	return roles, err
}

func (r *mongoRoleRepo) GetByName(ctx context.Context, name string) (*model.Role, error) {
	var role model.Role
	err := r.store.Collection("roles").FindOne(ctx, bson.M{"name": name}).Decode(&role)
	return &role, err
}

func (r *mongoRoleRepo) Create(ctx context.Context, role *model.Role) error {
	_, err := r.store.Collection("roles").InsertOne(ctx, role)
	return err
}

func (r *mongoRoleRepo) CreateIfNotExists(ctx context.Context, role *model.Role) error {
	existing, _ := r.GetByName(ctx, role.Name)
	if existing != nil {
		return nil
	}
	return r.Create(ctx, role)
}

func (r *mongoRoleRepo) Delete(ctx context.Context, id string) error {
	_, err := r.store.Collection("roles").DeleteOne(ctx, bson.M{"_id": id})
	return err
}