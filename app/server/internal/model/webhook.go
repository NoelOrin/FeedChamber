package model

import "time"

type Webhook struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	URL       string    `gorm:"size:512;not null" json:"url"`
	Secret    string    `gorm:"size:128" json:"-"`
	Events    string    `gorm:"type:text" json:"events"` // JSON array string
	Enabled   bool      `gorm:"default:true" json:"enabled"`
	TLSVerify bool      `gorm:"default:true" json:"tls_verify"`
	CreatedAt time.Time `json:"created_at"`
}

func (Webhook) TableName() string { return "webhooks" }

type WebhookDelivery struct {
	ID             string     `gorm:"primaryKey" json:"id"`
	WebhookID      string     `gorm:"index;not null" json:"webhook_id"`
	Event          string     `gorm:"size:64;not null" json:"event"`
	Payload        string     `gorm:"type:text" json:"payload"`
	Status         string     `gorm:"size:16;default:pending" json:"status"` // pending|success|failed
	Attempts       int        `gorm:"default:0" json:"attempts"`
	Kind           string     `gorm:"size:16;default:event" json:"kind"`
	ResponseStatus int        `json:"response_status,omitempty"`
	ResponseBody   string     `gorm:"type:text" json:"response_body,omitempty"`
	LatencyMs      int64      `json:"latency_ms,omitempty"`
	NextRetryAt    *time.Time `json:"next_retry_at,omitempty"`
	LastError      string     `gorm:"type:text" json:"last_error"`
	CreatedAt      time.Time  `json:"created_at"`
}

func (WebhookDelivery) TableName() string { return "webhook_deliveries" }
