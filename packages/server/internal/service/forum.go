package service

import (
	"context"
	"errors"
	"orinforum/internal/db"
	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"orinforum/internal/repository"
	"time"

	"gorm.io/gorm"
)

type PostService interface {
	List(ctx context.Context, page, pageSize int, categoryName, tagName, authorID, status string, startTime, endTime *time.Time) ([]model.PostListItem, int, error)
	GetByID(ctx context.Context, id string) (*model.PostListItem, error)
	GetContent(ctx context.Context, contentID string) (*model.Content, error)
	Create(ctx context.Context, authorID, title, body, categoryID string) (*model.Post, error)
	Update(ctx context.Context, id, title, body string) (*model.Post, error)
	Delete(ctx context.Context, id string) error
	Search(ctx context.Context, keyword string, page, pageSize int) ([]model.PostListItem, int, error)
	TogglePin(ctx context.Context, id string) (*model.Post, error)
	ToggleLock(ctx context.Context, id string) (*model.Post, error)
}

type postService struct {
	posts    repository.PostRepository
	contents repository.ContentRepository
	store    db.Store
}

func NewPostService(posts repository.PostRepository, contents repository.ContentRepository, store db.Store) PostService {
	return &postService{posts: posts, contents: contents, store: store}
}

func (s *postService) List(ctx context.Context, page, pageSize int, categoryName, tagName, authorID, status string, startTime, endTime *time.Time) ([]model.PostListItem, int, error) {
	items, err := s.posts.ListWithDetails(ctx, (page-1)*pageSize, pageSize, categoryName, tagName, authorID, status, startTime, endTime)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.posts.CountWithDetails(ctx, categoryName, tagName, authorID, status, startTime, endTime)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *postService) GetByID(ctx context.Context, id string) (*model.PostListItem, error) {
	item, err := s.posts.FindByIDWithDetails(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, pkg.NewAppError(pkg.NOT_FOUND, "帖子不存在")
		}
		return nil, err
	}
	if item == nil || item.ID == "" {
		return nil, pkg.NewAppError(pkg.NOT_FOUND, "帖子不存在")
	}
	_ = s.posts.IncrementViewCount(ctx, id)
	return item, nil
}

func (s *postService) GetContent(ctx context.Context, contentID string) (*model.Content, error) {
	return s.contents.FindByID(ctx, contentID)
}

func (s *postService) Create(ctx context.Context, authorID, title, body, categoryID string) (*model.Post, error) {
	post, _, err := repository.CreatePostWithContent(ctx, s.store, authorID, title, body, categoryID)
	return post, err
}

func (s *postService) Update(ctx context.Context, id, title, body string) (*model.Post, error) {
	post, err := s.posts.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	post.Title = title
	post.ContentDigest = repository.DigestBody(body, 200)
	post.UpdatedAt = time.Now()

	content, err := s.contents.FindByID(ctx, post.ContentID)
	if err != nil {
		return nil, err
	}
	content.Body = body
	content.UpdatedAt = time.Now()

	if err := s.contents.Update(ctx, content); err != nil {
		return nil, err
	}
	return post, s.posts.Update(ctx, post)
}

func (s *postService) Delete(ctx context.Context, id string) error {
	return s.posts.Delete(ctx, id)
}

func (s *postService) Search(ctx context.Context, keyword string, page, pageSize int) ([]model.PostListItem, int, error) {
	items, err := s.posts.Search(ctx, keyword, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.posts.CountSearch(ctx, keyword)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *postService) TogglePin(ctx context.Context, id string) (*model.Post, error) {
	post, err := s.posts.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, pkg.NewAppError(pkg.NOT_FOUND, "帖子不存在")
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	post.Pinned = !post.Pinned
	post.UpdatedAt = time.Now()
	return post, s.posts.Update(ctx, post)
}

func (s *postService) ToggleLock(ctx context.Context, id string) (*model.Post, error) {
	post, err := s.posts.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, pkg.NewAppError(pkg.NOT_FOUND, "帖子不存在")
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	post.Locked = !post.Locked
	post.UpdatedAt = time.Now()
	return post, s.posts.Update(ctx, post)
}

type CategoryService interface {
	List(ctx context.Context, page, pageSize int) ([]model.Category, int, error)
	Create(ctx context.Context, name, description string) (*model.Category, error)
	Update(ctx context.Context, id, name, description string) (*model.Category, error)
	Delete(ctx context.Context, id string) error
}

type categoryService struct {
	repo repository.CategoryRepository
}

func NewCategoryService(repo repository.CategoryRepository) CategoryService {
	return &categoryService{repo: repo}
}

func (s *categoryService) List(ctx context.Context, page, pageSize int) ([]model.Category, int, error) {
	items, err := s.repo.List(ctx, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.repo.Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *categoryService) Create(ctx context.Context, name, description string) (*model.Category, error) {
	cat := &model.Category{
		ID:          pkg.NewID(),
		Name:        name,
		Description: description,
		CreatedAt:   time.Now(),
	}
	return cat, s.repo.Create(ctx, cat)
}

func (s *categoryService) Update(ctx context.Context, id, name, description string) (*model.Category, error) {
	cat, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, pkg.NewAppError(pkg.NOT_FOUND, "分类不存在")
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	cat.Name = name
	cat.Description = description
	return cat, s.repo.Update(ctx, cat)
}

func (s *categoryService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

type ContentService interface {
	ListByPost(ctx context.Context, postID string, page, pageSize int, startTime, endTime *time.Time) ([]model.ContentListItem, int, error)
	ListReplies(ctx context.Context, parentID string, page, pageSize int, startTime, endTime *time.Time) ([]model.ContentListItem, int, error)
	Create(ctx context.Context, postID, authorID, body, parentID string) (*model.Content, error)
	Update(ctx context.Context, postID, id, body string) (*model.Content, error)
	Delete(ctx context.Context, postID, id string) error
}

type contentService struct {
	repo     repository.ContentRepository
	postRepo repository.PostRepository
}

func NewContentService(repo repository.ContentRepository, postRepo repository.PostRepository) ContentService {
	return &contentService{repo: repo, postRepo: postRepo}
}

func (s *contentService) ListByPost(ctx context.Context, postID string, page, pageSize int, startTime, endTime *time.Time) ([]model.ContentListItem, int, error) {
	post, err := s.postRepo.FindByID(ctx, postID)
	if err != nil {
		return nil, 0, err
	}
	items, err := s.repo.ListByRoot(ctx, post.ContentID, (page-1)*pageSize, pageSize, startTime, endTime)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.repo.CountByRoot(ctx, post.ContentID, startTime, endTime)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *contentService) ListReplies(ctx context.Context, parentID string, page, pageSize int, startTime, endTime *time.Time) ([]model.ContentListItem, int, error) {
	items, err := s.repo.ListByParent(ctx, parentID, (page-1)*pageSize, pageSize, startTime, endTime)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.repo.CountByParent(ctx, parentID, startTime, endTime)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *contentService) Create(ctx context.Context, postID, authorID, body, parentID string) (*model.Content, error) {
	post, err := s.postRepo.FindByID(ctx, postID)
	if err != nil {
		return nil, err
	}

	c := &model.Content{
		ID:        pkg.NewID(),
		AuthorID:  authorID,
		Body:      body,
		RootID:    post.ContentID,
		ParentID:  parentID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := s.repo.Create(ctx, c); err != nil {
		return nil, err
	}
	_ = s.postRepo.IncrementCommentCount(ctx, postID)
	return c, nil
}

func (s *contentService) Update(ctx context.Context, postID, id, body string) (*model.Content, error) {
	c, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, pkg.NewAppError(pkg.NOT_FOUND, "评论不存在")
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	post, err := s.postRepo.FindByID(ctx, postID)
	if err != nil {
		return nil, err
	}
	if c.RootID != post.ContentID {
		return nil, pkg.NewAppError(pkg.NOT_FOUND, "该帖子下不存在此评论")
	}

	c.Body = body
	c.UpdatedAt = time.Now()
	return c, s.repo.Update(ctx, c)
}

func (s *contentService) Delete(ctx context.Context, postID, id string) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	return s.postRepo.DecrementCommentCount(ctx, postID)
}
