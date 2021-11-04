package models

import (
	"github.com/globalsign/mgo/bson"
)

type EmailHost struct {
	ID        int    `bson:"id" json:"id"`
	Name      string `bson:"name" json:"name"`
	Port      int    `bson:"port"  json:"port"`
	Username  string `bson:"username" json:"username"`
	Password  string `bson:"password"  json:"password"`
	IsTLS     bool   `bson:"is_tls" json:"isTLS"`
	CreatedAt int64  `bson:"created_at" json:"created_at"`
	UpdatedAt int64  `bson:"updated_at" json:"updated_at"`
}

func (EmailHost) TableName() string {
	return "email_host"
}

type EmailService struct {
	ObjectID       bson.ObjectId `bson:"_id,omitempty"`
	ID             int           `json:"id"   	         bson:"id"`
	Name           string        `json:"name" 	         bson:"name"`
	Address        string        `json:"address" 	     bson:"address"`
	DisplayName    string        `json:"display_name" 	 bson:"display_name"`
	Theme          string        `json:"theme" 	         bson:"theme"`
	OrganizationID int           `json:"organization_id" bson:"organization_id"`
	CreatedAt      int64         `json:"created_at" 	 bson:"created_at"`
	UpdatedAt      int64         `json:"updated_at" 	 bson:"updated_at"`
	DeletedAt      int64         `json:"deleted_at" 	 bson:"deleted_at"`
}

func (EmailService) TableName() string {
	return "email_service"
}
