/*
Copyright 2021 The KodeRover Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package models

import (
	"github.com/koderover/zadig/pkg/types"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type WorkflowV3 struct {
	ID          primitive.ObjectID       `bson:"_id,omitempty"  json:"id,omitempty"`
	Name        string                   `bson:"name"           json:"name"`
	ProjectName string                   `bson:"project_name"   json:"project_name"`
	Description string                   `bson:"description"    json:"description"`
	Parameters  []*ParameterSetting      `bson:"parameters"     json:"parameters"`
	SubTasks    []map[string]interface{} `bson:"sub_tasks"      json:"sub_tasks"`
	CreatedBy   string                   `bson:"created_by"     json:"created_by"`
	CreateTime  int64                    `bson:"create_time"    json:"create_time"`
	UpdatedBy   string                   `bson:"updated_by"     json:"updated_by"`
	UpdateTime  int64                    `bson:"update_time"    json:"update_time"`
}

type ParameterSettingType string

const (
	StringType   ParameterSettingType = "string"
	ChoiceType   ParameterSettingType = "choice"
	ExternalType ParameterSettingType = "external"
)

type ParameterSetting struct {
	// External type parameter will NOT use this key.
	Key  string               `bson:"key" json:"key"`
	Type ParameterSettingType `bson:"type" json:"type"`
	//DefaultValue
	DefaultValue string `bson:"default_value" json:"default_value"`
	// choiceOption Are all options enumerated
	ChoiceOption []string `bson:"choice_option" json:"choice_option"`
	// ExternalSetting It is the configuration of the external system to obtain the variable
	ExternalSetting *ExternalSetting `bson:"external_setting" json:"external_setting"`
}

type ExternalSetting struct {
	SystemID string                  `bson:"system_id" json:"system_id"`
	Endpoint string                  `bson:"endpoint" json:"endpoint"`
	Method   string                  `bson:"method" json:"method"`
	Headers  []*KV                   `bson:"headers" json:"headers"`
	Body     string                  `bson:"body" json:"body"`
	Params   []*ExternalParamMapping `bson:"params" json:"params"`
}

type KV struct {
	Key   string `bson:"key"   json:"key"`
	Value string `bson:"value" json:"value"`
}

type ExternalParamMapping struct {
	// zadig变量名称
	ParamKey string `bson:"param_key" json:"param_key"`
	// 返回中的key的位置
	ResponseKey string `bson:"response_key" json:"response_key"`
	Display     bool   `bson:"display"      json:"display"`
}

// WorkflowV3Args 工作流v3任务参数
type WorkflowV3Args struct {
	ID          string              `bson:"id"                      json:"id"`
	ProjectName string              `bson:"project_name"            json:"project_name"`
	Name        string              `bson:"name"                    json:"name"`
	Builds      []*types.Repository `bson:"builds"                  json:"builds"`
	BuildArgs   []*KeyVal           `bson:"build_args"              json:"build_args"`
}

func (WorkflowV3) TableName() string {
	return "workflow_v3"
}
