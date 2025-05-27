package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type WorkflowV4GeneralHook struct {
	ID           primitive.ObjectID `bson:"_id,omitempty"       yaml:"-"                   json:"id"`
	Name         string             `bson:"name" json:"name"`
	WorkflowName string             `bson:"workflow_name"             json:"workflow_name"`
	ProjectName  string             `bson:"project_name"              json:"project_name"`
	Enabled      bool               `bson:"enabled" json:"enabled"`
	Description  string             `bson:"description" json:"description"`
	WorkflowArg  *WorkflowV4        `bson:"workflow_arg" json:"workflow_arg"`
}

func (WorkflowV4GeneralHook) TableName() string {
	return "workflow_v4_general_hook"
}
