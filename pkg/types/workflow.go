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
	StringType   ParameterSettingType = "string"
	ChoiceType   ParameterSettingType = "choice"
	ExternalType ParameterSettingType = "external"
)

type ParameterSetting struct {
	// External type parameter will NOT use this key.
	Key  string               `json:"key"`
	Type ParameterSettingType `json:"type"`
	//DefaultValue defines the default value of the parameter
	DefaultValue string `json:"default_value"`
	// choiceOption Are all options enumerated
	ChoiceOption []string `json:"choice_option"`
	// ExternalSetting It is the configuration of the external system to obtain the variable
	ExternalSetting *ExternalSetting `json:"external_setting"`
	IsCredential    bool             `json:"is_credential"`
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

// DockerBuildInfo is the struct for docker build in the product workflow
// Mind that the templateID will be empty in the openAPI mode
type DockerBuildInfo struct {
	WorkingDirectory    string `json:"working_directory"`
	DockerfileType      string `json:"dockerfile_type"` // it can be of local or template type
	DockerfileDirectory string `json:"dockerfile_directory"`
	BuildArgs           string `json:"build_args"`
	// when the dockerfile type is template, this field will be used to find the ID of the template
	TemplateName string `json:"template_name"`
	TemplateID   string `json:"template_id"`
}
