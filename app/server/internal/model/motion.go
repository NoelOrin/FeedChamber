package model

import "time"

// 社区自治动议（community motion）子系统。与 Report 完全并行、不共享状态机：
// 举报 = 违规上行 → 管理员私裁；动议 = 灰区集体裁判 → 信誉加权表决。
//
// 动议类型：
//   - fold：折叠 / 内容降权（不删不封）
//   - watch：观察（限时标记，留痕）
//   - escalate：移交版主（passed 后生成一条 Report，进入既有 report:manage 队列）
//   - exile：放逐 / 暗区软隔离（帖子路由到 dark-zone；用户置 exiled_until）
//   - reverse：翻案（针对已 fold / exile 的结果，达同等门槛解除软隔离）
const (
	MotionTypeFold     = "fold"
	MotionTypeWatch    = "watch"
	MotionTypeEscalate = "escalate"
	MotionTypeExile    = "exile"
	MotionTypeReverse  = "reverse"

	MotionStatusPending    = "pending"    // 公示 + 表决开放期
	MotionStatusPassed     = "passed"     // 达法定人数且赞成 > 反对
	MotionStatusRejected   = "rejected"   // 未达门槛或反对占优
	MotionStatusOverturned = "overturned" // 被 motion:manage 翻案

	MotionTargetPost    = "post"
	MotionTargetComment = "comment"
	MotionTargetUser    = "user"

	MotionVoteApprove = "approve"
	MotionVoteReject  = "reject"
)

// Motion 一条社区动议。
type Motion struct {
	ID          string `gorm:"primaryKey" json:"id"`
	TargetType  string `gorm:"size:16;index" json:"target_type"` // post|comment|user
	TargetID    string `gorm:"index" json:"target_id"`
	Type        string `gorm:"size:16;index" json:"type"` // fold|watch|escalate|exile|reverse
	Reason      string `gorm:"type:text" json:"reason"`
	Status      string `gorm:"size:16;default:pending;index" json:"status"`
	InitiatorID string `gorm:"index" json:"initiator_id"`
	// 加权票数（按投票人 reputation 加权，weight 之和）
	ApproveWeight float64 `gorm:"default:0" json:"approve_weight"`
	RejectWeight  float64 `gorm:"default:0" json:"reject_weight"`
	VoteCount     int     `gorm:"default:0" json:"vote_count"`
	// 反猎巫：rejected 后由 motion:manage 标记为恶意滥诉
	AbuseFlag bool `gorm:"default:false;index" json:"abuse_flag"`
	// 原动议（仅 reverse 类型指向被翻案的动议）
	SourceMotionID string `gorm:"index" json:"source_motion_id"`
	// exile 用户类动议的软隔离截止（空 = 永久）；fold/exile 内容标记复用 posts.exiled
	ExiledUntil *time.Time `json:"exiled_until,omitempty"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (Motion) TableName() string { return "motions" }

// MotionVote 单张选票。motion_id + voter_id 唯一，一人一票。
type MotionVote struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	MotionID  string    `gorm:"uniqueIndex:idx_motion_voter;index" json:"motion_id"`
	VoterID   string    `gorm:"uniqueIndex:idx_motion_voter" json:"voter_id"`
	Side      string    `gorm:"size:8" json:"side"` // approve|reject
	Weight    float64   `json:"weight"`
	CreatedAt time.Time `json:"created_at"`
}

func (MotionVote) TableName() string { return "motion_votes" }

// MotionListItem 列表扁平化输出。
type MotionListItem struct {
	ID            string     `json:"id"`
	TargetType    string     `json:"target_type"`
	TargetID      string     `json:"target_id"`
	Type          string     `json:"type"`
	Reason        string     `json:"reason"`
	Status        string     `json:"status"`
	InitiatorID   string     `json:"initiator_id"`
	ApproveWeight float64    `json:"approve_weight"`
	RejectWeight  float64    `json:"reject_weight"`
	VoteCount     int        `json:"vote_count"`
	AbuseFlag     bool       `json:"abuse_flag"`
	ExiledUntil   *time.Time `json:"exiled_until,omitempty"`
	ResolvedAt    *time.Time `json:"resolved_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

// ToListItem 把实体投影为列表 DTO。
func (m *Motion) ToListItem() MotionListItem {
	return MotionListItem{
		ID:            m.ID,
		TargetType:    m.TargetType,
		TargetID:      m.TargetID,
		Type:          m.Type,
		Reason:        m.Reason,
		Status:        m.Status,
		InitiatorID:   m.InitiatorID,
		ApproveWeight: m.ApproveWeight,
		RejectWeight:  m.RejectWeight,
		VoteCount:     m.VoteCount,
		AbuseFlag:     m.AbuseFlag,
		ExiledUntil:   m.ExiledUntil,
		ResolvedAt:    m.ResolvedAt,
		CreatedAt:     m.CreatedAt,
	}
}
