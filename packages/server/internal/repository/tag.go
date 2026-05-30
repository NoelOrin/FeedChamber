package repository

import (
	"context"
	"orinforum/internal/db"
	"orinforum/internal/model"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type TagRepository interface {
	List(ctx context.Context) ([]model.Tag, error)
	FindByID(ctx context.Context, id string) (*model.Tag, error)
	GetByName(ctx context.Context, name string) (*model.Tag, error)
	Create(ctx context.Context, tag *model.Tag) error
	CreateIfNotExists(ctx context.Context, tag *model.Tag) error
	Update(ctx context.Context, tag *model.Tag) error
	Delete(ctx context.Context, id string) error
	AddPostTag(ctx context.Context, postID, tagID string) error
	RemovePostTags(ctx context.Context, postID string) error
	GetTagsByPostID(ctx context.Context, postID string) ([]model.Tag, error)
	GetPostsByTagID(ctx context.Context, tagID string, offset, limit int) ([]string, error)
}

func NewTagRepository(store db.Store) TagRepository {
	if store.SQL() != nil {
		return &sqlTagRepo{db: store.SQL()}
	}
	return &mongoTagRepo{store: store.Mongo()}
}

// ── SQL ──────────────────────────────────────────────────────────────────────

type sqlTagRepo struct{ db db.SQLStore }

func (r *sqlTagRepo) List(ctx context.Context) ([]model.Tag, error) {
	var tags []model.Tag
	err := r.db.DB().WithContext(ctx).Order("name asc").Find(&tags).Error
	return tags, err
}

func (r *sqlTagRepo) GetByName(ctx context.Context, name string) (*model.Tag, error) {
	var t model.Tag
	err := r.db.DB().WithContext(ctx).First(&t, "name = ?", name).Error
	return &t, err
}

func (r *sqlTagRepo) FindByID(ctx context.Context, id string) (*model.Tag, error) {
	var t model.Tag
	err := r.db.DB().WithContext(ctx).First(&t, "id = ?", id).Error
	return &t, err
}

func (r *sqlTagRepo) Create(ctx context.Context, tag *model.Tag) error {
	return r.db.DB().WithContext(ctx).Create(tag).Error
}

func (r *sqlTagRepo) CreateIfNotExists(ctx context.Context, tag *model.Tag) error {
	var existing model.Tag
	err := r.db.DB().WithContext(ctx).First(&existing, "name = ?", tag.Name).Error
	if err == nil {
		*tag = existing
		return nil
	}
	return r.db.DB().WithContext(ctx).Create(tag).Error
}

func (r *sqlTagRepo) Update(ctx context.Context, tag *model.Tag) error {
	return r.db.DB().WithContext(ctx).Save(tag).Error
}

func (r *sqlTagRepo) Delete(ctx context.Context, id string) error {
	if err := r.db.DB().WithContext(ctx).Where("tag_id = ?", id).Delete(&model.PostTag{}).Error; err != nil {
		return err
	}
	return r.db.DB().WithContext(ctx).Delete(&model.Tag{}, "id = ?", id).Error
}

func (r *sqlTagRepo) AddPostTag(ctx context.Context, postID, tagID string) error {
	pt := &model.PostTag{PostID: postID, TagID: tagID}
	return r.db.DB().WithContext(ctx).Create(pt).Error
}

func (r *sqlTagRepo) RemovePostTags(ctx context.Context, postID string) error {
	return r.db.DB().WithContext(ctx).Where("post_id = ?", postID).Delete(&model.PostTag{}).Error
}

func (r *sqlTagRepo) GetTagsByPostID(ctx context.Context, postID string) ([]model.Tag, error) {
	var tags []model.Tag
	err := r.db.DB().WithContext(ctx).
		Joins("JOIN post_tags ON post_tags.tag_id = tags.id").
		Where("post_tags.post_id = ?", postID).
		Order("tags.name asc").
		Find(&tags).Error
	return tags, err
}

func (r *sqlTagRepo) GetPostsByTagID(ctx context.Context, tagID string, offset, limit int) ([]string, error) {
	var postIDs []string
	err := r.db.DB().WithContext(ctx).
		Model(&model.PostTag{}).
		Where("tag_id = ?", tagID).
		Offset(offset).Limit(limit).
		Pluck("post_id", &postIDs).Error
	return postIDs, err
}

// ── MongoDB ──────────────────────────────────────────────────────────────────

type mongoTagRepo struct{ store db.MongoStore }

func (r *mongoTagRepo) List(ctx context.Context) ([]model.Tag, error) {
	cursor, err := r.store.Collection("tags").Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	var tags []model.Tag
	err = cursor.All(ctx, &tags)
	return tags, err
}

func (r *mongoTagRepo) GetByName(ctx context.Context, name string) (*model.Tag, error) {
	var t model.Tag
	err := r.store.Collection("tags").FindOne(ctx, bson.M{"name": name}).Decode(&t)
	return &t, err
}

func (r *mongoTagRepo) FindByID(ctx context.Context, id string) (*model.Tag, error) {
	var t model.Tag
	err := r.store.Collection("tags").FindOne(ctx, bson.M{"_id": id}).Decode(&t)
	return &t, err
}

func (r *mongoTagRepo) Create(ctx context.Context, tag *model.Tag) error {
	_, err := r.store.Collection("tags").InsertOne(ctx, tag)
	return err
}

func (r *mongoTagRepo) CreateIfNotExists(ctx context.Context, tag *model.Tag) error {
	err := r.store.Collection("tags").FindOne(ctx, bson.M{"name": tag.Name}).Decode(tag)
	if err == nil {
		return nil
	}
	_, err = r.store.Collection("tags").InsertOne(ctx, tag)
	return err
}

func (r *mongoTagRepo) Update(ctx context.Context, tag *model.Tag) error {
	_, err := r.store.Collection("tags").ReplaceOne(ctx, bson.M{"_id": tag.ID}, tag)
	return err
}

func (r *mongoTagRepo) Delete(ctx context.Context, id string) error {
	_, err := r.store.Collection("post_tags").DeleteMany(ctx, bson.M{"tag_id": id})
	if err != nil {
		return err
	}
	_, err = r.store.Collection("tags").DeleteOne(ctx, bson.M{"_id": id})
	return err
}

func (r *mongoTagRepo) AddPostTag(ctx context.Context, postID, tagID string) error {
	pt := &model.PostTag{PostID: postID, TagID: tagID}
	_, err := r.store.Collection("post_tags").InsertOne(ctx, pt)
	return err
}

func (r *mongoTagRepo) RemovePostTags(ctx context.Context, postID string) error {
	_, err := r.store.Collection("post_tags").DeleteMany(ctx, bson.M{"post_id": postID})
	return err
}

func (r *mongoTagRepo) GetTagsByPostID(ctx context.Context, postID string) ([]model.Tag, error) {
	cursor, err := r.store.Collection("post_tags").Find(ctx, bson.M{"post_id": postID})
	if err != nil {
		return nil, err
	}
	var postTags []model.PostTag
	if err = cursor.All(ctx, &postTags); err != nil {
		return nil, err
	}
	tagIDs := make([]string, len(postTags))
	for i, pt := range postTags {
		tagIDs[i] = pt.TagID
	}
	if len(tagIDs) == 0 {
		return []model.Tag{}, nil
	}
	tagCursor, err := r.store.Collection("tags").Find(ctx, bson.M{"_id": bson.M{"$in": tagIDs}})
	if err != nil {
		return nil, err
	}
	var tags []model.Tag
	err = tagCursor.All(ctx, &tags)
	return tags, err
}

func (r *mongoTagRepo) GetPostsByTagID(ctx context.Context, tagID string, offset, limit int) ([]string, error) {
	opts := options.Find().SetSkip(int64(offset)).SetLimit(int64(limit))
	cursor, err := r.store.Collection("post_tags").Find(ctx, bson.M{"tag_id": tagID}, opts)
	if err != nil {
		return nil, err
	}
	var postTags []model.PostTag
	if err = cursor.All(ctx, &postTags); err != nil {
		return nil, err
	}
	postIDs := make([]string, len(postTags))
	for i, pt := range postTags {
		postIDs[i] = pt.PostID
	}
	return postIDs, nil
}
