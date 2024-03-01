package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type ImageTags struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	RegistryID  string             `bson:"registry_id" json:"registry_id"`
	RegProvider string             `bson:"reg_provider" json:"reg_provider"`
	ImageName   string             `bson:"image_name" json:"image_name"`
	Namespace   string             `bson:"namespace" json:"namespace"`
	ImageTags   []*ImageTag        `bson:"image_tags" json:"image_tags"`
}

type ImageTag struct {
	TagName string `bson:"tag_name" json:"tag_name"`
	Digest  string `bson:"digest" json:"digest"`
	Created string `bson:"created" json:"created"`
}

func (ImageTags) TableName() string {
	return "image_tags"
}
