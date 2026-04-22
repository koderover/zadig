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
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/fs"
	commontypes "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/types"
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
	Name               string                           `json:"name"`
	Content            string                           `json:"content"`
	VariableYaml       string                           `json:"variable_yaml"`
	ServiceVariableKVs []*commontypes.ServiceVariableKV `json:"service_variable_kvs"`

	Source      string         `json:"source,omitempty"`
	CodehostID  int            `json:"codehostID,omitempty"`
	RepoOwner   string         `json:"repo_owner,omitempty"`
	Namespace   string         `json:"namespace,omitempty"`
	RepoName    string         `json:"repo,omitempty"`
	Path        string         `json:"path,omitempty"`
	BranchName  string         `json:"branch,omitempty"`
	RemoteName  string         `json:"remote_name,omitempty"`
	LoadFromDir bool           `json:"load_from_dir,omitempty"`
	Commit      *models.Commit `json:"commit,omitempty"`
}

type YamlListObject struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Source      string         `json:"source,omitempty"`
	CodehostID  int            `json:"codehostID,omitempty"`
	RepoOwner   string         `json:"repo_owner,omitempty"`
	Namespace   string         `json:"namespace,omitempty"`
	Repo        string         `json:"repo,omitempty"`
	Path        string         `json:"path,omitempty"`
	Branch      string         `json:"branch,omitempty"`
	RemoteName  string         `json:"remote_name,omitempty"`
	LoadFromDir bool           `json:"load_from_dir,omitempty"`
	Commit      *models.Commit `json:"commit,omitempty"`
}

type YamlDetail struct {
	ID                 string                           `json:"id"`
	Name               string                           `json:"name"`
	Content            string                           `json:"content"`
	VariableYaml       string                           `json:"variable_yaml"`
	ServiceVariableKVs []*commontypes.ServiceVariableKV `json:"service_variable_kvs"`
	Source             string                           `json:"source,omitempty"`
	CodehostID         int                              `json:"codehostID,omitempty"`
	RepoOwner          string                           `json:"repo_owner,omitempty"`
	Namespace          string                           `json:"namespace,omitempty"`
	RepoName           string                           `json:"repo_name,omitempty"`
	Path               string                           `json:"path,omitempty"`
	BranchName         string                           `json:"branch_name,omitempty"`
	RemoteName         string                           `json:"remote_name,omitempty"`
	LoadFromDir        bool                             `json:"load_from_dir,omitempty"`
	Commit             *models.Commit                   `json:"commit,omitempty"`
}

type ServiceReference struct {
	ProjectName string `json:"project_name"`
	ServiceName string `json:"service_name"`
	Production  bool   `json:"production"`
}

type BuildTemplateReference struct {
	BuildName     string   `json:"build_name"`
	ProjectName   string   `json:"project_name"`
	ServiceModule []string `json:"service_module"`
}

type ScanningTemplateReference struct {
	ScanningName string `json:"scanning_name"`
	ScanningID   string `json:"id"`
	DisplayName  string `json:"display_name"`
	ProjectName  string `json:"project_name"`
}
