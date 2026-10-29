package model

import "time"

// LoginHistory records each login attempt for security auditing.
type LoginHistory struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserUUID  string    `gorm:"index;not null" json:"user_uuid"`
	IP        string    `json:"ip"`
	Location  string    `json:"location"`
	UserAgent string    `json:"user_agent"`
	Success   bool      `json:"success"`
	CreatedAt time.Time `json:"created_at"`
}

func (LoginHistory) TableName() string { return "login_histories" }
