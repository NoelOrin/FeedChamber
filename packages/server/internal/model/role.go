package model

import "time"

// Role 角色表，启动时 seeds 写入默认角色 admin/user/ban
type Role struct {
	ID        string    `gorm:"primaryKey" bson:"_id"`
	Name      string    `gorm:"uniqueIndex" bson:"name"`
	CreatedAt time.Time `bson:"created_at"`
}

var DefaultRoles = []Role{
	{Name: "admin"},
	{Name: "user"},
	{Name: "ban"},
}

// roleCache 进程级角色缓存
var roleCache = map[string]struct{}{}

func LoadRoleCache(roles []Role) {
	for k := range roleCache {
		delete(roleCache, k)
	}
	for _, r := range roles {
		roleCache[r.Name] = struct{}{}
	}
}

func IsValidRole(name string) bool {
	_, ok := roleCache[name]
	return ok
}

func HasBanRole(name string) bool {
	return IsValidRole(name) && name == "ban"
}