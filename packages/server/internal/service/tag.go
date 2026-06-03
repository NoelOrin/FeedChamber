package service

import (
	"context"
	"errors"
	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"orinforum/internal/repository"
	"time"

	"gorm.io/gorm"
)

type TagService interface {
	List(ctx context.Context, page, pageSize int) ([]model.Tag, int, error)
	Create(ctx context.Context, name string) (*model.Tag, error)
	Update(ctx context.Context, id, name string) (*model.Tag, error)
	Delete(ctx context.Context, id string) error
	GetTagsByPostID(ctx context.Context, postID string) ([]model.Tag, error)
	SetPostTags(ctx context.Context, postID string, tagNames []string) error
	GetPostsByTagName(ctx context.Context, tagName string, page, pageSize int) ([]string, error)
}

type tagService struct {
	repo repository.TagRepository
}

func NewTagService(repo repository.TagRepository) TagService {
	return &tagService{repo: repo}
}

func (s *tagService) List(ctx context.Context, page, pageSize int) ([]model.Tag, int, error) {
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

func (s *tagService) Create(ctx context.Context, name string) (*model.Tag, error) {
	tag := &model.Tag{
		ID:        pkg.NewID(),
		Name:      name,
		CreatedAt: time.Now(),
	}
	return tag, s.repo.Create(ctx, tag)
}

func (s *tagService) Update(ctx context.Context, id, name string) (*model.Tag, error) {
	tag, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, pkg.NewAppError(pkg.NOT_FOUND, "标签不存在")
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	tag.Name = name
	return tag, s.repo.Update(ctx, tag)
}

func (s *tagService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func (s *tagService) GetTagsByPostID(ctx context.Context, postID string) ([]model.Tag, error) {
	return s.repo.GetTagsByPostID(ctx, postID)
}

func (s *tagService) SetPostTags(ctx context.Context, postID string, tagNames []string) error {
	if err := s.repo.RemovePostTags(ctx, postID); err != nil {
		return err
	}
	for _, name := range tagNames {
		tag := &model.Tag{
			ID:        pkg.NewID(),
			Name:      name,
			CreatedAt: time.Now(),
		}
		if err := s.repo.CreateIfNotExists(ctx, tag); err != nil {
			return err
		}
		if err := s.repo.AddPostTag(ctx, postID, tag.ID); err != nil {
			return err
		}
	}
	return nil
}

func (s *tagService) GetPostsByTagName(ctx context.Context, tagName string, page, pageSize int) ([]string, error) {
	tag, err := s.repo.GetByName(ctx, tagName)
	if err != nil {
		return nil, err
	}
	offset := (page - 1) * pageSize
	return s.repo.GetPostsByTagID(ctx, tag.ID, offset, pageSize)
}
