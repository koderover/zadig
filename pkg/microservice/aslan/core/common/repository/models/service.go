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

// Service : service template struct
// Service template config has 3 types mainly.
// 1. Kubernetes service, and yaml+config is held in aslan: type == "k8s"; source == "spock"; yaml != ""
// 2. Kubernetes service, and yaml+config is held in gitlab: type == "k8s"; source == "gitlab"; src_path != ""
type Service struct {
	ServiceName      string           `bson:"service_name"                   json:"service_name"`
	Type             string           `bson:"type"                           json:"type"`
	Team             string           `bson:"team,omitempty"                 json:"team,omitempty"`
	ProductName      string           `bson:"product_name"                   json:"product_name"`
	Revision         int64            `bson:"revision"                       json:"revision"`
	Source           string           `bson:"source,omitempty"               json:"source,omitempty"`
	GUIConfig        *GUIConfig       `bson:"gui_config,omitempty"           json:"gui_config,omitempty"`
	Yaml             string           `bson:"yaml,omitempty"                 json:"yaml"`
	SrcPath          string           `bson:"src_path,omitempty"             json:"src_path,omitempty"`
	Commit           *Commit          `bson:"commit,omitempty"               json:"commit,omitempty"`
	KubeYamls        []string         `bson:"-"                              json:"-"`
	Hash             string           `bson:"hash256,omitempty"              json:"hash256,omitempty"`
	CreateTime       int64            `bson:"create_time"                    json:"create_time"`
	CreateBy         string           `bson:"create_by"                      json:"create_by"`
	Containers       []*Container     `bson:"containers,omitempty"           json:"containers,omitempty"`
	Description      string           `bson:"description,omitempty"          json:"description,omitempty"`
	Visibility       string           `bson:"visibility,omitempty"           json:"visibility,omitempty"`
	Status           string           `bson:"status,omitempty"               json:"status,omitempty"`
	GerritRepoName   string           `bson:"gerrit_repo_name,omitempty"     json:"gerrit_repo_name,omitempty"`
	GerritBranchName string           `bson:"gerrit_branch_name,omitempty"   json:"gerrit_branch_name,omitempty"`
	GerritRemoteName string           `bson:"gerrit_remote_name,omitempty"   json:"gerrit_remote_name,omitempty"`
	GerritPath       string           `bson:"gerrit_path,omitempty"          json:"gerrit_path,omitempty"`
	GerritCodeHostID int              `bson:"gerrit_codeHost_id,omitempty"   json:"gerrit_codeHost_id,omitempty"`
	BuildName        string           `bson:"build_name,omitempty"           json:"build_name,omitempty"`
	HelmChart        *HelmChart       `bson:"helm_chart,omitempty"           json:"helm_chart,omitempty"`
	EnvConfigs       []*EnvConfig     `bson:"env_configs,omitempty"          json:"env_configs,omitempty"`
	EnvStatuses      []*EnvStatus     `bson:"env_statuses,omitempty"         json:"env_statuses,omitempty"`
	CodehostID       int              `bson:"codehost_id"                    json:"codehost_id"`
	RepoOwner        string           `bson:"repo_owner"                     json:"repo_owner"`
	RepoName         string           `bson:"repo_name"                      json:"repo_name"`
	RepoUUID         string           `bson:"repo_uuid,omitempty"            json:"repo_uuid,omitempty"`
	BranchName       string           `bson:"branch_name"                    json:"branch_name"`
	LoadPath         string           `bson:"load_path"                      json:"load_path"`
	LoadFromDir      bool             `bson:"is_dir"                         json:"is_dir"`
	HealthChecks     []*PmHealthCheck `bson:"health_checks,omitempty"        json:"health_checks,omitempty"`
}

type GUIConfig struct {
	Deployment interface{} `bson:"deployment,omitempty"           json:"deployment,omitempty"`
	Ingress    interface{} `bson:"ingress,omitempty"              json:"ingress,omitempty"`
	Service    interface{} `bson:"service,omitempty"              json:"service,omitempty"`
}

type YamlPreview struct {
	Kind string `bson:"-"           json:"kind"`
}

type YamlPreviewForPorts struct {
	Kind string `bson:"-"           json:"kind"`
	Spec *Spec  `bson:"-"           json:"spec"`
}

type Spec struct {
	Ports []Port `bson:"-"           json:"ports"`
}

type Port struct {
	Name string `bson:"-"           json:"name"`
	Port int    `bson:"-"           json:"port"`
}

// Commit ...
type Commit struct {
	SHA     string `bson:"sha"              json:"sha"`
	Message string `bson:"message"          json:"message"`
}

// Container ...
type Container struct {
	Name  string `bson:"name"           json:"name"`
	Image string `bson:"image"          json:"image"`
}

// ServiceTmplPipeResp ...router
type ServiceTmplPipeResp struct {
	ID               ServiceTmplRevision `bson:"_id"                            json:"_id"`
	Revision         int64               `bson:"revision"                       json:"revision"`
	SrcPath          string              `bson:"src_path"                       json:"src_path"`
	Visibility       string              `bson:"visibility,omitempty"           json:"visibility,omitempty"`
	Containers       []*Container        `bson:"containers,omitempty"           json:"containers,omitempty"`
	Source           string              `bson:"source"                         json:"source"`
	CodehostID       int                 `bson:"codehost_id"                    json:"codehost_id"`
	RepoOwner        string              `bson:"repo_owner"                     json:"repo_owner"`
	RepoName         string              `bson:"repo_name"                      json:"repo_name"`
	RepoUUID         string              `bson:"repo_uuid"                      json:"repo_uuid"`
	BranchName       string              `bson:"branch_name"                    json:"branch_name"`
	LoadPath         string              `bson:"load_path"                      json:"load_path"`
	LoadFromDir      bool                `bson:"is_dir"                         json:"is_dir"`
	GerritRemoteName string              `bson:"gerrit_remote_name,omitempty"   json:"gerrit_remote_name,omitempty"`
}

// ServiceTmplRevision ...
type ServiceTmplRevision struct {
	ProductName      string `bson:"product_name"                   json:"product_name"`
	ServiceName      string `bson:"service_name"                   json:"service_name"`
	Type             string `bson:"type"                           json:"type"`
	Revision         int64  `bson:"revision,omitempty"             json:"revision,omitempty"`
	Source           string `bson:"source"                         json:"source"`
	CodehostID       int    `bson:"codehost_id"                    json:"codehost_id"`
	RepoOwner        string `bson:"repo_owner"                     json:"repo_owner"`
	RepoName         string `bson:"repo_name"                      json:"repo_name"`
	BranchName       string `bson:"branch_name"                    json:"branch_name"`
	LoadPath         string `bson:"load_path"                      json:"load_path"`
	LoadFromDir      bool   `bson:"is_dir"                         json:"is_dir"`
	GerritRemoteName string `bson:"gerrit_remote_name,omitempty"   json:"gerrit_remote_name,omitempty"`
}

type HelmChart struct {
	Name       string `bson:"name"               json:"name"`
	Version    string `bson:"version"     json:"version"`
	ValuesYaml string `bson:"values_yaml"        json:"values_yaml"`
}

type HelmService struct {
	ProductName string       `json:"product_name"`
	Project     string       `json:"project"`
	Visibility  string       `json:"visibility"`
	Type        string       `json:"type"`
	CreateBy    string       `json:"create_by"`
	Revision    int64        `json:"revision"`
	HelmCharts  []*HelmChart `json:"helm_charts"`
}

type HelmServiceArgs struct {
	ProductName      string             `json:"product_name"`
	CreateBy         string             `json:"create_by"`
	HelmServiceInfos []*HelmServiceInfo `json:"helm_service_infos"`
}

type HelmServiceInfo struct {
	ServiceName string `json:"service_name"`
	FilePath    string `json:"file_path"`
	FileName    string `json:"file_name"`
	FileContent string `json:"file_content"`
}

type HelmServiceResp struct {
	ProductName   string `json:"product_name"`
	ServiceName   string `json:"service_name"`
	Type          string `json:"type"`
	Revision      int64  `json:"revision"`
	LatestVersion string `json:"latest_version"`
	ValuesYaml    string `json:"values_yaml"`
}

type HelmVersions struct {
	ServiceName       string `json:"service_name"`
	CurrentVersion    string `json:"current_version"`
	CurrentValuesYaml string `json:"current_values_yaml"`
	LatestVersion     string `json:"latest_version"`
	LatestValuesYaml  string `json:"latest_values_yaml"`
}

type HelmServiceRespArgs struct {
	HelmServices []*HelmServiceResp `json:"helm_services"`
}

type EnvStatus struct {
	HostID  string `bson:"host_id,omitempty"           json:"host_id"`
	EnvName string `bson:"env_name,omitempty"          json:"env_name"`
	Address string `bson:"address,omitempty"           json:"address"`
	Status  string `bson:"status,omitempty"            json:"status"`
}

type EnvConfig struct {
	EnvName string   `bson:"env_name,omitempty" json:"env_name"`
	HostIDs []string `bson:"host_ids,omitempty" json:"host_ids"`
}

type PmHealthCheck struct {
	Protocol            string `bson:"protocol,omitempty"              json:"protocol,omitempty"`
	Port                int    `bson:"port,omitempty"                  json:"port,omitempty"`
	Path                string `bson:"path,omitempty"                  json:"path,omitempty"`
	TimeOut             int64  `bson:"time_out,omitempty"              json:"time_out,omitempty"`
	Interval            uint64 `bson:"interval,omitempty"              json:"interval,omitempty"`
	HealthyThreshold    int    `bson:"healthy_threshold,omitempty"     json:"healthy_threshold,omitempty"`
	UnhealthyThreshold  int    `bson:"unhealthy_threshold,omitempty"   json:"unhealthy_threshold,omitempty"`
	CurrentHealthyNum   int    `bson:"current_healthy_num,omitempty"   json:"current_healthy_num,omitempty"`
	CurrentUnhealthyNum int    `bson:"current_unhealthy_num,omitempty" json:"current_unhealthy_num,omitempty"`
}

func (Service) TableName() string {
	return "template_service"
}
