package repository

import (
	"context"
	"orinforum/internal/db"
	"orinforum/internal/model"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type PostRepository interface {
	List(ctx context.Context, offset, limit int) ([]model.Post, error)
	FindByID(ctx context.Context, id string) (*model.Post, error)
	Create(ctx context.Context, post *model.Post) error
	Update(ctx context.Context, post *model.Post) error
	Delete(ctx context.Context, id string) error
}

func NewPostRepository(store db.Store) PostRepository {
	if store.SQL() != nil {
		return &sqlPostRepo{db: store.SQL()}
	}
	return &mongoPostRepo{store: store.Mongo()}
}

type sqlPostRepo struct{ db db.SQLStore }

func (r *sqlPostRepo) List(ctx context.Context, offset, limit int) ([]model.Post, error) {
	var posts []model.Post
	err := r.db.DB().WithContext(ctx).Order("created_at desc").Offset(offset).Limit(limit).Find(&posts).Error
	return posts, err
}

func (r *sqlPostRepo) FindByID(ctx context.Context, id string) (*model.Post, error) {
	var p model.Post
	err := r.db.DB().WithContext(ctx).First(&p, "id = ?", id).Error
	return &p, err
}

func (r *sqlPostRepo) Create(ctx context.Context, post *model.Post) error {
	return r.db.DB().WithContext(ctx).Create(post).Error
}

func (r *sqlPostRepo) Update(ctx context.Context, post *model.Post) error {
	return r.db.DB().WithContext(ctx).Save(post).Error
}

func (r *sqlPostRepo) Delete(ctx context.Context, id string) error {
	return r.db.DB().WithContext(ctx).Delete(&model.Post{}, "id = ?", id).Error
}

type mongoPostRepo struct{ store db.MongoStore }

func (r *mongoPostRepo) List(ctx context.Context, offset, limit int) ([]model.Post, error) {
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})
	cursor, err := r.store.Collection("posts").Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	var posts []model.Post
	err = cursor.All(ctx, &posts)
	return posts, err
}

func (r *mongoPostRepo) FindByID(ctx context.Context, id string) (*model.Post, error) {
	var p model.Post
	err := r.store.Collection("posts").FindOne(ctx, bson.M{"_id": id}).Decode(&p)
	return &p, err
}

func (r *mongoPostRepo) Create(ctx context.Context, post *model.Post) error {
	_, err := r.store.Collection("posts").InsertOne(ctx, post)
	return err
}

func (r *mongoPostRepo) Update(ctx context.Context, post *model.Post) error {
	_, err := r.store.Collection("posts").ReplaceOne(ctx, bson.M{"_id": post.ID}, post)
	return err
}

func (r *mongoPostRepo) Delete(ctx context.Context, id string) error {
	_, err := r.store.Collection("posts").DeleteOne(ctx, bson.M{"_id": id})
	return err
}

// ── Category ─────────────────────────────────────────────────────────────────

type CategoryRepository interface {
	List(ctx context.Context) ([]model.Category, error)
	Create(ctx context.Context, c *model.Category) error
	Delete(ctx context.Context, id string) error
}

func NewCategoryRepository(store db.Store) CategoryRepository {
	if store.SQL() != nil {
		return &sqlCategoryRepo{db: store.SQL()}
	}
	return &mongoCategoryRepo{store: store.Mongo()}
}

type sqlCategoryRepo struct{ db db.SQLStore }

func (r *sqlCategoryRepo) List(ctx context.Context) ([]model.Category, error) {
	var cats []model.Category
	err := r.db.DB().WithContext(ctx).Order("name asc").Find(&cats).Error
	return cats, err
}

func (r *sqlCategoryRepo) Create(ctx context.Context, c *model.Category) error {
	return r.db.DB().WithContext(ctx).Create(c).Error
}

func (r *sqlCategoryRepo) Delete(ctx context.Context, id string) error {
	return r.db.DB().WithContext(ctx).Delete(&model.Category{}, "id = ?", id).Error
}

type mongoCategoryRepo struct{ store db.MongoStore }

func (r *mongoCategoryRepo) List(ctx context.Context) ([]model.Category, error) {
	cursor, err := r.store.Collection("categories").Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	var cats []model.Category
	err = cursor.All(ctx, &cats)
	return cats, err
}

func (r *mongoCategoryRepo) Create(ctx context.Context, c *model.Category) error {
	_, err := r.store.Collection("categories").InsertOne(ctx, c)
	return err
}

func (r *mongoCategoryRepo) Delete(ctx context.Context, id string) error {
	_, err := r.store.Collection("categories").DeleteOne(ctx, bson.M{"_id": id})
	return err
}

// ── Comment ──────────────────────────────────────────────────────────────────

type CommentRepository interface {
	ListByPost(ctx context.Context, postID string) ([]model.Comment, error)
	Create(ctx context.Context, c *model.Comment) error
	Delete(ctx context.Context, id string) error
}

func NewCommentRepository(store db.Store) CommentRepository {
	if store.SQL() != nil {
		return &sqlCommentRepo{db: store.SQL()}
	}
	return &mongoCommentRepo{store: store.Mongo()}
}

type sqlCommentRepo struct{ db db.SQLStore }

func (r *sqlCommentRepo) ListByPost(ctx context.Context, postID string) ([]model.Comment, error) {
	var comments []model.Comment
	err := r.db.DB().WithContext(ctx).Where("post_id = ?", postID).Order("created_at asc").Find(&comments).Error
	return comments, err
}

func (r *sqlCommentRepo) Create(ctx context.Context, c *model.Comment) error {
	return r.db.DB().WithContext(ctx).Create(c).Error
}

func (r *sqlCommentRepo) Delete(ctx context.Context, id string) error {
	return r.db.DB().WithContext(ctx).Delete(&model.Comment{}, "id = ?", id).Error
}

type mongoCommentRepo struct{ store db.MongoStore }

func (r *mongoCommentRepo) ListByPost(ctx context.Context, postID string) ([]model.Comment, error) {
	cursor, err := r.store.Collection("comments").Find(ctx, bson.M{"post_id": postID})
	if err != nil {
		return nil, err
	}
	var comments []model.Comment
	err = cursor.All(ctx, &comments)
	return comments, err
}

func (r *mongoCommentRepo) Create(ctx context.Context, c *model.Comment) error {
	_, err := r.store.Collection("comments").InsertOne(ctx, c)
	return err
}

func (r *mongoCommentRepo) Delete(ctx context.Context, id string) error {
	_, err := r.store.Collection("comments").DeleteOne(ctx, bson.M{"_id": id})
	return err
}