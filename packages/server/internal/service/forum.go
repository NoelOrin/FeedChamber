package service

import (
	"context"
	"orinforum/internal/model"
	"orinforum/internal/repository"
	"time"

	"github.com/google/uuid"
)

type PostService interface {
	List(ctx context.Context, page, pageSize int) ([]model.PostListItem, error)
	GetByID(ctx context.Context, id string) (*model.PostListItem, error)
	Create(ctx context.Context, authorID, title, content, categoryID string) (*model.Post, error)
	Update(ctx context.Context, id, title, content string) (*model.Post, error)
	Delete(ctx context.Context, id string) error
	Search(ctx context.Context, keyword string, page, pageSize int) ([]model.PostListItem, error)
}

type postService struct {
	posts repository.PostRepository
}

func NewPostService(posts repository.PostRepository) PostService {
	return &postService{posts: posts}
}

func (s *postService) List(ctx context.Context, page, pageSize int) ([]model.PostListItem, error) {
	return s.posts.ListWithDetails(ctx, (page-1)*pageSize, pageSize)
}

func (s *postService) GetByID(ctx context.Context, id string) (*model.PostListItem, error) {
	item, err := s.posts.FindByIDWithDetails(ctx, id)
	if err != nil {
		return nil, err
	}
	// Best-effort view count increment
	_ = s.posts.IncrementViewCount(ctx, id)
	return item, nil
}

func (s *postService) Create(ctx context.Context, authorID, title, content, categoryID string) (*model.Post, error) {
	post := &model.Post{
		ID:         uuid.NewString(),
		Title:      title,
		Content:    content,
		AuthorID:   authorID,
		CategoryID: categoryID,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	return post, s.posts.Create(ctx, post)
}

func (s *postService) Update(ctx context.Context, id, title, content string) (*model.Post, error) {
	post, err := s.posts.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	post.Title = title
	post.Content = content
	post.UpdatedAt = time.Now()
	return post, s.posts.Update(ctx, post)
}

func (s *postService) Delete(ctx context.Context, id string) error {
	return s.posts.Delete(ctx, id)
}

func (s *postService) Search(ctx context.Context, keyword string, page, pageSize int) ([]model.PostListItem, error) {
	return s.posts.Search(ctx, keyword, (page-1)*pageSize, pageSize)
}

type CategoryService interface {
	List(ctx context.Context) ([]model.Category, error)
	Create(ctx context.Context, name, description string) (*model.Category, error)
	Delete(ctx context.Context, id string) error
}

type categoryService struct {
	repo repository.CategoryRepository
}

func NewCategoryService(repo repository.CategoryRepository) CategoryService {
	return &categoryService{repo: repo}
}

func (s *categoryService) List(ctx context.Context) ([]model.Category, error) {
	return s.repo.List(ctx)
}

func (s *categoryService) Create(ctx context.Context, name, description string) (*model.Category, error) {
	cat := &model.Category{
		ID:          uuid.NewString(),
		Name:        name,
		Description: description,
		CreatedAt:   time.Now(),
	}
	return cat, s.repo.Create(ctx, cat)
}

func (s *categoryService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

type CommentService interface {
	ListByPost(ctx context.Context, postID string) ([]model.Comment, error)
	Create(ctx context.Context, postID, authorID, content, parentID string) (*model.Comment, error)
	Delete(ctx context.Context, id string) error
}

type commentService struct {
	repo repository.CommentRepository
}

func NewCommentService(repo repository.CommentRepository) CommentService {
	return &commentService{repo: repo}
}

func (s *commentService) ListByPost(ctx context.Context, postID string) ([]model.Comment, error) {
	return s.repo.ListByPost(ctx, postID)
}

func (s *commentService) Create(ctx context.Context, postID, authorID, content, parentID string) (*model.Comment, error) {
	c := &model.Comment{
		ID:        uuid.NewString(),
		PostID:    postID,
		AuthorID:  authorID,
		Content:   content,
		ParentID:  parentID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	return c, s.repo.Create(ctx, c)
}

func (s *commentService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}