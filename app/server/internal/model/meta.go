package model

import "time"

// Meta stores internal bookkeeping metadata as key-value pairs, such as
// whether a one-time seed step has already run. It is distinct from Setting,
// which holds user-facing site configuration.
type Meta struct {
	Key       string    `gorm:"primaryKey;size:64" json:"key"`
	Value     string    `gorm:"type:text" json:"value"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Meta) TableName() string { return "meta" }

// Meta keys.
const (
	// MetaSeeded marks that the one-time seed has completed. Once set, later
	// restarts skip the destructive seed steps (default role-permission sync
	// and post/content reseed) so an administrator's customizations survive
	// (R-04). Each gated step still re-runs if its own table is empty, so a
	// wiped table can recover.
	MetaSeeded = "seeded"

	// MustChangePasswordPrefix + userID marks an account that must change its
	// password before it can be trusted (e.g. the seeded admin/admin123). The
	// key's mere presence means "must change"; it is deleted once the password
	// is changed. (R-05)
	MustChangePasswordPrefix = "must_change_password:"
)

// MustChangePasswordKey returns the meta key flagging that the given user must
// change their password. Keyed by user ID (stable) rather than username, which
// can be renamed during first change.
func MustChangePasswordKey(userID string) string {
	return MustChangePasswordPrefix + userID
}
