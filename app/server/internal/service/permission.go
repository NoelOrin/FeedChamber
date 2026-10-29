package service

import (
	"context"
	"sync"

	"orinforum/internal/metrics"
	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"orinforum/internal/pkg/kvstore"
	"orinforum/internal/repository"
)

const (
	permSyncChannel  = "perm:sync"
	maxHierarchyWalk = 32
)

// groupMembershipReader is the minimal group repo surface used by PermissionService.
type groupMembershipReader interface {
	ListGroupIDsByUser(ctx context.Context, userID string) ([]string, error)
	GetAllGroupPermissions(ctx context.Context) (map[string][]string, error)
}

// SpacePermissionCacheUpdater allows SpaceService to keep PermissionService's hierarchy cache in sync.
type SpacePermissionCacheUpdater interface {
	UpdateSpaceParent(spaceID, parentID string)
	RemoveSpace(spaceID string)
}

type permissionService struct {
	permRepo   repository.PermissionRepository
	spaceRepo  repository.SpaceRolePermissionRepository
	groupRepo  groupMembershipReader
	kv         kvstore.Store
	cache      map[string]map[string]struct{}            // role -> set of perm codes
	spaceCache map[string]map[string]map[string]struct{} // spaceID -> role -> set of perm codes
	groupCache map[string]map[string]struct{}            // groupID -> set of perm codes
	cacheMu    sync.RWMutex

	// NGA hierarchical: spaceID -> parentID for inheritance walk.
	spaceParents   map[string]string
	spaceParentsMu sync.RWMutex
	// optional SpaceRepository for RefreshSpaceParents; set via SetSpaceRepository.
	spaceHierRepo repository.SpaceRepository
}

func NewPermissionService(permRepo repository.PermissionRepository, spaceRepo repository.SpaceRolePermissionRepository, groupRepo repository.GroupRepository, kv kvstore.Store) PermissionService {
	var membership groupMembershipReader
	if groupRepo != nil {
		membership = groupRepo
	}
	return &permissionService{
		permRepo:     permRepo,
		spaceRepo:    spaceRepo,
		groupRepo:    membership,
		kv:           kv,
		cache:        make(map[string]map[string]struct{}),
		spaceCache:   make(map[string]map[string]map[string]struct{}),
		groupCache:   make(map[string]map[string]struct{}),
		spaceParents: make(map[string]string),
	}
}

// SetSpaceRepository injects SpaceRepository for hierarchy loading (call after New).
func (s *permissionService) SetSpaceRepository(repo repository.SpaceRepository) {
	s.spaceParentsMu.Lock()
	defer s.spaceParentsMu.Unlock()
	s.spaceHierRepo = repo
}

// RefreshSpaceParents rebuilds the space parent map from DB (hierarchical inheritance).
func (s *permissionService) RefreshSpaceParents(ctx context.Context) error {
	if s.spaceHierRepo == nil {
		return nil
	}
	spaces, err := s.spaceHierRepo.List(ctx)
	if err != nil {
		return err
	}
	m := make(map[string]string, len(spaces))
	for _, sp := range spaces {
		m[sp.ID] = sp.ParentID
	}
	s.spaceParentsMu.Lock()
	s.spaceParents = m
	s.spaceParentsMu.Unlock()
	return nil
}

// UpdateSpaceParent keeps the hierarchy cache in sync after Create/Update/Move.
func (s *permissionService) UpdateSpaceParent(spaceID, parentID string) {
	s.spaceParentsMu.Lock()
	if s.spaceParents == nil {
		s.spaceParents = make(map[string]string)
	}
	s.spaceParents[spaceID] = parentID
	s.spaceParentsMu.Unlock()
	// Publish for multi-instance sync (hierarchy)
	if s.kv != nil {
		_ = s.kv.Publish(context.Background(), permSyncChannel, []byte("hierarchy:"+spaceID))
	}
}

// RemoveSpace removes a space from the hierarchy cache after Delete.
func (s *permissionService) RemoveSpace(spaceID string) {
	s.spaceParentsMu.Lock()
	delete(s.spaceParents, spaceID)
	s.spaceParentsMu.Unlock()
	if s.kv != nil {
		_ = s.kv.Publish(context.Background(), permSyncChannel, []byte("hierarchy:"+spaceID))
	}
}

// LoadCache 启动时从 DB 加载全局、版块与组权限到内存。
func (s *permissionService) LoadCache(ctx context.Context) error {
	rolePerms, err := s.permRepo.GetAllRolePermissions(ctx)
	if err != nil {
		return err
	}
	spacePerms, err := s.spaceRepo.GetAllSpaceRolePermissions(ctx)
	if err != nil {
		return err
	}

	var groupPerms map[string][]string
	if s.groupRepo != nil {
		groupPerms, err = s.groupRepo.GetAllGroupPermissions(ctx)
		if err != nil {
			return err
		}
	}

	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()

	s.cache = make(map[string]map[string]struct{}, len(rolePerms))
	for role, codes := range rolePerms {
		set := make(map[string]struct{}, len(codes))
		for _, c := range codes {
			set[c] = struct{}{}
		}
		s.cache[role] = set
	}

	s.spaceCache = make(map[string]map[string]map[string]struct{}, len(spacePerms))
	for spaceID, roleMap := range spacePerms {
		s.spaceCache[spaceID] = make(map[string]map[string]struct{}, len(roleMap))
		for role, codes := range roleMap {
			set := make(map[string]struct{}, len(codes))
			for _, c := range codes {
				set[c] = struct{}{}
			}
			s.spaceCache[spaceID][role] = set
		}
	}

	s.groupCache = make(map[string]map[string]struct{}, len(groupPerms))
	for groupID, codes := range groupPerms {
		set := make(map[string]struct{}, len(codes))
		for _, c := range codes {
			set[c] = struct{}{}
		}
		s.groupCache[groupID] = set
	}

	// Report sizes while the write lock is still held, so the gauges always
	// describe a fully rebuilt cache rather than a half-populated map.
	metrics.ObservePermissionCacheSize(metrics.PermissionCacheRole, len(s.cache))
	metrics.ObservePermissionCacheSize(metrics.PermissionCacheSpace, len(s.spaceCache))
	metrics.ObservePermissionCacheSize(metrics.PermissionCacheGroup, len(s.groupCache))
	return nil
}

// ReloadGroupCache 仅重载组权限缓存。
func (s *permissionService) ReloadGroupCache(ctx context.Context) error {
	if s.groupRepo == nil {
		return nil
	}
	groupPerms, err := s.groupRepo.GetAllGroupPermissions(ctx)
	if err != nil {
		return err
	}
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	s.groupCache = make(map[string]map[string]struct{}, len(groupPerms))
	for groupID, codes := range groupPerms {
		set := make(map[string]struct{}, len(codes))
		for _, c := range codes {
			set[c] = struct{}{}
		}
		s.groupCache[groupID] = set
	}
	return nil
}

// StartSyncSubscriber 订阅 perm:sync 频道，收到消息后重新加载全部缓存。
func (s *permissionService) StartSyncSubscriber(ctx context.Context) error {
	ch, err := s.kv.Subscribe(ctx, permSyncChannel)
	if err != nil {
		return err
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-ch:
				if !ok {
					return
				}
				_ = s.LoadCache(context.Background())
				// Also refresh hierarchy if needed
				_ = s.RefreshSpaceParents(context.Background())
			}
		}
	}()
	return nil
}

// HasPermission 检查角色是否拥有指定全局权限码。
// 自服务权限 (*:self, content:like 等) 对任意已认证的非 ban 角色隐式放行，
// 避免角色权限被不完整同步覆盖后导致登出/改密等基础能力不可用。
func (s *permissionService) HasPermission(roleName, permCode string) (allowed bool) {
	defer func() { metrics.ObservePermission("global", allowed) }()

	if isSelfServicePerm(permCode) {
		if roleName != "" && roleName != "ban" {
			return true
		}
	}
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()
	return s.hasGlobalLocked(roleName, permCode)
}

// isSelfServicePerm 判断是否为任意已认证用户均应拥有的自服务权限。
func isSelfServicePerm(permCode string) bool {
	// 后缀 :self 通配
	if len(permCode) > 5 && permCode[len(permCode)-5:] == ":self" {
		return true
	}
	// 非 :self 但仍为自服务的权限码（与 model 中 Perm*Self 定义保持一致）
	switch permCode {
	case "content:like", "follow:read", "security:read", "security:report", "dashboard:read":
		return true
	}
	return false
}

func (s *permissionService) hasGlobalLocked(roleName, permCode string) bool {
	perms, ok := s.cache[roleName]
	if !ok {
		return false
	}
	_, found := perms[permCode]
	return found
}

// HasPermissionInSpace 覆盖式解析 + NGA 层级继承：
//  1. 从当前版面向上遍历祖先链，找到最近的对该角色的覆盖（若存在则完全替换全局）
//  2. 若整条链均无覆盖，回退全局
//
// 锁序：先快照 spaceParents，再读 spaceCache，避免嵌套 RLock 死锁风险。
func (s *permissionService) HasPermissionInSpace(roleName, spaceID, permCode string) (allowed bool) {
	defer func() { metrics.ObservePermission("space", allowed) }()

	if isSelfServicePerm(permCode) {
		if roleName != "" && roleName != "ban" {
			return true
		}
	}
	// snapshot parents to avoid nested lock
	s.spaceParentsMu.RLock()
	parents := make(map[string]string, len(s.spaceParents))
	for k, v := range s.spaceParents {
		parents[k] = v
	}
	s.spaceParentsMu.RUnlock()

	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()
	if spaceID != "" {
		cur := spaceID
		visited := make(map[string]struct{}, 8)
		for cur != "" {
			if _, seen := visited[cur]; seen {
				break // cycle guard
			}
			visited[cur] = struct{}{}
			if roleMap, ok := s.spaceCache[cur]; ok {
				if permSet, ok := roleMap[roleName]; ok {
					_, found := permSet[permCode]
					return found
				}
			}
			parent, ok := parents[cur]
			if !ok {
				break
			}
			cur = parent
			if len(visited) > maxHierarchyWalk {
				break
			}
		}
	}
	return s.hasGlobalLocked(roleName, permCode)
}

// HasPermissionForUser 聚合 = role perms ∪ 用户 groups perms。
func (s *permissionService) HasPermissionForUser(ctx context.Context, userID, roleName, permCode string) bool {
	if s.HasPermission(roleName, permCode) {
		return true
	}
	if userID == "" || s.groupRepo == nil {
		return false
	}
	groupIDs, err := s.groupRepo.ListGroupIDsByUser(ctx, userID)
	if err != nil || len(groupIDs) == 0 {
		return false
	}
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()
	for _, gid := range groupIDs {
		if perms, ok := s.groupCache[gid]; ok {
			if _, found := perms[permCode]; found {
				return true
			}
		}
	}
	return false
}

// HasPermissionInSpaceForUser 先检查角色版块覆盖/全局（层级继承），再并上组权限（组权限目前为全局）。
func (s *permissionService) HasPermissionInSpaceForUser(ctx context.Context, userID, roleName, spaceID, permCode string) bool {
	if s.HasPermissionInSpace(roleName, spaceID, permCode) {
		return true
	}
	// 组权限作为额外授予（union），不参与 space override 替换语义
	return s.HasPermissionForUser(ctx, userID, "", permCode)
}

func (s *permissionService) GetRolePermissions(roleName string) []string {
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()
	perms, ok := s.cache[roleName]
	if !ok {
		return nil
	}
	codes := make([]string, 0, len(perms))
	for c := range perms {
		codes = append(codes, c)
	}
	return codes
}

func (s *permissionService) ListPermissions(ctx context.Context) ([]model.Permission, error) {
	return s.permRepo.List(ctx)
}

func (s *permissionService) SyncRolePermissions(ctx context.Context, roleName string, permCodes []string) error {
	if err := s.permRepo.SyncRolePermissions(ctx, roleName, permCodes); err != nil {
		return err
	}
	s.cacheMu.Lock()
	set := make(map[string]struct{}, len(permCodes))
	for _, c := range permCodes {
		set[c] = struct{}{}
	}
	s.cache[roleName] = set
	s.cacheMu.Unlock()
	_ = s.kv.Publish(ctx, permSyncChannel, []byte(roleName))
	return nil
}

func (s *permissionService) GetSpaceRolePermissions(ctx context.Context, spaceID, roleName string) ([]string, bool, error) {
	return s.spaceRepo.GetSpaceRolePermissions(ctx, spaceID, roleName)
}

// GetEffectiveSpaceRolePermissions 返回层级继承后的有效权限：
//   - 若当前或祖先有覆盖，返回最近覆盖的权限及来源版面
//   - 否则返回全局权限（hasOverride=false）
func (s *permissionService) GetEffectiveSpaceRolePermissions(ctx context.Context, spaceID, roleName string) ([]string, string, bool, bool, error) {
	s.spaceParentsMu.RLock()
	parents := make(map[string]string, len(s.spaceParents))
	for k, v := range s.spaceParents {
		parents[k] = v
	}
	s.spaceParentsMu.RUnlock()

	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()
	cur := spaceID
	visited := make(map[string]struct{}, 8)
	for cur != "" {
		if _, seen := visited[cur]; seen {
			break
		}
		visited[cur] = struct{}{}
		if roleMap, ok := s.spaceCache[cur]; ok {
			if permSet, ok := roleMap[roleName]; ok {
				codes := make([]string, 0, len(permSet))
				for c := range permSet {
					codes = append(codes, c)
				}
				inherited := cur != spaceID
				return codes, cur, true, inherited, nil
			}
		}
		parent, ok := parents[cur]
		if !ok {
			break
		}
		cur = parent
		if len(visited) > maxHierarchyWalk {
			break
		}
	}
	// fallback global
	if perms, ok := s.cache[roleName]; ok {
		codes := make([]string, 0, len(perms))
		for c := range perms {
			codes = append(codes, c)
		}
		return codes, "", false, false, nil
	}
	return []string{}, "", false, false, nil
}

func (s *permissionService) SyncSpaceRolePermissions(ctx context.Context, spaceID, roleName string, permCodes []string) error {
	if len(permCodes) == 0 {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "版块覆盖权限不能为空；如需回退全局请清除覆盖")
	}
	if err := s.spaceRepo.SyncSpaceRolePermissions(ctx, spaceID, roleName, permCodes); err != nil {
		return err
	}
	s.cacheMu.Lock()
	if s.spaceCache[spaceID] == nil {
		s.spaceCache[spaceID] = make(map[string]map[string]struct{})
	}
	set := make(map[string]struct{}, len(permCodes))
	for _, c := range permCodes {
		set[c] = struct{}{}
	}
	s.spaceCache[spaceID][roleName] = set
	s.cacheMu.Unlock()
	_ = s.kv.Publish(ctx, permSyncChannel, []byte("space:"+spaceID))
	return nil
}

func (s *permissionService) ClearSpaceRoleOverride(ctx context.Context, spaceID, roleName string) error {
	if err := s.spaceRepo.ClearSpaceRolePermissions(ctx, spaceID, roleName); err != nil {
		return err
	}
	s.cacheMu.Lock()
	if roleMap, ok := s.spaceCache[spaceID]; ok {
		delete(roleMap, roleName)
		if len(roleMap) == 0 {
			delete(s.spaceCache, spaceID)
		}
	}
	s.cacheMu.Unlock()
	_ = s.kv.Publish(ctx, permSyncChannel, []byte("space:"+spaceID))
	return nil
}

// ListSpacesWithEffectivePermissions returns a map spaceID -> effective codes for a role (for tree UI).
func (s *permissionService) ListSpacesWithEffectivePermissions(ctx context.Context, roleName string, spaceIDs []string) (map[string][]string, error) {
	result := make(map[string][]string, len(spaceIDs))
	for _, sid := range spaceIDs {
		codes, _, _, _, err := s.GetEffectiveSpaceRolePermissions(ctx, sid, roleName)
		if err != nil {
			return nil, err
		}
		if codes == nil {
			codes = []string{}
		}
		result[sid] = codes
	}
	return result, nil
}
