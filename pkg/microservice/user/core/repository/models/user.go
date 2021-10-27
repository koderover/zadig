package models

import "github.com/koderover/zadig/pkg/microservice/user/core"

type User struct {
	core.Model
	UID          string `json:"uid"`
	Name         string `json:"name"`
	IdentityType string `gorm:"default:'unknown'" json:"identity_type"`
	Email        string `json:"email"`
	Phone        string `json:"phone"`
}

// TableName sets the insert table name for this struct type
func (User) TableName() string {
	return "user"
}
