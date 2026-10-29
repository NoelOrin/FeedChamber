package mcp

import (
	"context"

	"feedchamber/internal/model"

	"gorm.io/gorm"
)

// gormLookup 用 GORM 实现 APIKeyLookup。
//
// 为什么不去复用 gateway.APIKeyManager：那个 manager 直接写 iris 响应并
// stop 执行，错误形状是论坛信封；而 MCP 必须回 JSON-RPC error。两者共享的是
// 「校验语义」（bcrypt / 过期 / 吊销），不是代码——语义在本包 enforcer.go 里
// 与 gateway 保持一致，测试用 TestEnforcer_* 钉住。
type gormLookup struct{ db *gorm.DB }

// NewGORMLookup 构造一个基于 GORM 的凭据查询器。
func NewGORMLookup(db *gorm.DB) APIKeyLookup {
	return &gormLookup{db: db}
}

func (g *gormLookup) LookupAPIKey(ctx context.Context, keyID string) (*model.APIKey, error) {
	if g.db == nil {
		return nil, nil
	}
	var key model.APIKey
	err := g.db.WithContext(ctx).First(&key, "key_id = ?", keyID).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &key, nil
}

func (g *gormLookup) LookupUser(ctx context.Context, userID string) (*model.User, error) {
	if g.db == nil {
		return nil, nil
	}
	var user model.User
	err := g.db.WithContext(ctx).First(&user, "id = ?", userID).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}
