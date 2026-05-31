package model

import (
	"time"

	"orinforum/internal/pkg"

	"gorm.io/gorm"
)

type User struct {
	ID        string    `gorm:"primaryKey" json:"id" bson:"_id"`
	UUID      string    `gorm:"uniqueIndex" json:"uuid" bson:"uuid"`
	Username  string    `gorm:"uniqueIndex;not null" json:"username" bson:"username"`
	Email     string    `gorm:"uniqueIndex;not null" json:"email" bson:"email"`
	Password  string    `json:"-" bson:"password"`
	Avatar    string    `json:"avatar" bson:"avatar"`
	Bio       string    `gorm:"size:500" json:"bio" bson:"bio"`
	Role      string    `gorm:"default:user" json:"role" bson:"role"`
	CreatedAt time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt time.Time `json:"updated_at" bson:"updated_at"`
}

func (u *User) BeforeCreate(_ *gorm.DB) error {
	if u.UUID == "" {
		u.UUID = pkg.NewID()
	}
	return nil
}

func (u *User) TableName() string { return "users" }