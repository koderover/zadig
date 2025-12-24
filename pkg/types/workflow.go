/*
Copyright 2022 The KodeRover Authors.

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

package types

type ParameterSettingType string

const (
	StringType      ParameterSettingType = "string"
	ChoiceType      ParameterSettingType = "choice"
	MultiSelectType ParameterSettingType = "multi-select"
	ImageType       ParameterSettingType = "image"
	Script          ParameterSettingType = "script"
	FileType        ParameterSettingType = "file"
)

type ParameterSetting struct {
	// 参数名称
	Key string `json:"key" binding:"required"`
	// 参数类型
	Type ParameterSettingType `json:"type" binding:"required"`
	// 默认值
	DefaultValue string `json:"default_value" binding:"required"`
	// 可选值列表
	ChoiceOption []string `json:"choice_option"`
	// 多选值列表
	ChoiceValue []string `json:"choice_value"`
	// 是否为敏感信息
	IsCredential bool `json:"is_credential"`
	// 参数描述
	Description string `json:"description"`
}

type ExternalSetting struct {
	SystemID string                  `json:"system_id"`
	Endpoint string                  `json:"endpoint"`
	Method   string                  `json:"method"`
	Headers  []*KV                   `json:"headers"`
	Body     string                  `json:"body"`
	Params   []*ExternalParamMapping `json:"params"`
}

type ExternalParamMapping struct {
	// zadig parameter key
	ParamKey string `json:"param_key"`
	// response parameter key
	ResponseKey string `json:"response_key"`
	Display     bool   `json:"display"`
}

// OpenAPIDockerBuildStep is the struct for docker build in the product workflow
// Mind that the templateID will be empty in the openAPI mode
type OpenAPIDockerBuildStep struct {
	// 构建上下文目录
	BuildContextDir string `json:"build_context_dir"`
	// Dockerfile 源，local 为代码库，template 为模板库
	DockerfileSource string `json:"dockerfile_source"`
	// Dockerfile 的绝对路径
	DockerfileDirectory string `json:"dockerfile_directory"`
	// 模板名称
	TemplateName string `json:"template_name"`
	// 构建参数
	BuildArgs string `json:"build_args"`
	// 是否启用 Buildkit
	EnableBuildkit bool `json:"enable_buildkit"`
	// 平台
	Platforms string `json:"platforms"`
}
