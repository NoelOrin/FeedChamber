package repository

import (
	"context"

	"feedchamber/internal/db"
	"feedchamber/internal/model"
)

// MCPServerRepository 负责外部 MCP Server 登记的持久化。
type MCPServerRepository interface {
	Create(ctx context.Context, s *model.MCPServer) error
	Update(ctx context.Context, s *model.MCPServer) error
	Get(ctx context.Context, id string) (*model.MCPServer, error)
	List(ctx context.Context) ([]model.MCPServer, error)
	// ListEnabledByPurpose 供审核链路查询「承担审核职责且已启用」的服务。
	ListEnabledByPurpose(ctx context.Context, purpose string) ([]model.MCPServer, error)
	Delete(ctx context.Context, id string) error
}

func NewMCPServerRepository(store db.Store) MCPServerRepository {
	return &sqlMCPServerRepo{db: store.SQL()}
}
