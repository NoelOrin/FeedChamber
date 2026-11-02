package service

import (
	"context"
	"time"

	"feedchamber/internal/model"
)

// MotionConfig 动议子系统可调参数（可由 settings 扩展，本期用默认值）。
type MotionConfig struct {
	// 版面法定人数（加权总参与门槛）。user 类动议乘以 UserQuorumFactor。
	Quorum           float64
	UserQuorumFactor float64
	// 反猎巫：被标记滥诉达此阈值后限制发起资格。
	AbuseThreshold int
	// 限权时长（天）。
	MotionBanDays int
	// OpenDuration 单一开放期（design D1 的 24h）：期满未达法定人数即按
	// 当前票数结算，避免动议永远悬在 pending。
	OpenDuration time.Duration
}

// DefaultMotionConfig 默认参数。
func DefaultMotionConfig() MotionConfig {
	return MotionConfig{
		Quorum:           20,
		UserQuorumFactor: 2,
		AbuseThreshold:   3,
		MotionBanDays:    7,
		OpenDuration:     24 * time.Hour,
	}
}

type MotionService interface {
	// Create 发起动议。actorID 不能是受限（motion_banned_until）用户。
	Create(ctx context.Context, initiatorID, targetType, targetID, motionType, reason, sourceMotionID string) (*model.Motion, error)
	// Vote 信誉加权投票。side = approve|reject。开放期（pending）可投。
	Vote(ctx context.Context, motionID, voterID, side string) (*model.Motion, error)
	// List 列表（按 status / targetType 过滤 + 分页）。
	List(ctx context.Context, status, targetType string, page, pageSize int) ([]model.MotionListItem, int64, error)
	// GetByID 详情。
	GetByID(ctx context.Context, id string) (*model.Motion, error)
	// Resolve 结算（可在开放期满或达法定人数时由任意有权限方触发；此处不做权限门，门在 handler/网关）。
	Resolve(ctx context.Context, id, actorID string) (*model.Motion, error)
	// UpdateStatus motion:manage 操作：翻案（overturned）/ 标记滥诉（abuse）/ 升级硬封禁（hardBan）。
	UpdateStatus(ctx context.Context, id, actorID, status string, abuse *bool, hardBan bool) (*model.Motion, error)
	// Delete 发起者撤回（仅 pending）。
	Delete(ctx context.Context, id, actorID string) error
	// StartResolveTicker 后台周期结算已过开放期的 pending 动议，返回停止函数。
	StartResolveTicker(ctx context.Context) context.CancelFunc
}
