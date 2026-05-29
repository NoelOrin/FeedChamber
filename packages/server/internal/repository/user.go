package repository

import (
	"context"
	"orinforum/internal/db"
	"orinforum/internal/model"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type UserRepository interface {
	FindByID(ctx context.Context, id string) (*model.User, error)
	FindByUUID(ctx context.Context, uuid string) (*model.User, error)
	FindByEmail(ctx context.Context, email string) (*model.User, error)
	FindByUsername(ctx context.Context, username string) (*model.User, error)
	List(ctx context.Context, offset, limit int) ([]model.User, error)
	Search(ctx context.Context, keyword, role string, offset, limit int) ([]model.User, error)
	Create(ctx context.Context, user *model.User) error
	Update(ctx context.Context, user *model.User) error
	Delete(ctx context.Context, id string) error
}

func NewUserRepository(store db.Store) UserRepository {
	if store.SQL() != nil {
		return &sqlUserRepo{db: store.SQL()}
	}
	return &mongoUserRepo{store: store.Mongo()}
}

// ── SQL implementation ──────────────────────────────────────────────────────

type sqlUserRepo struct{ db db.SQLStore }

func (r *sqlUserRepo) FindByID(ctx context.Context, id string) (*model.User, error) {
	var u model.User
	return &u, r.db.DB().WithContext(ctx).First(&u, "id = ?", id).Error
}

func (r *sqlUserRepo) FindByUUID(ctx context.Context, uuid string) (*model.User, error) {
	var u model.User
	return &u, r.db.DB().WithContext(ctx).First(&u, "uuid = ?", uuid).Error
}

func (r *sqlUserRepo) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	var u model.User
	return &u, r.db.DB().WithContext(ctx).First(&u, "email = ?", email).Error
}

func (r *sqlUserRepo) FindByUsername(ctx context.Context, username string) (*model.User, error) {
	var u model.User
	return &u, r.db.DB().WithContext(ctx).First(&u, "username = ?", username).Error
}

func (r *sqlUserRepo) List(ctx context.Context, offset, limit int) ([]model.User, error) {
	var users []model.User
	err := r.db.DB().WithContext(ctx).Order("created_at desc").Offset(offset).Limit(limit).Find(&users).Error
	return users, err
}

func (r *sqlUserRepo) Search(ctx context.Context, keyword, role string, offset, limit int) ([]model.User, error) {
	query := r.db.DB().WithContext(ctx).Model(&model.User{})
	if keyword != "" {
		query = query.Where("username LIKE ? OR email LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	if role != "" {
		query = query.Where("role = ?", role)
	}
	var users []model.User
	err := query.Order("created_at desc").Offset(offset).Limit(limit).Find(&users).Error
	return users, err
}

func (r *sqlUserRepo) Create(ctx context.Context, user *model.User) error {
	return r.db.DB().WithContext(ctx).Create(user).Error
}

func (r *sqlUserRepo) Update(ctx context.Context, user *model.User) error {
	return r.db.DB().WithContext(ctx).Save(user).Error
}

func (r *sqlUserRepo) Delete(ctx context.Context, id string) error {
	return r.db.DB().WithContext(ctx).Delete(&model.User{}, "id = ?", id).Error
}

// ── MongoDB implementation ──────────────────────────────────────────────────

type mongoUserRepo struct{ store db.MongoStore }

func (r *mongoUserRepo) FindByID(ctx context.Context, id string) (*model.User, error) {
	var u model.User
	err := r.store.Collection("users").FindOne(ctx, bson.M{"_id": id}).Decode(&u)
	return &u, err
}

func (r *mongoUserRepo) FindByUUID(ctx context.Context, uuid string) (*model.User, error) {
	var u model.User
	err := r.store.Collection("users").FindOne(ctx, bson.M{"uuid": uuid}).Decode(&u)
	return &u, err
}

func (r *mongoUserRepo) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	var u model.User
	err := r.store.Collection("users").FindOne(ctx, bson.M{"email": email}).Decode(&u)
	return &u, err
}

func (r *mongoUserRepo) FindByUsername(ctx context.Context, username string) (*model.User, error) {
	var u model.User
	err := r.store.Collection("users").FindOne(ctx, bson.M{"username": username}).Decode(&u)
	return &u, err
}

func (r *mongoUserRepo) List(ctx context.Context, offset, limit int) ([]model.User, error) {
	cursor, err := r.store.Collection("users").Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	var users []model.User
	err = cursor.All(ctx, &users)
	return users, err
}

func (r *mongoUserRepo) Search(ctx context.Context, keyword, role string, offset, limit int) ([]model.User, error) {
	filter := bson.M{}
	if keyword != "" {
		filter["$or"] = bson.A{
			bson.M{"username": bson.M{"$regex": keyword, "$options": "i"}},
			bson.M{"email": bson.M{"$regex": keyword, "$options": "i"}},
		}
	}
	if role != "" {
		filter["role"] = role
	}
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}).SetSkip(int64(offset)).SetLimit(int64(limit))
	cursor, err := r.store.Collection("users").Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	var users []model.User
	err = cursor.All(ctx, &users)
	return users, err
}

func (r *mongoUserRepo) Create(ctx context.Context, user *model.User) error {
	_, err := r.store.Collection("users").InsertOne(ctx, user)
	return err
}

func (r *mongoUserRepo) Update(ctx context.Context, user *model.User) error {
	_, err := r.store.Collection("users").ReplaceOne(ctx, bson.M{"_id": user.ID}, user)
	return err
}

func (r *mongoUserRepo) Delete(ctx context.Context, id string) error {
	_, err := r.store.Collection("users").DeleteOne(ctx, bson.M{"_id": id})
	return err
}
