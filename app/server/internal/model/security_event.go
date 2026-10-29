package model

// SecurityEvent records security-related events for audit and alerting.
type SecurityEvent struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	UserUUID  string `gorm:"index" json:"user_uuid"`
	IP        string `gorm:"index;size:45" json:"ip"`
	EventType string `gorm:"index" json:"event_type"`       // brute_force, unusual_location, rate_anomaly, token_reuse
	Severity  string `gorm:"index;size:20" json:"severity"` // low, medium, high, critical
	Detail    string `gorm:"type:text" json:"detail"`
	Action    string `json:"action"` // blocked, mfa_required, alert_only
	Handled   bool   `json:"handled"`
	CreatedAt int64  `json:"created_at"`
}

func (SecurityEvent) TableName() string { return "security_events" }
