package repository

import (
	"context"
	"fmt"
	"orinforum/internal/db"
	"orinforum/internal/model"
	"regexp"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"gorm.io/gorm"
)

type PostRepository interface {
	List(ctx context.Context, offset, limit int) ([]model.Post, error)
	FindByID(ctx context.Context, id string) (*model.Post, error)
	Create(ctx context.Context, post *model.Post) error
	Update(ctx context.Context, post *model.Post) error
	Delete(ctx context.Context, id string) error
	IncrementViewCount(ctx context.Context, id string) error
	ListWithDetails(ctx context.Context, offset, limit int) ([]model.PostListItem, error)
	FindByIDWithDetails(ctx context.Context, id string) (*model.PostListItem, error)
	Search(ctx context.Context, keyword string, offset, limit int) ([]model.PostListItem, error)
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

func (r *sqlPostRepo) IncrementViewCount(ctx context.Context, id string) error {
	return r.db.DB().WithContext(ctx).Model(&model.Post{}).Where("id = ?", id).
		UpdateColumn("view_count", gorm.Expr("view_count + 1")).Error
}

func (r *sqlPostRepo) ListWithDetails(ctx context.Context, offset, limit int) ([]model.PostListItem, error) {
	var items []model.PostListItem
	err := r.db.DB().WithContext(ctx).Raw(`
		SELECT p.id, p.title, p.content, p.author_id, u.username AS author_name, u.avatar AS author_avatar,
		       p.category_id, c.name AS category_name, p.pinned, p.locked, p.view_count,
		       (SELECT COUNT(*) FROM comments WHERE post_id = p.id) AS comment_count,
		       p.created_at, p.updated_at
		FROM posts p
		LEFT JOIN users u ON p.author_id = u.id
		LEFT JOIN categories c ON p.category_id = c.id
		ORDER BY p.created_at DESC
		LIMIT ? OFFSET ?
	`, limit, offset).Scan(&items).Error
	return items, err
}

func (r *sqlPostRepo) FindByIDWithDetails(ctx context.Context, id string) (*model.PostListItem, error) {
	var item model.PostListItem
	err := r.db.DB().WithContext(ctx).Raw(`
		SELECT p.id, p.title, p.content, p.author_id, u.username AS author_name, u.avatar AS author_avatar,
		       p.category_id, c.name AS category_name, p.pinned, p.locked, p.view_count,
		       (SELECT COUNT(*) FROM comments WHERE post_id = p.id) AS comment_count,
		       p.created_at, p.updated_at
		FROM posts p
		LEFT JOIN users u ON p.author_id = u.id
		LEFT JOIN categories c ON p.category_id = c.id
		WHERE p.id = ?
	`, id).Scan(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *sqlPostRepo) Search(ctx context.Context, keyword string, offset, limit int) ([]model.PostListItem, error) {
	var items []model.PostListItem
	like := "%" + keyword + "%"
	err := r.db.DB().WithContext(ctx).Raw(`
		SELECT p.id, p.title, p.content, p.author_id, u.username AS author_name, u.avatar AS author_avatar,
		       p.category_id, c.name AS category_name, p.pinned, p.locked, p.view_count,
		       (SELECT COUNT(*) FROM comments WHERE post_id = p.id) AS comment_count,
		       p.created_at, p.updated_at
		FROM posts p
		LEFT JOIN users u ON p.author_id = u.id
		LEFT JOIN categories c ON p.category_id = c.id
		WHERE p.title LIKE ? OR p.content LIKE ?
		ORDER BY p.created_at DESC
		LIMIT ? OFFSET ?
	`, like, like, limit, offset).Scan(&items).Error
	return items, err
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

func (r *mongoPostRepo) IncrementViewCount(ctx context.Context, id string) error {
	_, err := r.store.Collection("posts").UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$inc": bson.M{"view_count": 1}})
	return err
}

// assemblePostItems enriches raw posts with author, category, and comment count data.
func (r *mongoPostRepo) assemblePostItems(ctx context.Context, posts []model.Post) ([]model.PostListItem, error) {
	if len(posts) == 0 {
		return []model.PostListItem{}, nil
	}

	// Collect unique author and category IDs
	authorIDs := make(map[string]struct{})
	categoryIDs := make(map[string]struct{})
	postIDs := make([]string, len(posts))
	for i, p := range posts {
		postIDs[i] = p.ID
		authorIDs[p.AuthorID] = struct{}{}
		if p.CategoryID != "" {
			categoryIDs[p.CategoryID] = struct{}{}
		}
	}

	// Lookup authors
	authorMap := make(map[string]struct{ Name, Avatar string })
	aIDs := make([]string, 0, len(authorIDs))
	for id := range authorIDs {
		aIDs = append(aIDs, id)
	}
	cursor, err := r.store.Collection("users").Find(ctx, bson.M{"_id": bson.M{"$in": aIDs}})
	if err != nil {
		return nil, err
	}
	var users []struct {
		ID       string `bson:"_id"`
		Username string `bson:"username"`
		Avatar   string `bson:"avatar"`
	}
	if err := cursor.All(ctx, &users); err != nil {
		return nil, err
	}
	for _, u := range users {
		authorMap[u.ID] = struct{ Name, Avatar string }{Name: u.Username, Avatar: u.Avatar}
	}

	// Lookup categories
	categoryMap := make(map[string]string)
	cIDs := make([]string, 0, len(categoryIDs))
	for id := range categoryIDs {
		cIDs = append(cIDs, id)
	}
	if len(cIDs) > 0 {
		cursor, err = r.store.Collection("categories").Find(ctx, bson.M{"_id": bson.M{"$in": cIDs}})
		if err != nil {
			return nil, err
		}
		var cats []struct {
			ID   string `bson:"_id"`
			Name string `bson:"name"`
		}
		if err := cursor.All(ctx, &cats); err != nil {
			return nil, err
		}
		for _, c := range cats {
			categoryMap[c.ID] = c.Name
		}
	}

	// Count comments per post using aggregation
	commentCounts := make(map[string]int64)
	cursor, err = r.store.Collection("comments").Aggregate(ctx, bson.A{
		bson.M{"$match": bson.M{"post_id": bson.M{"$in": postIDs}}},
		bson.M{"$group": bson.M{"_id": "$post_id", "count": bson.M{"$sum": 1}}},
	})
	if err != nil {
		return nil, err
	}
	var countResults []struct {
		ID    string `bson:"_id"`
		Count int64  `bson:"count"`
	}
	if err := cursor.All(ctx, &countResults); err != nil {
		return nil, err
	}
	for _, cr := range countResults {
		commentCounts[cr.ID] = cr.Count
	}

	// Assemble PostListItem
	items := make([]model.PostListItem, len(posts))
	for i, p := range posts {
		author := authorMap[p.AuthorID]
		items[i] = model.PostListItem{
			ID:           p.ID,
			Title:        p.Title,
			Content:      p.Content,
			AuthorID:     p.AuthorID,
			AuthorName:   author.Name,
			AuthorAvatar: author.Avatar,
			CategoryID:   p.CategoryID,
			CategoryName: categoryMap[p.CategoryID],
			Pinned:       p.Pinned,
			Locked:       p.Locked,
			ViewCount:    p.ViewCount,
			CommentCount: commentCounts[p.ID],
			CreatedAt:    p.CreatedAt,
			UpdatedAt:    p.UpdatedAt,
		}
	}
	return items, nil
}

func (r *mongoPostRepo) ListWithDetails(ctx context.Context, offset, limit int) ([]model.PostListItem, error) {
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}).SetSkip(int64(offset)).SetLimit(int64(limit))
	cursor, err := r.store.Collection("posts").Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	var posts []model.Post
	if err := cursor.All(ctx, &posts); err != nil {
		return nil, err
	}
	return r.assemblePostItems(ctx, posts)
}

func (r *mongoPostRepo) FindByIDWithDetails(ctx context.Context, id string) (*model.PostListItem, error) {
	var p model.Post
	if err := r.store.Collection("posts").FindOne(ctx, bson.M{"_id": id}).Decode(&p); err != nil {
		return nil, err
	}
	items, err := r.assemblePostItems(ctx, []model.Post{p})
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("post not found")
	}
	return &items[0], nil
}

func (r *mongoPostRepo) Search(ctx context.Context, keyword string, offset, limit int) ([]model.PostListItem, error) {
	escaped := regexp.QuoteMeta(keyword)
	filter := bson.M{"$or": bson.A{
		bson.M{"title": bson.M{"$regex": escaped, "$options": "i"}},
		bson.M{"content": bson.M{"$regex": escaped, "$options": "i"}},
	}}
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}).SetSkip(int64(offset)).SetLimit(int64(limit))
	cursor, err := r.store.Collection("posts").Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	var posts []model.Post
	if err := cursor.All(ctx, &posts); err != nil {
		return nil, err
	}
	return r.assemblePostItems(ctx, posts)
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
	FindByID(ctx context.Context, id string) (*model.Comment, error)
	Create(ctx context.Context, c *model.Comment) error
	Update(ctx context.Context, c *model.Comment) error
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

func (r *sqlCommentRepo) FindByID(ctx context.Context, id string) (*model.Comment, error) {
	var c model.Comment
	err := r.db.DB().WithContext(ctx).First(&c, "id = ?", id).Error
	return &c, err
}

func (r *sqlCommentRepo) Update(ctx context.Context, c *model.Comment) error {
	return r.db.DB().WithContext(ctx).Save(c).Error
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

func (r *mongoCommentRepo) FindByID(ctx context.Context, id string) (*model.Comment, error) {
	var c model.Comment
	err := r.store.Collection("comments").FindOne(ctx, bson.M{"_id": id}).Decode(&c)
	return &c, err
}

func (r *mongoCommentRepo) Update(ctx context.Context, c *model.Comment) error {
	_, err := r.store.Collection("comments").ReplaceOne(ctx, bson.M{"_id": c.ID}, c)
	return err
}

func (r *mongoCommentRepo) Delete(ctx context.Context, id string) error {
	_, err := r.store.Collection("comments").DeleteOne(ctx, bson.M{"_id": id})
	return err
}