package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type DockerfileTemplate struct {
	ID      primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	Name    string             `bson:"name"          json:"name"`
	Content string             `bson:"content"       json:"content"`
}

func (DockerfileTemplate) TableName() string {
	return "dockerfile_template"
}
