package models

import (
	"github.com/globalsign/mgo/bson"
)

type Jira struct {
	ObjectID       bson.ObjectId `bson:"_id,omitempty"`
	ID             int64         `bson:"id"`
	Host           string        `bson:"host"`
	User           string        `bson:"user"`
	AccessToken    string        `bson:"access_token"`
	OrganizationID int           `bson:"organization_id"`
	CreatedAt      int64         `bson:"created_at"`
	UpdatedAt      int64         `bson:"updated_at"`
	DeletedAt      int64         `bson:"deleted_at"`
}

func (Jira) TableName() string {
	return "jira"
}
