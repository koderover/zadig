package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type ExternalLink struct {
	ID         primitive.ObjectID `bson:"_id,omitempty"          json:"id,omitempty"`
	Name       string             `bson:"name"                   json:"name"`
	URL        string             `bson:"url"                    json:"url"`
	CreateTime int64              `bson:"create_time"            json:"create_time"`
	UpdateTime int64              `bson:"update_time"            json:"update_time"`
	UpdateBy   string             `bson:"update_by"              json:"update_by"`
}

func (ExternalLink) TableName() string {
	return "external_link"
}
