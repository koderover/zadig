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

type JenkinsBuildParam struct {
	Name         string           `bson:"name,omitempty"                json:"name,omitempty"`
	Value        interface{}      `bson:"value,omitempty"               json:"value,omitempty"`
	Type         JenkinsParamType `bson:"type,omitempty"                json:"type,omitempty"`
	AutoGenerate bool             `bson:"auto_generate,omitempty"       json:"auto_generate,omitempty"`
	ChoiceOption []string         `bson:"choice_option,omitempty"       json:"choice_option,omitempty"`
	ChoiceValue  []string         `bson:"choice_value,omitempty"        json:"choice_value,omitempty"`
}

type JenkinsParamType string

const (
	Str    JenkinsParamType = "string"
	Choice JenkinsParamType = "choice"
)

type VMDeployType string

const (
	VMDeployTypeLocal    VMDeployType = "local"
	VMDeployTypeSSHAgent VMDeployType = "ssh_agent"
)

type VMDeployArtifactType string

const (
	VMDeployArtifactTypeFile  VMDeployArtifactType = "file"
	VMDeployArtifactTypeImage VMDeployArtifactType = "image"
	VMDeployArtifactTypeOther VMDeployArtifactType = "other"
)
