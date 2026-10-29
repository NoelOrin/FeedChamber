package service

import (
	"context"
	"errors"
	"fmt"
	"orinforum/internal/db"
	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"orinforum/internal/pkg/kvstore"
	"orinforum/internal/pkg/readcache"
	"orinforum/internal/repository"
	"time"
)

const maxSpaceDepth = 20

type spaceService struct {
	repo      repository.SpaceRepository
	postRepo  repository.PostRepository
	txMgr     db.TransactionManager
	permCache SpacePermissionCacheUpdater

	// readCache 缓存树/扁平列表：导航类数据，全站共用，变更少。
	// 它与帖子列表缓存同属"侧栏热点"，失效时一并清掉 postlist 前缀，
	// 因为列表过滤依赖版面层级（includeSub 会展开后代 ID）。
	readCache *readcache.Cache
}

const (
	spaceTreeCachePrefix = "cache:spacetree:"
	spaceFlatCachePrefix = "cache:spaceflat:"
	spaceCacheTTL        = 60 * time.Second
)

func NewSpaceService(repo repository.SpaceRepository, postRepo repository.PostRepository, txMgr db.TransactionManager, kv kvstore.Store) SpaceService {
	return &spaceService{repo: repo, postRepo: postRepo, txMgr: txMgr, readCache: readcache.New(kv, spaceCacheTTL)}
}

// invalidateSpaceCaches 清空版面侧缓存；版面树变了，带 includeSub 的帖子列表
// 过滤结果也跟着变，所以一并清掉帖子列表缓存。
func (s *spaceService) invalidateSpaceCaches(ctx context.Context) {
	if s.readCache == nil {
		return
	}
	s.readCache.InvalidatePrefix(ctx, spaceTreeCachePrefix)
	s.readCache.InvalidatePrefix(ctx, spaceFlatCachePrefix)
	s.readCache.InvalidatePrefix(ctx, "cache:postlist:")
}

func (s *spaceService) SetPermissionCacheUpdater(updater SpacePermissionCacheUpdater) {
	s.permCache = updater
}

func (s *spaceService) Tree(ctx context.Context) ([]*model.SpaceNode, error) {
	key := readcache.Key(spaceTreeCachePrefix)
	var cached []*model.SpaceNode
	if s.readCache.Get(ctx, "space_tree", key, &cached) {
		return cached, nil
	}
	spaces, err := s.repo.List(ctx)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	tree := model.BuildSpaceTree(spaces)
	s.readCache.Put(ctx, key, tree)
	return tree, nil
}

func (s *spaceService) Flat(ctx context.Context) ([]model.Space, error) {
	key := readcache.Key(spaceFlatCachePrefix)
	var cached []model.Space
	if s.readCache.Get(ctx, "space_flat", key, &cached) {
		return cached, nil
	}
	spaces, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	s.readCache.Put(ctx, key, spaces)
	return spaces, nil
}

func (s *spaceService) GetByID(ctx context.Context, id string) (*model.Space, error) {
	space, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, pkg.NewAppError(pkg.NOT_FOUND, "版面不存在")
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return space, nil
}

func (s *spaceService) GetBySlug(ctx context.Context, slug string) (*model.Space, error) {
	space, err := s.repo.FindBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, pkg.NewAppError(pkg.NOT_FOUND, "版面不存在")
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return space, nil
}

func (s *spaceService) Create(ctx context.Context, name, description, icon, slug, parentID string, sortOrder int, visibility string, minLevel int) (*model.Space, error) {
	s.invalidateSpaceCaches(ctx)
	if name == "" || slug == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "名称和标识不能为空")
	}
	if err := validateSpaceVisibility(visibility, minLevel); err != nil {
		return nil, err
	}
	var depth int
	var path string
	newID := pkg.NewID()
	if parentID != "" {
		parent, err := s.repo.FindByID(ctx, parentID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return nil, pkg.NewAppError(pkg.NOT_FOUND, "父版面不存在")
			}
			return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
		ancestors, err := s.repo.FindAncestors(ctx, parentID)
		if err != nil {
			return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
		depth = len(ancestors) + 1
		if depth > maxSpaceDepth {
			return nil, pkg.NewAppError(pkg.INVALID_PARAMS, fmt.Sprintf("层级过深，最多支持 %d 层", maxSpaceDepth))
		}
		if parent.Path != "" {
			path = parent.Path + "/" + newID
		} else {
			ids := make([]string, 0, len(ancestors)+1)
			for _, a := range ancestors {
				ids = append(ids, a.ID)
			}
			ids = append(ids, parent.ID)
			path = model.BuildPath(ids, newID)
		}
	} else {
		depth = 0
		path = "/" + newID
	}
	if sortOrder == 0 {
		if cnt, _ := s.repo.CountChildren(ctx, parentID); cnt > 0 {
			sortOrder = cnt
		}
	}
	space := &model.Space{
		ID:          newID,
		Name:        name,
		Description: description,
		Icon:        icon,
		Slug:        slug,
		ParentID:    parentID,
		Depth:       depth,
		Path:        path,
		SortOrder:   sortOrder,
		IsActive:    true,
		Visibility:  normalizeSpaceVisibility(visibility),
		MinLevel:    minLevel,
	}
	if err := s.repo.Create(ctx, space); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if s.permCache != nil {
		s.permCache.UpdateSpaceParent(space.ID, space.ParentID)
	}
	return space, nil
}

func (s *spaceService) Update(ctx context.Context, id, name, description, icon, slug, parentID string, sortOrder int, isActive bool, visibility string, minLevel int) (*model.Space, error) {
	s.invalidateSpaceCaches(ctx)
	if err := validateSpaceVisibility(visibility, minLevel); err != nil {
		return nil, err
	}
	space, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, pkg.NewAppError(pkg.NOT_FOUND, "版面不存在")
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	parentChanged := parentID != space.ParentID
	oldPath := space.Path
	oldDepth := space.Depth
	if parentChanged {
		if parentID == id {
			return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "不能将版面移动到自身下")
		}
		if parentID != "" {
			if _, err := s.repo.FindByID(ctx, parentID); err != nil {
				if errors.Is(err, repository.ErrNotFound) {
					return nil, pkg.NewAppError(pkg.NOT_FOUND, "父版面不存在")
				}
				return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
			}
			descIDs, err := s.repo.FindDescendantIDs(ctx, id)
			if err != nil {
				return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
			}
			for _, did := range descIDs {
				if did == parentID {
					return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "不能将版面移动到其子版面下，会形成循环")
				}
			}
			ancestors, err := s.repo.FindAncestors(ctx, parentID)
			if err != nil {
				return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
			}
			newDepth := len(ancestors) + 1
			if newDepth > maxSpaceDepth {
				return nil, pkg.NewAppError(pkg.INVALID_PARAMS, fmt.Sprintf("层级过深，最多支持 %d 层", maxSpaceDepth))
			}
			descendants, err := s.repo.FindDescendants(ctx, id)
			if err != nil {
				return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
			}
			maxDescDepth := oldDepth
			for _, d := range descendants {
				if d.Depth > maxDescDepth {
					maxDescDepth = d.Depth
				}
			}
			if maxDescDepth+(newDepth-oldDepth) > maxSpaceDepth {
				return nil, pkg.NewAppError(pkg.INVALID_PARAMS, fmt.Sprintf("移动后子版面层级过深，最多支持 %d 层", maxSpaceDepth))
			}
		} else {
			descendants, err := s.repo.FindDescendants(ctx, id)
			if err != nil {
				return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
			}
			maxDescDepth := oldDepth
			for _, d := range descendants {
				if d.Depth > maxDescDepth {
					maxDescDepth = d.Depth
				}
			}
			if maxDescDepth-oldDepth > maxSpaceDepth {
				return nil, pkg.NewAppError(pkg.INVALID_PARAMS, fmt.Sprintf("移动后子版面层级过深，最多支持 %d 层", maxSpaceDepth))
			}
		}
	}
	if name != "" {
		space.Name = name
	}
	if slug != "" {
		space.Slug = slug
	}
	space.Description = description
	space.Icon = icon
	if parentChanged {
		space.ParentID = parentID
	}
	space.SortOrder = sortOrder
	space.IsActive = isActive
	if visibility != "" {
		space.Visibility = visibility
	}
	space.MinLevel = minLevel
	if space.Visibility == "" {
		space.Visibility = model.SpaceVisibilityPublic
	}
	if parentChanged {
		var newDepth int
		var newPath string
		if parentID == "" {
			newDepth = 0
			newPath = "/" + space.ID
		} else {
			parent, _ := s.repo.FindByID(ctx, parentID)
			ancestors, _ := s.repo.FindAncestors(ctx, parentID)
			newDepth = len(ancestors) + 1
			if parent != nil && parent.Path != "" {
				newPath = parent.Path + "/" + space.ID
			} else {
				ids := make([]string, 0, len(ancestors)+1)
				for _, a := range ancestors {
					ids = append(ids, a.ID)
				}
				if parent != nil {
					ids = append(ids, parent.ID)
				}
				newPath = model.BuildPath(ids, space.ID)
			}
		}
		space.Depth = newDepth
		space.Path = newPath
		descendants, err := s.repo.FindDescendants(ctx, id)
		if err != nil {
			return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
		updates := make(map[string]struct {
			Depth int
			Path  string
		})
		if oldPath == "" {
			ancOld, _ := s.repo.FindAncestors(ctx, id)
			ids := make([]string, 0, len(ancOld))
			for _, a := range ancOld {
				ids = append(ids, a.ID)
			}
			oldPath = model.BuildPath(ids, id)
		}
		for _, d := range descendants {
			oldDescPath := d.Path
			if oldDescPath == "" {
				anc, _ := s.repo.FindAncestors(ctx, d.ID)
				ids := make([]string, 0, len(anc))
				for _, a := range anc {
					ids = append(ids, a.ID)
				}
				oldDescPath = model.BuildPath(ids, d.ID)
				if oldDescPath == "" {
					oldDescPath = "/" + d.ID
				}
			}
			suffix := ""
			if len(oldDescPath) > len(oldPath) && oldDescPath[:len(oldPath)] == oldPath {
				suffix = oldDescPath[len(oldPath):]
			} else {
				suffix = "/" + d.ID
			}
			newDescPath := newPath + suffix
			newDescDepth := newDepth + (d.Depth - oldDepth)
			if oldDepth == 0 && d.Depth == 0 && d.ParentID != "" {
				newSlashes := 0
				for _, c := range newDescPath {
					if c == '/' {
						newSlashes++
					}
				}
				newDescDepth = newSlashes - 1
			}
			updates[d.ID] = struct {
				Depth int
				Path  string
			}{Depth: newDescDepth, Path: newDescPath}
		}
		err = s.txMgr.WithTx(ctx, func(txCtx context.Context) error {
			if err := s.repo.Update(txCtx, space); err != nil {
				return err
			}
			if len(updates) > 0 {
				if err := s.repo.BatchUpdateDepthPath(txCtx, updates); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
		if s.permCache != nil {
			s.permCache.UpdateSpaceParent(space.ID, space.ParentID)
		}
		return space, nil
	}
	if err := s.repo.Update(ctx, space); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return space, nil
}

func (s *spaceService) Delete(ctx context.Context, id string) error {
	s.invalidateSpaceCaches(ctx)
	err := s.txMgr.WithTx(ctx, func(txCtx context.Context) error {
		if _, err := s.repo.FindByID(txCtx, id); err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return pkg.NewAppError(pkg.NOT_FOUND, "版面不存在")
			}
			return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
		hasChildren, err := s.repo.HasChildren(txCtx, id)
		if err != nil {
			return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
		if hasChildren {
			return pkg.NewAppError(pkg.INVALID_PARAMS, "该版面下还有子版面，请先删除或移动子版面")
		}
		count, err := s.postRepo.CountBySpaceID(txCtx, id)
		if err != nil {
			return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
		if count > 0 {
			return pkg.NewAppError(pkg.INVALID_PARAMS, "该版面下还有帖子，无法删除")
		}
		return s.repo.Delete(txCtx, id)
	})
	if err == nil && s.permCache != nil {
		s.permCache.RemoveSpace(id)
	}
	return err
}

func (s *spaceService) ToggleActive(ctx context.Context, id string) (*model.Space, error) {
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, pkg.NewAppError(pkg.NOT_FOUND, "版面不存在")
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return s.repo.ToggleActive(ctx, id)
}

// VisibilityIndex 构建版面有效可见性索引（含祖先继承），供内容出口与详情
// 门控共用；这是「版面可见性」判定的唯一来源。
func (s *spaceService) VisibilityIndex(ctx context.Context) (VisibilityIndex, error) {
	spaces, err := s.repo.List(ctx)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return BuildVisibilityIndex(spaces), nil
}

func (s *spaceService) GetChildren(ctx context.Context, parentID string) ([]model.Space, error) {
	if parentID == "" {
		return s.repo.ListRoots(ctx)
	}
	if _, err := s.repo.FindByID(ctx, parentID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, pkg.NewAppError(pkg.NOT_FOUND, "父版面不存在")
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return s.repo.FindChildren(ctx, parentID)
}

func (s *spaceService) GetAncestors(ctx context.Context, id string) ([]model.Space, error) {
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, pkg.NewAppError(pkg.NOT_FOUND, "版面不存在")
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return s.repo.FindAncestors(ctx, id)
}

func (s *spaceService) GetBreadcrumb(ctx context.Context, id string) ([]model.Space, error) {
	space, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, pkg.NewAppError(pkg.NOT_FOUND, "版面不存在")
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	ancestors, err := s.repo.FindAncestors(ctx, id)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return append(ancestors, *space), nil
}

func (s *spaceService) GetDescendants(ctx context.Context, id string) ([]model.Space, error) {
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, pkg.NewAppError(pkg.NOT_FOUND, "版面不存在")
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return s.repo.FindDescendants(ctx, id)
}

func (s *spaceService) GetDescendantIDs(ctx context.Context, id string) ([]string, error) {
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, pkg.NewAppError(pkg.NOT_FOUND, "版面不存在")
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return s.repo.FindDescendantIDs(ctx, id)
}

func (s *spaceService) GetSubTree(ctx context.Context, id string) (*model.SpaceNode, error) {
	space, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, pkg.NewAppError(pkg.NOT_FOUND, "版面不存在")
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	all, err := s.repo.List(ctx)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	forest := model.BuildSpaceTree(all)
	var find func(nodes []*model.SpaceNode) *model.SpaceNode
	find = func(nodes []*model.SpaceNode) *model.SpaceNode {
		for _, n := range nodes {
			if n.ID == id {
				return n
			}
			if len(n.Children) > 0 {
				if found := find(n.Children); found != nil {
					return found
				}
			}
		}
		return nil
	}
	node := find(forest)
	if node == nil {
		node = &model.SpaceNode{Space: *space, Level: space.Depth, HasChildren: false}
	}
	return node, nil
}

func (s *spaceService) Move(ctx context.Context, id, newParentID string, sortOrder *int) (*model.Space, error) {
	space, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, pkg.NewAppError(pkg.NOT_FOUND, "版面不存在")
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if newParentID == id {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "不能将版面移动到自身下")
	}
	if newParentID != "" {
		if _, err := s.repo.FindByID(ctx, newParentID); err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return nil, pkg.NewAppError(pkg.NOT_FOUND, "目标父版面不存在")
			}
			return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
		descIDs, err := s.repo.FindDescendantIDs(ctx, id)
		if err != nil {
			return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
		for _, did := range descIDs {
			if did == newParentID {
				return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "不能将版面移动到其子孙版面下，会形成循环")
			}
		}
	}
	newSort := space.SortOrder
	if sortOrder != nil {
		newSort = *sortOrder
	} else if newParentID != space.ParentID {
		if cnt, _ := s.repo.CountChildren(ctx, newParentID); cnt >= 0 {
			newSort = cnt
		}
	}
	return s.Update(ctx, id, space.Name, space.Description, space.Icon, space.Slug, newParentID, newSort, space.IsActive, space.Visibility, space.MinLevel)
}

func (s *spaceService) Reorder(ctx context.Context, parentID string, orderedIDs []string) error {
	if len(orderedIDs) == 0 {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "排序列表不能为空")
	}
	spaces, err := s.repo.List(ctx)
	if err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	byID := make(map[string]model.Space, len(spaces))
	for _, sp := range spaces {
		byID[sp.ID] = sp
	}
	for _, oid := range orderedIDs {
		sp, ok := byID[oid]
		if !ok {
			return pkg.NewAppError(pkg.NOT_FOUND, fmt.Sprintf("版面 %s 不存在", oid))
		}
		if sp.ParentID != parentID {
			return pkg.NewAppError(pkg.INVALID_PARAMS, fmt.Sprintf("版面 %s 的父级不匹配", oid))
		}
	}
	return s.repo.BatchUpdateSortOrder(ctx, orderedIDs)
}

func (s *spaceService) ListRoots(ctx context.Context) ([]model.Space, error) {
	return s.repo.ListRoots(ctx)
}

func validateSpaceVisibility(visibility string, minLevel int) error {
	switch visibility {
	case "", model.SpaceVisibilityPublic, model.SpaceVisibilityLogin, model.SpaceVisibilityLevel:
	default:
		return pkg.NewAppError(pkg.INVALID_PARAMS, "可见性必须为 public、login 或 level")
	}
	if minLevel < 0 {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "最低等级不能为负数")
	}
	return nil
}

func normalizeSpaceVisibility(visibility string) string {
	if visibility == "" {
		return model.SpaceVisibilityPublic
	}
	return visibility
}
