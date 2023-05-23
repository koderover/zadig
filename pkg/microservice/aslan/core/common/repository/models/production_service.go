package models

import (
	commontypes "github.com/koderover/zadig/pkg/microservice/aslan/core/common/types"
	"github.com/koderover/zadig/pkg/setting"
)

// Service : service template struct
// Service template config has 3 types mainly.
// 1. Kubernetes service, and yaml+config is held in aslan: type == "k8s"; source == "spock"; yaml != ""
// 2. Kubernetes service, and yaml+config is held in gitlab: type == "k8s"; source == "gitlab"; src_path != ""
type ProductionService struct {
	ServiceName        string                           `bson:"service_name"                   json:"service_name"`
	Type               string                           `bson:"type"                           json:"type"`
	Team               string                           `bson:"team,omitempty"                 json:"team,omitempty"`
	ProductName        string                           `bson:"product_name"                   json:"product_name"`
	Revision           int64                            `bson:"revision"                       json:"revision"`
	Source             string                           `bson:"source,omitempty"               json:"source,omitempty"`
	GUIConfig          *GUIConfig                       `bson:"gui_config,omitempty"           json:"gui_config,omitempty"`
	Yaml               string                           `bson:"yaml,omitempty"                 json:"yaml"`
	RenderedYaml       string                           `bson:"-"                              json:"-"`
	SrcPath            string                           `bson:"src_path,omitempty"             json:"src_path,omitempty"`
	Commit             *Commit                          `bson:"commit,omitempty"               json:"commit,omitempty"`
	KubeYamls          []string                         `bson:"-"                              json:"-"`
	Hash               string                           `bson:"hash256,omitempty"              json:"hash256,omitempty"`
	CreateTime         int64                            `bson:"create_time"                    json:"create_time"`
	CreateBy           string                           `bson:"create_by"                      json:"create_by"`
	Containers         []*Container                     `bson:"containers,omitempty"           json:"containers,omitempty"`
	Description        string                           `bson:"description,omitempty"          json:"description,omitempty"`
	Visibility         string                           `bson:"visibility,omitempty"           json:"visibility,omitempty"`
	Status             string                           `bson:"status,omitempty"               json:"status,omitempty"`
	GerritRepoName     string                           `bson:"gerrit_repo_name,omitempty"     json:"gerrit_repo_name,omitempty"`
	GerritBranchName   string                           `bson:"gerrit_branch_name,omitempty"   json:"gerrit_branch_name,omitempty"`
	GerritRemoteName   string                           `bson:"gerrit_remote_name,omitempty"   json:"gerrit_remote_name,omitempty"`
	GerritPath         string                           `bson:"gerrit_path,omitempty"          json:"gerrit_path,omitempty"`
	GerritCodeHostID   int                              `bson:"gerrit_codeHost_id,omitempty"   json:"gerrit_codeHost_id,omitempty"`
	GiteePath          string                           `bson:"gitee_path,omitempty"           json:"gitee_path,omitempty"`
	BuildName          string                           `bson:"build_name"                     json:"build_name"`
	VariableYaml       string                           `bson:"variable_yaml"                  json:"variable_yaml"`        // New since 1.16.0, stores the variable yaml of k8s services
	ServiceVariableKVs []*commontypes.ServiceVariableKV `bson:"service_variable_kvs"           json:"service_variable_kvs"` // New since 1.18.0, stores the variable kvs of k8s services
	ServiceVars        []string                         `bson:"service_vars"                   json:"service_vars"`         // DEPRECATED, New since 1.16.0, stores keys in variables which can be set in env
	HelmChart          *HelmChart                       `bson:"helm_chart,omitempty"           json:"helm_chart,omitempty"`
	EnvConfigs         []*EnvConfig                     `bson:"env_configs,omitempty"          json:"env_configs,omitempty"`
	EnvStatuses        []*EnvStatus                     `bson:"env_statuses,omitempty"         json:"env_statuses,omitempty"`
	ReleaseNaming      string                           `bson:"release_naming"                 json:"release_naming"`
	CodehostID         int                              `bson:"codehost_id,omitempty"          json:"codehost_id,omitempty"`
	RepoOwner          string                           `bson:"repo_owner,omitempty"           json:"repo_owner,omitempty"`
	RepoNamespace      string                           `bson:"repo_namespace,omitempty"       json:"repo_namespace,omitempty"`
	RepoName           string                           `bson:"repo_name,omitempty"            json:"repo_name,omitempty"`
	RepoUUID           string                           `bson:"repo_uuid,omitempty"            json:"repo_uuid,omitempty"`
	BranchName         string                           `bson:"branch_name,omitempty"          json:"branch_name,omitempty"`
	LoadPath           string                           `bson:"load_path,omitempty"            json:"load_path,omitempty"`
	LoadFromDir        bool                             `bson:"is_dir,omitempty"               json:"is_dir,omitempty"`
	CreateFrom         interface{}                      `bson:"create_from,omitempty"          json:"create_from,omitempty"`
	HealthChecks       []*PmHealthCheck                 `bson:"health_checks,omitempty"        json:"health_checks,omitempty"`
	WorkloadType       string                           `bson:"workload_type,omitempty"        json:"workload_type,omitempty"` // WorkloadType is set in host projects
	EnvName            string                           `bson:"env_name,omitempty"             json:"env_name,omitempty"`
	TemplateID         string                           `bson:"template_id,omitempty"          json:"template_id,omitempty"`
	AutoSync           bool                             `bson:"auto_sync"                      json:"auto_sync"`
}

func (svc *ProductionService) GetRepoNamespace() string {
	if svc.RepoNamespace != "" {
		return svc.RepoNamespace
	}
	return svc.RepoOwner
}

func (svc *ProductionService) GetReleaseNaming() string {
	if len(svc.ReleaseNaming) > 0 {
		return svc.ReleaseNaming
	}
	return setting.ReleaseNamingPlaceholder
}

func (ProductionService) TableName() string {
	return "production_template_service"
}
