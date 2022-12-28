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

package template

import (
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/fs"
)

type Chart struct {
	Name       string                  `json:"name"`
	CodehostID int                     `json:"codehostID"`
	Owner      string                  `json:"owner"`
	Namespace  string                  `json:"namespace"`
	Repo       string                  `json:"repo"`
	Branch     string                  `json:"branch"`
	Path       string                  `json:"path"`
	Variables  []*models.ChartVariable `json:"variables,omitempty"`

	Files []*fs.FileInfo `json:"files,omitempty"`
}

type DockerfileTemplate struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type DockerfileListObject struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type DockerfileDetail struct {
	ID        string                  `json:"id"`
	Name      string                  `json:"name"`
	Content   string                  `json:"content"`
	Variables []*models.ChartVariable `json:"variable"`
}

type BuildReference struct {
	BuildName   string `json:"build_name"`
	ProjectName string `json:"project_name"`
}

type YamlTemplate struct {
	Name         string                  `json:"name"`
	Content      string                  `json:"content"`
	Variable     []*models.ChartVariable `json:"variable"`
	VariableYaml string                  `json:"variable_yaml"`
	// services vars stores the flat keys of variable which can be edited in service templates, used in k8s projects
	ServiceVars []string `json:"service_vars"`
}

type YamlListObject struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type YamlDetail struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Content string `json:"content"`
	//Variables    []*models.ChartVariable `json:"variable"`
	VariableYaml string               `json:"variable_yaml"`
	VariableKVs  []*models.VariableKV `json:"variable_kvs"`
	ServiceVars  []string             `json:"service_vars"`
}

type ServiceReference struct {
	ProjectName string `json:"project_name"`
	ServiceName string `json:"service_name"`
}

type BuildTemplateReference struct {
	BuildName     string   `json:"build_name"`
	ProjectName   string   `json:"project_name"`
	ServiceModule []string `json:"service_module"`
}
