package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type YamlTemplate struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	Name      string             `bson:"name"          json:"name"`
	Content   string             `bson:"content"       json:"content"`
	Variables []*Variable        `bson:"variables"     json:"variables"`
}

type Variable struct {
	Key   string `bson:"key"   json:"key"`
	Value string `bson:"value" json:"value"`
}

func (YamlTemplate) TableName() string {
	return "yaml_template"
}
