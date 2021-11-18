package models

import (
	"github.com/koderover/zadig/pkg/types"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type WorkflowV3 struct {
	ID          primitive.ObjectID       `bson:"_id"`
	Name        string                   `bson:"name"`
	ProjectName string                   `bson:"project_name"`
	Description string                   `bson:"description"`
	Parameters  []*ParameterSetting      `bson:"parameters"`
	SubTasks    []map[string]interface{} `bson:"sub_tasks"`
	CreatedBy   string                   `bson:"created_by"`
	CreateTime  int64                    `bson:"create_time"`
	UpdatedBy   string                   `bson:"updated_by"`
	UpdateTime  int64                    `bson:"update_time"`
}

type ParameterSetting struct {
	// External type parameter will NOT use this key.
	Key string `bson:"key"`
	// Type list：
	// string
	// choice
	// external
	Type string `bson:"type"`
	//DefaultValue is the
	DefaultValue string `bson:"default_value"`
	// choiceOption 是枚举的所有选项
	ChoiceOption []string `bson:"choice_option"`
	// ExternalSetting 是外部系统获取变量的配置
	ExternalSetting *ExternalSetting `bson:"external_setting"`
}

type ExternalSetting struct {
	// 外部系统ID
	SystemID string `bson:"system_id"`
	// Endpoint路径
	Endpoint string `bson:"endpoint"`
	// 请求方法
	Method string `bson:"method"`
	// 请求头
	Headers []*KV `bson:"headers"`
	// 请求体
	Body string `bson:"body"`
	// 外部变量配置
	Params []*ExternalParamMapping `bson:"params"`
}

type KV struct {
	Key   string `bson:"key"`
	Value string `bson:"value"`
}

type ExternalParamMapping struct {
	// zadig变量名称
	ParamKey string `bson:"param_key"`
	// 返回中的key的位置
	ResponseKey string `bson:"response_key"`
	Display     bool   `bson:"display"`
}

// WorkflowV3Args 工作流v3任务参数
type WorkflowV3Args struct {
	ProjectName string              `bson:"project_name"            json:"project_name"`
	ID          string              `bson:"id"                      json:"id"`
	Name        string              `bson:"name"                    json:"name"`
	Builds      []*types.Repository `bson:"builds"                  json:"builds"`
	BuildArgs   []*KeyVal           `bson:"build_args"              json:"build_args"`
}

func (WorkflowV3) TableName() string {
	return "workflow_v3"
}
