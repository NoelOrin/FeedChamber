package model

import (
	"sync"
	"time"
)

// Role 角色表，启动时 seeds 写入默认角色 admin/moderator/user/ban
type Role struct {
	ID          string    `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"uniqueIndex" json:"name"`
	DisplayName string    `json:"display_name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

var DefaultRoles = []Role{
	{Name: "admin", DisplayName: "管理员"},
	{Name: "moderator", DisplayName: "版主"},
	{Name: "user", DisplayName: "用户"},
	// pending_user：审批注册模式下的待审核用户，默认无发帖/评论权限
	{Name: "pending_user", DisplayName: "待审核用户"},
	// ban 是账号封禁角色（封号），与 Banned 表的禁言（禁止发言）是两套独立机制：
	// ban 是账号封禁角色（封号），与 Banned 表的禁言（禁止发言）是两套独立机制：
	// - role = "ban"：账号层面封禁，Login() 直接拒绝登录，封号时应同步吊销全部 token
	// - Banned 表：操作层面禁言，用户仍可登录但 BanCheck 中间件拦截写操作
	{Name: "ban"},
}

// roleCache 进程级角色缓存
var roleCache = struct {
	sync.RWMutex
	m map[string]struct{}
}{
	m: make(map[string]struct{}),
}

func LoadRoleCache(roles []Role) {
	roleCache.Lock()
	defer roleCache.Unlock()
	for k := range roleCache.m {
		delete(roleCache.m, k)
	}
	for _, r := range roles {
		roleCache.m[r.Name] = struct{}{}
	}
}

func IsValidRole(name string) bool {
	roleCache.RLock()
	defer roleCache.RUnlock()
	_, ok := roleCache.m[name]
	return ok
}

// HasBanRole 检查账号是否被封号（role = "ban"）。
// 注意：这是账号层面的封禁，在 Login() 阶段拒绝颁发 token。
// 与 BannedService（禁言）是两套独立机制，不要混淆。
func HasBanRole(name string) bool {
	roleCache.RLock()
	defer roleCache.RUnlock()
	_, ok := roleCache.m[name]
	if !ok {
		return false
	}
	return name == "ban"
}
