package model

import (
	"time"

	"feedchamber/internal/pkg"

	"gorm.io/gorm"
)

type User struct {
	// ID is the canonical user identifier (ULID). Used everywhere: sessions,
	// API routes, FK columns, and cross-service calls.
	ID            string `gorm:"primaryKey" json:"id"`
	Username      string `gorm:"uniqueIndex;not null" json:"username"`
	DisplayName   string `gorm:"size:32" json:"display_name"`
	Email         string `gorm:"uniqueIndex;not null" json:"email"`
	Avatar        string `json:"avatar"`
	Bio           string `gorm:"size:500" json:"bio"`
	CoverImage    string `gorm:"size:512" json:"cover_image"`
	ProfileLayout string `gorm:"type:text" json:"profile_layout"`
	Role          string `gorm:"default:user" json:"role"`
	EmailVerified bool   `gorm:"default:false" json:"email_verified"`

	// Trust level / XP 派生缓存。权威来源是行为表 + xp_rules + trust_levels +
	// xp_adjustments，写入单点是 service.TrustService.Recompute；任何其它
	// service 直接改这些字段都算漂移。
	TrustLevel       int        `gorm:"default:0;index" json:"trust_level"`
	TotalXP          int        `gorm:"column:total_xp;default:0" json:"total_xp"`
	LevelEvaluatedAt *time.Time `json:"level_evaluated_at,omitempty"`
	LevelMetrics     string     `gorm:"type:text" json:"level_metrics,omitempty"`
	// LevelManual 表示当前等级由管理员手工指定；为 true 时 Recompute 不自动
	// 升降级（仍会刷新 XP 与指标快照），避免管理员设级被下一次发帖静默打回。
	LevelManual bool `gorm:"default:false" json:"level_manual"`

	// Check-in statistics
	TotalCheckIns   int    `gorm:"default:0" json:"total_checkins"`
	CurrentStreak   int    `gorm:"default:0" json:"current_streak"`
	MaxStreak       int    `gorm:"default:0" json:"max_streak"`
	LastCheckInDate string `gorm:"size:10" json:"last_check_in_date"`

	// 社区自治（motion）相关字段
	AbuseCount        int        `gorm:"default:0" json:"abuse_count"`  // 被标记为滥诉的动议次数
	ExiledUntil       *time.Time `json:"exiled_until,omitempty"`        // 暗区软隔离截止（空=永久），不等于封号
	MotionBannedUntil *time.Time `json:"motion_banned_until,omitempty"` // 限权：禁止发起动议的截止

	// Transient: populated only for leaderboard posts ranking, not persisted
	PostCount int `gorm:"-" json:"post_count"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty" swaggertype:"string"`
}

// UserResponse is the public-facing DTO for User.
// It omits internal fields like check-in stats and DeletedAt.
type UserResponse struct {
	ID            string    `json:"id"`
	Username      string    `json:"username"`
	DisplayName   string    `json:"display_name"`
	Email         string    `json:"email"`
	Avatar        string    `json:"avatar"`
	Bio           string    `json:"bio"`
	CoverImage    string    `json:"cover_image"`
	ProfileLayout string    `json:"profile_layout"`
	Role          string    `json:"role"`
	EmailVerified bool      `json:"email_verified"`
	TrustLevel    int       `json:"trust_level"`
	TotalXP       int       `json:"total_xp"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// PublicUser is the public-facing user DTO for anonymous profile views.
// It intentionally omits email and other private account fields.
type PublicUser struct {
	ID            string    `json:"id"`
	Username      string    `json:"username"`
	DisplayName   string    `json:"display_name"`
	Avatar        string    `json:"avatar"`
	Bio           string    `json:"bio"`
	CoverImage    string    `json:"cover_image"`
	ProfileLayout string    `json:"profile_layout"`
	Role          string    `json:"role"`
	TrustLevel    int       `json:"trust_level"`
	TotalXP       int       `json:"total_xp"`
	CreatedAt     time.Time `json:"created_at"`
}

func (u *User) ToResponse() UserResponse {
	return UserResponse{
		ID:            u.ID,
		Username:      u.Username,
		DisplayName:   u.DisplayName,
		Email:         u.Email,
		Avatar:        u.Avatar,
		Bio:           u.Bio,
		CoverImage:    u.CoverImage,
		ProfileLayout: u.ProfileLayout,
		Role:          u.Role,
		EmailVerified: u.EmailVerified,
		TrustLevel:    u.TrustLevel,
		TotalXP:       u.TotalXP,
		CreatedAt:     u.CreatedAt,
		UpdatedAt:     u.UpdatedAt,
	}
}

func (u *User) ToPublic() PublicUser {
	return PublicUser{
		ID:            u.ID,
		Username:      u.Username,
		DisplayName:   u.DisplayName,
		Avatar:        u.Avatar,
		Bio:           u.Bio,
		CoverImage:    u.CoverImage,
		ProfileLayout: u.ProfileLayout,
		Role:          u.Role,
		TrustLevel:    u.TrustLevel,
		TotalXP:       u.TotalXP,
		CreatedAt:     u.CreatedAt,
	}
}

func (u *User) BeforeCreate(_ *gorm.DB) error {
	if u.ID == "" {
		u.ID = pkg.NewID()
	}
	return nil
}

func (u *User) TableName() string { return "users" }

type PublicProfileResponse struct {
	User         PublicUser `json:"user"`
	PostCount    int        `json:"post_count"`
	CommentCount int        `json:"comment_count"`
	LikeCount    int        `json:"like_count"`
	IsFollowing  bool       `json:"is_following"`
}

type ActivityResponse struct {
	Year    int            `json:"year"`
	Entries map[string]int `json:"entries"`
}
