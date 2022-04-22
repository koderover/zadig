package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type SonarIntegration struct {
	ID            primitive.ObjectID `bson:"_id,omitempty"`
	ServerAddress string             `bson:"server_address"`
	Token         string             `bson:"token"`
}

func (SonarIntegration) TableName() string {
	return "sonar_integration"
}
