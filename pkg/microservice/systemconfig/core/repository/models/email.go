package models

import (
	"github.com/globalsign/mgo/bson"
)

type EmailHost struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	Port           int    `json:"port"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	IsTLS          bool   `json:"isTLS"`
	OrganizationID int    `json:"orgId"`
	CreatedAt      int64  `json:"created_at"`
	UpdatedAt      int64  `json:"updated_at"`
}

func (EmailHost) TableName() string {
	return "email_host"
}

type EmailService struct {
	ObjectID       bson.ObjectId `bson:"_id,omitempty"`
	ID             int           `bson:"id"`
	Name           string        `bson:"name"`
	Address        string        `bson:"address"`
	DisplayName    string        `bson:"display_name"`
	Theme          string        `bson:"theme"`
	OrganizationID int           `bson:"organization_id"`
	CreatedAt      int64         `bson:"created_at"`
	UpdatedAt      int64         `bson:"updated_at"`
	DeletedAt      int64         `bson:"deleted_at"`
}

func (EmailService) TableName() string {
	return "email_service"
}
