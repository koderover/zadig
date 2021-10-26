package models

import "github.com/koderover/zadig/pkg/microservice/user/core"

type User struct {
	core.Model
	Uid          string `json:"uid"`
	Name         string `json:"name"`
	IdentityType string `gorm:"default:'unknown'" json:"identity_type"`
	Email        string `json:"email"`
	Phone        string `json:"phone"`
}
