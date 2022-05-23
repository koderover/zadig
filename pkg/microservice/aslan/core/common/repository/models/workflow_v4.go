package models

import (
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type WorkflowV4 struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"  json:"id,omitempty"`
	Name        string             `bson:"name"           json:"name"`
	Args        []*KeyVal          `bson:"args"           json:"args"`
	Stages      []*WorkflowStage   `bson:"stages"         json:"stages"`
	ProjectName string             `bson:"project_name"   json:"project_name"`
	Description string             `bson:"description"    json:"description"`
	CreatedBy   string             `bson:"created_by"     json:"created_by"`
	CreateTime  int64              `bson:"create_time"    json:"create_time"`
	UpdatedBy   string             `bson:"updated_by"     json:"updated_by"`
	UpdateTime  int64              `bson:"update_time"    json:"update_time"`
}

type WorkflowStage struct {
	Name string `bson:"name"          json:"name"`
	// default is custom defined by user.
	StageType string      `bson:"type"           json:"type"`
	Spec      interface{} `bson:"spec"           json:"spec"`
	Jobs      []*Job      `bson:"jobs"           json:"jobs"`
}

type Job struct {
	Name       string         `bson:"name"           json:"name"`
	Properties *JobProperties `bson:"properties"     json:"properties"`
	JobType    string         `bson:"type"           json:"type"`
	Spec       interface{}    `bson:"spec"           json:"spec"`
	Steps      []*Step        `bson:"steps"          json:"steps"`
	Outputs    []*Output      `bson:"outputs"        son:"outputs"`
}

type JobProperties struct {
	Timeout         int64     `bson:"timeout"                json:"timeout"`
	Retry           int64     `bson:"retry"                  json:"retry"`
	ResourceRequest string    `bson:"resource_request"       json:"resource_request"`
	ClusterID       string    `bson:"cluster_id"             json:"cluster_id"`
	Args            []*KeyVal `bson:"args"                   json:"args"`
}

type Step struct {
	Name     string          `bson:"name"           json:"name"`
	Timeout  int64           `bson:"timeout"        json:"timeout"`
	StepType config.StepType `bson:"step_type"      json:"step_type"`
	Spec     interface{}     `bson:"Spec"           json:"Spec"`
}

type Output struct {
	Name        string `bson:"name"           json:"name"`
	Description string `bson:"description"    json:"description"`
}

func (WorkflowV4) TableName() string {
	return "workflow_v4"
}
