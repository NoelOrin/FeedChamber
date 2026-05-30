package repository

import (
	"context"
	"fmt"
	"orinforum/internal/db"
	"orinforum/internal/model"
	"regexp"
	"time"

	"github.com/google/uuid"
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
	IncrementCommentCount(ctx context.Context, id string) error
	DecrementCommentCount(ctx context.Context, id string) error
	ListWithDetails(ctx context.Context, offset, limit int, categoryID, authorID, status string) ([]model.PostListItem, error)
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

func (r *sqlPostRepo) IncrementCommentCount(ctx context.Context, id string) error {
	return r.db.DB().WithContext(ctx).Model(&model.Post{}).Where("id = ?", id).
		UpdateColumn("comment_count", gorm.Expr("comment_count + 1")).Error
}

func (r *sqlPostRepo) DecrementCommentCount(ctx context.Context, id string) error {
	return r.db.DB().WithContext(ctx).Model(&model.Post{}).Where("id = ? AND comment_count > 0", id).
		UpdateColumn("comment_count", gorm.Expr("comment_count - 1")).Error
}

func (r *sqlPostRepo) ListWithDetails(ctx context.Context, offset, limit int, categoryID, authorID, status string) ([]model.PostListItem, error) {
	var items []model.PostListItem
	query := `
		SELECT p.id, p.title, p.content_digest, p.author_id, u.username AS author_name, u.avatar AS author_avatar,
		       p.category_id, c.name AS category_name, p.status, p.pinned, p.locked, p.view_count,
		       p.comment_count, p.like_count, p.favorite_count,
		       p.created_at, p.updated_at
		FROM posts p
		LEFT JOIN users u ON p.author_id = u.id
		LEFT JOIN categories c ON p.category_id = c.id
		WHERE p.deleted_at IS NULL`
	args := []interface{}{}
	if categoryID != "" {
		query += " AND p.category_id = ?"
		args = append(args, categoryID)
	}
	if authorID != "" {
		query += " AND p.author_id = ?"
		args = append(args, authorID)
	}
	if status != "" {
		query += " AND p.status = ?"
		args = append(args, status)
	}
	query += " ORDER BY p.created_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)
	err := r.db.DB().WithContext(ctx).Raw(query, args...).Scan(&items).Error
	return items, err
}

func (r *sqlPostRepo) FindByIDWithDetails(ctx context.Context, id string) (*model.PostListItem, error) {
	var item model.PostListItem
	err := r.db.DB().WithContext(ctx).Raw(`
		SELECT p.id, p.title, p.content_digest, p.author_id, u.username AS author_name, u.avatar AS author_avatar,
		       p.category_id, c.name AS category_name, p.status, p.pinned, p.locked, p.view_count,
		       p.comment_count, p.like_count, p.favorite_count,
		       p.created_at, p.updated_at
		FROM posts p
		LEFT JOIN users u ON p.author_id = u.id
		LEFT JOIN categories c ON p.category_id = c.id
		WHERE p.id = ? AND p.deleted_at IS NULL
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
		SELECT p.id, p.title, p.content_digest, p.author_id, u.username AS author_name, u.avatar AS author_avatar,
		       p.category_id, c.name AS category_name, p.status, p.pinned, p.locked, p.view_count,
		       p.comment_count, p.like_count, p.favorite_count,
		       p.created_at, p.updated_at
		FROM posts p
		LEFT JOIN users u ON p.author_id = u.id
		LEFT JOIN categories c ON p.category_id = c.id
		WHERE p.deleted_at IS NULL AND (p.title LIKE ? OR p.content_digest LIKE ?)
		ORDER BY p.created_at DESC
		LIMIT ? OFFSET ?
	`, like, like, limit, offset).Scan(&items).Error
	return items, err
}

type mongoPostRepo struct{ store db.MongoStore }

func (r *mongoPostRepo) List(ctx context.Context, offset, limit int) ([]model.Post, error) {
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})
	cursor, err := r.store.Collection("posts").Find(ctx, bson.M{"deleted_at": nil}, opts)
	if err != nil {
		return nil, err
	}
	var posts []model.Post
	err = cursor.All(ctx, &posts)
	return posts, err
}

func (r *mongoPostRepo) FindByID(ctx context.Context, id string) (*model.Post, error) {
	var p model.Post
	err := r.store.Collection("posts").FindOne(ctx, bson.M{"_id": id, "deleted_at": nil}).Decode(&p)
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
	_, err := r.store.Collection("posts").UpdateOne(ctx,
		bson.M{"_id": id, "deleted_at": nil},
		bson.M{"$set": bson.M{"deleted_at": time.Now()}},
	)
	return err
}

func (r *mongoPostRepo) IncrementViewCount(ctx context.Context, id string) error {
	_, err := r.store.Collection("posts").UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$inc": bson.M{"view_count": 1}})
	return err
}

func (r *mongoPostRepo) IncrementCommentCount(ctx context.Context, id string) error {
	_, err := r.store.Collection("posts").UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$inc": bson.M{"comment_count": 1}})
	return err
}

func (r *mongoPostRepo) DecrementCommentCount(ctx context.Context, id string) error {
	_, err := r.store.Collection("posts").UpdateOne(ctx, bson.M{"_id": id, "comment_count": bson.M{"$gt": 0}}, bson.M{"$inc": bson.M{"comment_count": -1}})
	return err
}

func (r *mongoPostRepo) assemblePostItems(ctx context.Context, posts []model.Post) ([]model.PostListItem, error) {
	if len(posts) == 0 {
		return []model.PostListItem{}, nil
	}

	authorIDs := make(map[string]struct{})
	categoryIDs := make(map[string]struct{})
	for _, p := range posts {
		authorIDs[p.AuthorID] = struct{}{}
		if p.CategoryID != "" {
			categoryIDs[p.CategoryID] = struct{}{}
		}
	}

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

	items := make([]model.PostListItem, len(posts))
	for i, p := range posts {
		author := authorMap[p.AuthorID]
		items[i] = model.PostListItem{
			ID:            p.ID,
			Title:         p.Title,
			ContentDigest: p.ContentDigest,
			AuthorID:      p.AuthorID,
			AuthorName:    author.Name,
			AuthorAvatar:  author.Avatar,
			CategoryID:    p.CategoryID,
			CategoryName:  categoryMap[p.CategoryID],
			Status:        p.Status,
			Pinned:        p.Pinned,
			Locked:        p.Locked,
			ViewCount:     p.ViewCount,
			CommentCount:  p.CommentCount,
			LikeCount:     p.LikeCount,
			FavoriteCount: p.FavoriteCount,
			CreatedAt:     p.CreatedAt,
			UpdatedAt:     p.UpdatedAt,
		}
	}
	return items, nil
}

func (r *mongoPostRepo) ListWithDetails(ctx context.Context, offset, limit int, categoryID, authorID, status string) ([]model.PostListItem, error) {
	filter := bson.M{"deleted_at": nil}
	if categoryID != "" {
		filter["category_id"] = categoryID
	}
	if authorID != "" {
		filter["author_id"] = authorID
	}
	if status != "" {
		filter["status"] = status
	}
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

func (r *mongoPostRepo) FindByIDWithDetails(ctx context.Context, id string) (*model.PostListItem, error) {
	var p model.Post
	if err := r.store.Collection("posts").FindOne(ctx, bson.M{"_id": id, "deleted_at": nil}).Decode(&p); err != nil {
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
	filter := bson.M{
		"$and": bson.A{
			bson.M{"deleted_at": nil},
			bson.M{"$or": bson.A{
				bson.M{"title": bson.M{"$regex": escaped, "$options": "i"}},
				bson.M{"content_digest": bson.M{"$regex": escaped, "$options": "i"}},
			}},
		},
	}
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
	FindByID(ctx context.Context, id string) (*model.Category, error)
	Create(ctx context.Context, c *model.Category) error
	Update(ctx context.Context, c *model.Category) error
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

func (r *sqlCategoryRepo) FindByID(ctx context.Context, id string) (*model.Category, error) {
	var c model.Category
	err := r.db.DB().WithContext(ctx).First(&c, "id = ?", id).Error
	return &c, err
}

func (r *sqlCategoryRepo) Update(ctx context.Context, c *model.Category) error {
	return r.db.DB().WithContext(ctx).Save(c).Error
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

func (r *mongoCategoryRepo) FindByID(ctx context.Context, id string) (*model.Category, error) {
	var c model.Category
	err := r.store.Collection("categories").FindOne(ctx, bson.M{"_id": id}).Decode(&c)
	return &c, err
}

func (r *mongoCategoryRepo) Update(ctx context.Context, c *model.Category) error {
	_, err := r.store.Collection("categories").ReplaceOne(ctx, bson.M{"_id": c.ID}, c)
	return err
}

func (r *mongoCategoryRepo) Delete(ctx context.Context, id string) error {
	_, err := r.store.Collection("categories").DeleteOne(ctx, bson.M{"_id": id})
	return err
}

// ── Content ──────────────────────────────────────────────────────────────────

type ContentRepository interface {
	ListByRoot(ctx context.Context, rootID string, offset, limit int) ([]model.Content, error)
	ListByParent(ctx context.Context, parentID string, offset, limit int) ([]model.Content, error)
	FindByID(ctx context.Context, id string) (*model.Content, error)
	Create(ctx context.Context, c *model.Content) error
	Update(ctx context.Context, c *model.Content) error
	Delete(ctx context.Context, id string) error
}

func NewContentRepository(store db.Store) ContentRepository {
	if store.SQL() != nil {
		return &sqlContentRepo{db: store.SQL()}
	}
	return &mongoContentRepo{store: store.Mongo()}
}

type sqlContentRepo struct{ db db.SQLStore }

func (r *sqlContentRepo) ListByRoot(ctx context.Context, rootID string, offset, limit int) ([]model.Content, error) {
	var contents []model.Content
	err := r.db.DB().WithContext(ctx).Where("root_id = ? AND id != root_id", rootID).Order("created_at asc").Offset(offset).Limit(limit).Find(&contents).Error
	return contents, err
}

func (r *sqlContentRepo) ListByParent(ctx context.Context, parentID string, offset, limit int) ([]model.Content, error) {
	var contents []model.Content
	err := r.db.DB().WithContext(ctx).Where("parent_id = ?", parentID).Order("created_at asc").Offset(offset).Limit(limit).Find(&contents).Error
	return contents, err
}

func (r *sqlContentRepo) FindByID(ctx context.Context, id string) (*model.Content, error) {
	var c model.Content
	err := r.db.DB().WithContext(ctx).First(&c, "id = ?", id).Error
	return &c, err
}

func (r *sqlContentRepo) Create(ctx context.Context, c *model.Content) error {
	return r.db.DB().WithContext(ctx).Create(c).Error
}

func (r *sqlContentRepo) Update(ctx context.Context, c *model.Content) error {
	return r.db.DB().WithContext(ctx).Save(c).Error
}

func (r *sqlContentRepo) Delete(ctx context.Context, id string) error {
	return r.db.DB().WithContext(ctx).Delete(&model.Content{}, "id = ?", id).Error
}

type mongoContentRepo struct{ store db.MongoStore }

func (r *mongoContentRepo) ListByRoot(ctx context.Context, rootID string, offset, limit int) ([]model.Content, error) {
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}}).SetSkip(int64(offset)).SetLimit(int64(limit))
	cursor, err := r.store.Collection("contents").Find(ctx, bson.M{"root_id": rootID, "_id": bson.M{"$ne": rootID}, "deleted_at": nil}, opts)
	if err != nil {
		return nil, err
	}
	var contents []model.Content
	err = cursor.All(ctx, &contents)
	return contents, err
}

func (r *mongoContentRepo) ListByParent(ctx context.Context, parentID string, offset, limit int) ([]model.Content, error) {
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}}).SetSkip(int64(offset)).SetLimit(int64(limit))
	cursor, err := r.store.Collection("contents").Find(ctx, bson.M{"parent_id": parentID, "deleted_at": nil}, opts)
	if err != nil {
		return nil, err
	}
	var contents []model.Content
	err = cursor.All(ctx, &contents)
	return contents, err
}

func (r *mongoContentRepo) FindByID(ctx context.Context, id string) (*model.Content, error) {
	var c model.Content
	err := r.store.Collection("contents").FindOne(ctx, bson.M{"_id": id, "deleted_at": nil}).Decode(&c)
	return &c, err
}

func (r *mongoContentRepo) Create(ctx context.Context, c *model.Content) error {
	_, err := r.store.Collection("contents").InsertOne(ctx, c)
	return err
}

func (r *mongoContentRepo) Update(ctx context.Context, c *model.Content) error {
	_, err := r.store.Collection("contents").ReplaceOne(ctx, bson.M{"_id": c.ID}, c)
	return err
}

func (r *mongoContentRepo) Delete(ctx context.Context, id string) error {
	_, err := r.store.Collection("contents").UpdateOne(ctx,
		bson.M{"_id": id, "deleted_at": nil},
		bson.M{"$set": bson.M{"deleted_at": time.Now()}},
	)
	return err
}

// ── helpers ──────────────────────────────────────────────────────────────────

func DigestBody(body string, maxLen int) string {
	runes := []rune(body)
	if len(runes) > maxLen {
		return string(runes[:maxLen])
	}
	return body
}

// CreatePostWithContent is a helper to create post + content atomically.
func CreatePostWithContent(ctx context.Context, store db.Store, authorID, title, body, categoryID string) (*model.Post, *model.Content, error) {
	now := time.Now()
	contentID := uuid.NewString()

	content := &model.Content{
		ID:        contentID,
		AuthorID:  authorID,
		Body:      body,
		RootID:    contentID, // self-referencing for post body
		ParentID:  "",
		CreatedAt: now,
		UpdatedAt: now,
	}

	post := &model.Post{
		ID:            uuid.NewString(),
		Title:         title,
		ContentID:     contentID,
		ContentDigest: DigestBody(body, 200),
		AuthorID:      authorID,
		CategoryID:    categoryID,
		Status:        "approved",
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if store.SQL() != nil {
		return post, content, store.SQL().DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(content).Error; err != nil {
				return err
			}
			return tx.Create(post).Error
		})
	}

	// MongoDB
	if _, err := store.Mongo().Collection("contents").InsertOne(ctx, content); err != nil {
		return nil, nil, err
	}
	if _, err := store.Mongo().Collection("posts").InsertOne(ctx, post); err != nil {
		return nil, nil, err
	}
	return post, content, nil
}
