package repository

import (
	"context"

	"feedchamber/internal/db"
	"feedchamber/internal/model"
)

// PluginInstallRepository 负责插件安装记录（装在哪、当前状态）的持久化。
//
// 它与 PluginRepository 的分工见 model.PluginInstall 的注释：Plugin 是
// 「登记了什么」，这里是「这一份装在哪个目录、现在什么状态」。
type PluginInstallRepository interface {
	Create(ctx context.Context, in *model.PluginInstall) error
	Update(ctx context.Context, in *model.PluginInstall) error
	GetByPluginID(ctx context.Context, pluginID string) (*model.PluginInstall, error)
	List(ctx context.Context) ([]model.PluginInstall, error)
	// ListByStatus 返回处于给定状态的安装记录，供宿主启动时筛出可恢复的插件。
	ListByStatus(ctx context.Context, status string) ([]model.PluginInstall, error)
	// UpdateStatus 是运行状态回写的窄接口：只改 status/last_error/updated_at，
	// 不做「读-改-写」。状态回调来自 supervisor 的 goroutine，用整行 Update
	// 会把并发的 restarts 计数覆盖掉。
	UpdateStatus(ctx context.Context, pluginID, status, lastError string) error
	Delete(ctx context.Context, pluginID string) error
}

func NewPluginInstallRepository(store db.Store) PluginInstallRepository {
	return &sqlPluginInstallRepo{db: store.SQL()}
}
