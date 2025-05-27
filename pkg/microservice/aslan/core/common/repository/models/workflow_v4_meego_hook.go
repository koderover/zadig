package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type WorkflowV4MeegoHook struct {
	ID                  primitive.ObjectID `bson:"_id,omitempty"       yaml:"-"                   json:"id"`
	Name                string             `bson:"name" json:"name"`
	WorkflowName        string             `bson:"workflow_name"             json:"workflow_name"`
	ProjectName         string             `bson:"project_name"              json:"project_name"`
	MeegoID             string             `bson:"meego_id" json:"meego_id"`
	MeegoSystemIdentity string             `bson:"meego_system_identity" json:"meego_system_identity"`
	MeegoURL            string             `bson:"meego_url"             json:"meego_url"`
	Enabled             bool               `bson:"enabled" json:"enabled"`
	Description         string             `bson:"description" json:"description"`
	WorkflowArg         *WorkflowV4        `bson:"workflow_arg" json:"workflow_arg"`
}

func (WorkflowV4MeegoHook) TableName() string {
	return "workflow_v4_meego_hook"
}
