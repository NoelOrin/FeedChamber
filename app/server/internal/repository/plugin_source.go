package repository

import (
	"context"

	"feedchamber/internal/db"
	"feedchamber/internal/model"
)

// PluginSourceRepository 负责插件来源登记的持久化。
//
// 它是信任模型第一层的数据面：service 在 Install 时读 Status 判断是否
// 已审批，因此 Get 必须返回真实状态（不做缓存），审批与安装之间的
// 竞态窗口只能靠这一读来收窄。
type PluginSourceRepository interface {
	Create(ctx context.Context, s *model.PluginSource) error
	Update(ctx context.Context, s *model.PluginSource) error
	Get(ctx context.Context, id string) (*model.PluginSource, error)
	List(ctx context.Context) ([]model.PluginSource, error)
	Delete(ctx context.Context, id string) error
}

func NewPluginSourceRepository(store db.Store) PluginSourceRepository {
	return &sqlPluginSourceRepo{db: store.SQL()}
}
