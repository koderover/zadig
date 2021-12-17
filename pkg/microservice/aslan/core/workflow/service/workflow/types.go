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

package workflow

type WorkflowV3 struct {
	ID          string                   `json:"id"`
	Name        string                   `json:"name"`
	ProjectName string                   `json:"project_name"`
	Description string                   `json:"description"`
	Parameters  []*ParameterSetting      `json:"parameters"`
	SubTasks    []map[string]interface{} `json:"sub_tasks"`
}

type WorkflowV3Brief struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ProjectName string `json:"project_name"`
}

type ParameterSettingType string

const (
	StringType   ParameterSettingType = "string"
	ChoiceType   ParameterSettingType = "choice"
	ExternalType ParameterSettingType = "external"
)

type ParameterSetting struct {
	// External type parameter will NOT use this key.
	Key  string               `json:"key"`
	Type ParameterSettingType `json:"type"`
	//DefaultValue is the
	DefaultValue string `json:"default_value"`
	// choiceOption Are all options enumerated
	ChoiceOption []string `json:"choice_option"`
	// ExternalSetting It is the configuration of the external system to obtain the variable
	ExternalSetting *ExternalSetting `json:"external_setting"`
}

type ExternalSetting struct {
	SystemID string                  `json:"system_id"`
	Endpoint string                  `json:"endpoint"`
	Method   string                  `json:"method"`
	Headers  []*KV                   `json:"headers"`
	Body     string                  `json:"body"`
	Params   []*ExternalParamMapping `json:"params"`
}

type KV struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type ExternalParamMapping struct {
	// zadig变量名称
	ParamKey string `json:"param_key"`
	// 返回中的key的位置
	ResponseKey string `json:"response_key"`
	Display     bool   `json:"display"`
}

type WorkflowV3TaskArgs struct {
	Type    string                   `json:"type"`
	Key     string                   `json:"key,omitempty"`
	Value   string                   `json:"value,omitempty"`
	Choice  []string                 `json:"choice,omitempty"`
	Options []map[string]interface{} `json:"options,omitempty"`
}
