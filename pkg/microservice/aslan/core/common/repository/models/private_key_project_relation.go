package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type PrivateKeyProjectRelation struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"          json:"id,omitempty"`
	Name        string             `bson:"name"                   json:"name"`
	ProjectName string             `bson:"project_name"           json:"project_name"`
	IP          string             `bson:"ip"                     json:"ip"`
}

func (PrivateKeyProjectRelation) TableName() string {
	return "private_key_project_relation"
}
