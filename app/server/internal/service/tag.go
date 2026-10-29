package service

import (
	"context"
	"errors"
	"feedchamber/internal/db"
	"feedchamber/internal/model"
	"feedchamber/internal/pkg"
	"feedchamber/internal/pkg/kvstore"
	"feedchamber/internal/pkg/readcache"
	"feedchamber/internal/repository"
	"time"
)

type tagService struct {
	repo  repository.TagRepository
	txMgr db.TransactionManager

	// readCache 缓存标签列表：它是侧栏数据，全站共用，变更极少。
	readCache *readcache.Cache
}

const (
	tagListCachePrefix = "cache:taglist:"
	tagListCacheTTL    = 60 * time.Second
)

func NewTagService(repo repository.TagRepository, txMgr db.TransactionManager, kv kvstore.Store) TagService {
	return &tagService{repo: repo, txMgr: txMgr, readCache: readcache.New(kv, tagListCacheTTL)}
}

func (s *tagService) List(ctx context.Context, page, pageSize int) ([]model.Tag, int, error) {
	key := readcache.Key(tagListCachePrefix, page, pageSize)
	var cached tagListPage
	if s.readCache.Get(ctx, "tag_list", key, &cached) {
		return cached.Items, cached.Total, nil
	}
	items, err := s.repo.List(ctx, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.repo.Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	s.readCache.Put(ctx, key, tagListPage{Items: items, Total: total})
	return items, total, nil
}

type tagListPage struct {
	Items []model.Tag `json:"items"`
	Total int         `json:"total"`
}

func (s *tagService) Create(ctx context.Context, name string) (*model.Tag, error) {
	s.readCache.InvalidatePrefix(ctx, tagListCachePrefix)
	tag := &model.Tag{
		ID:        pkg.NewID(),
		Name:      name,
		CreatedAt: time.Now(),
	}
	return tag, s.repo.Create(ctx, tag)
}

func (s *tagService) Update(ctx context.Context, id, name string) (*model.Tag, error) {
	s.readCache.InvalidatePrefix(ctx, tagListCachePrefix)
	tag, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, pkg.NewAppError(pkg.NOT_FOUND, "标签不存在")
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	tag.Name = name
	return tag, s.repo.Update(ctx, tag)
}

func (s *tagService) Delete(ctx context.Context, id string) error {
	s.readCache.InvalidatePrefix(ctx, tagListCachePrefix)
	return s.txMgr.WithTx(ctx, func(txCtx context.Context) error {
		return s.repo.Delete(txCtx, id)
	})
}

func (s *tagService) GetTagsByPostID(ctx context.Context, postID string) ([]model.Tag, error) {
	return s.repo.GetTagsByPostID(ctx, postID)
}

func (s *tagService) SetPostTags(ctx context.Context, postID string, tagNames []string) error {
	s.readCache.InvalidatePrefix(ctx, tagListCachePrefix)
	// 先收集新标签 ID，全部成功后再替换关联，避免中间失败导致帖子丢失所有标签
	type tagEntry struct {
		id   string
		name string
	}
	var entries []tagEntry
	for _, name := range tagNames {
		tag := &model.Tag{
			ID:        pkg.NewID(),
			Name:      name,
			CreatedAt: time.Now(),
		}
		if err := s.repo.CreateIfNotExists(ctx, tag); err != nil {
			return err
		}
		entries = append(entries, tagEntry{id: tag.ID, name: name})
	}

	// 所有标签创建成功，在事务内替换关联，保证原子性
	return s.txMgr.WithTx(ctx, func(txCtx context.Context) error {
		if err := s.repo.RemovePostTags(txCtx, postID); err != nil {
			return err
		}
		for _, e := range entries {
			if err := s.repo.AddPostTag(txCtx, postID, e.id); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *tagService) GetPostsByTagName(ctx context.Context, tagName string, page, pageSize int) ([]string, error) {
	tag, err := s.repo.GetByName(ctx, tagName)
	if err != nil {
		return nil, err
	}
	offset := (page - 1) * pageSize
	return s.repo.GetPostsByTagID(ctx, tag.ID, offset, pageSize)
}
