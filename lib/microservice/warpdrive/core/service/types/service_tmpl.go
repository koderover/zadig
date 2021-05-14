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

package types

// Service : service template struct
// Service template config has 3 types mainly.
// 1. Kubernetes service, and yaml+config is held in spock: type == "k8s"; source == "spock"; yaml != ""
// 2. Kubernetes service, and yaml+config is held in gitlab: type == "k8s"; source == "gitlab"; src_path != ""
type ServiceTmpl struct {
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
	Descritpion      string           `bson:"description,omitempty"          json:"description,omitempty"`
	Visibility       string           `bson:"visibility,omitempty"           json:"visibility,omitempty"`
	Status           string           `bson:"status,omitempty"               json:"status,omitempty"`
	GerritRepoName   string           `bson:"gerrit_repo_name,omitempty"     json:"gerrit_repo_name,omitempty"`
	GerritBranchName string           `bson:"gerrit_branch_name,omitempty"   json:"gerrit_branch_name,omitempty"`
	GerritRemoteName string           `bson:"gerrit_remote_name,omitempty"   json:"gerrit_remote_name,omitempty"`
	GerritPath       string           `bson:"gerrit_path,omitempty"          json:"gerrit_path,omitempty"`
	GerritCodeHostID int              `bson:"gerrit_codeHost_id,omitempty"   json:"gerrit_codeHost_id,omitempty"`
	BuildName        string           `bson:"build_name,omitempty"           json:"build_name,omitempty"`
	HealthChecks     []*PmHealthCheck `bson:"health_checks,omitempty"        json:"health_checks,omitempty"`
	HelmChart        *HelmChart       `bson:"helm_chart,omitempty"           json:"helm_chart,omitempty"`
	EnvConfigs       []*EnvConfig     `bson:"env_configs,omitempty"          json:"env_configs,omitempty"`
	EnvStatuses      []*EnvStatus     `bson:"env_statuses,omitempty"         json:"env_statuses,omitempty"`
	CodehostID       int              `bson:"codehost_id,omitempty"          json:"codehost_id,omitempty"`
	RepoOwner        string           `bson:"repo_owner,omitempty"           json:"repo_owner,omitempty"`
	RepoName         string           `bson:"repo_name,omitempty"            json:"repo_name,omitempty"`
	BranchName       string           `bson:"branch_name,omitempty"          json:"branch_name,omitempty"`
	LoadPath         string           `bson:"load_path,omitempty"            json:"load_path,omitempty"`
	LoadFromDir      bool             `bson:"is_dir,omitempty"               json:"is_dir"`
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

type EnvStatus struct {
	HostID        string         `bson:"host_id,omitempty"           json:"host_id"`
	EnvName       string         `bson:"env_name,omitempty"          json:"env_name"`
	Address       string         `bson:"address,omitempty"           json:"address"`
	Status        string         `bson:"status,omitempty"            json:"status"`
	PmHealthCheck *PmHealthCheck `bson:"health_checks,omitempty"     json:"health_checks"`
}

type EnvConfig struct {
	EnvName string   `bson:"env_name,omitempty" json:"env_name"`
	HostIDs []string `bson:"host_ids,omitempty" json:"host_ids"`
}

type HelmChart struct {
	Name       string `bson:"name"               json:"name"`
	Version    string `bson:"version"     json:"version"`
	ValuesYaml string `bson:"values_yaml"        json:"values_yaml"`
}
