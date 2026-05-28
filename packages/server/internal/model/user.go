package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type User struct {
	ID        string    `gorm:"primaryKey" bson:"_id"`
	UUID      string    `gorm:"type:uuid;uniqueIndex" bson:"uuid"`
	Username  string    `gorm:"uniqueIndex;not null" bson:"username"`
	Email     string    `gorm:"uniqueIndex;not null" bson:"email"`
	Password  string    `json:"-" bson:"password"`
	Avatar    string    `bson:"avatar"`
	Role      string    `gorm:"default:user" bson:"role"`
	CreatedAt time.Time `bson:"created_at"`
	UpdatedAt time.Time `bson:"updated_at"`
}

func (u *User) BeforeCreate(_ *gorm.DB) error {
	if u.UUID == "" {
		u.UUID = uuid.New().String()
	}
	return nil
}

func (u *User) TableName() string { return "users" }