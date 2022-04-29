package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type JenkinsIntegration struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"         json:"id,omitempty"`
	URL       string             `bson:"url"                   json:"url"`
	Username  string             `bson:"username"              json:"username"`
	Password  string             `bson:"password"              json:"password"`
	UpdateBy  string             `bson:"update_by"             json:"update_by"`
	UpdatedAt int64              `bson:"updated_at"            json:"updated_at"`
}

func (j JenkinsIntegration) TableName() string {
	return "jenkins_integration"
}
