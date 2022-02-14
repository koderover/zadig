package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type SystemSetting struct {
	ID                  primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	WorkflowConcurrency int64              `bson:"workflow_concurrency" json:"workflow_concurrency"`
	BuildConcurrency    int64              `bson:"build_concurrency" json:"build_concurrency"`
	UpdateTime          int64              `bson:"update_time" json:"update_time"`
}

func (SystemSetting) TableName() string {
	return "system_setting"
}
