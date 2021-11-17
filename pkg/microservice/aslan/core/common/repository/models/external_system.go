package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type ExternalSystem struct {
	ID       primitive.ObjectID `bson:"_id,omitempty"`
	Name     string             `bson:"name"`
	Server   string             `bson:"server"`
	APIToken string             `bson:"api_token"`
}

func (ExternalSystem) TableName() string {
	return "external_system"
}
