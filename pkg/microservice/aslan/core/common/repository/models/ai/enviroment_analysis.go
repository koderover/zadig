package ai

import "go.mongodb.org/mongo-driver/bson/primitive"

type EnvAIAnalysis struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"                json:"id,omitempty"`
	ProjectName string             `bson:"project_name" json:"project_name"`
	EnvName     string             `bson:"env_name" json:"env_name"`
	Production  bool               `bson:"production" json:"production"`
	Status      string             `bson:"status" json:"status"`
	Result      string             `bson:"result" json:"result"`
	Err         string             `bson:"err" json:"err"`
	StartTime   int64              `bson:"start_time" json:"start_time"`
	EndTime     int64              `bson:"end_time" json:"end_time"`
	Updated     int64              `bson:"updated" json:"updated"`
	TriggerName string             `bson:"trigger_name" json:"trigger_name"`
	// 人名 or 触发器名称...
	CreatedBy string `bson:"created_by" json:"created_by"`
	// 人为 or 触发器...
	Source string `bson:"source" json:"source"`
}

func (EnvAIAnalysis) TableName() string {
	return "env_ai_analysis"
}
