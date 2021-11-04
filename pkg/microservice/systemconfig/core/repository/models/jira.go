package models

import (
	"github.com/globalsign/mgo/bson"
)

type Jira struct {
	ObjectID       bson.ObjectId `json:"-"					bson:"_id,omitempty"`
	ID             int64         `json:"id"					bson:"id"`
	Host           string        `json:"host"				bson:"host"`
	User           string        `json:"user"				bson:"user"`
	AccessToken    string        `json:"access_token"		bson:"access_token"`
	OrganizationID int           `json:"organization_id"	bson:"organization_id"`
	CreatedAt      int64         `json:"created_at"			bson:"created_at"`
	UpdatedAt      int64         `json:"updated_at"			bson:"updated_at"`
	DeletedAt      int64         `json:"deleted_at"			bson:"deleted_at"`
}

func (Jira) TableName() string {
	return "jira"
}
