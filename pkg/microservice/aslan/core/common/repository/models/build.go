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
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/types"
)

type Build struct {
	ID         primitive.ObjectID `bson:"_id,omitempty"                json:"id,omitempty"`
	Name       string             `bson:"name"                         json:"name"`
	Team       string             `bson:"team,omitempty"               json:"team,omitempty"`
	Source     string             `bson:"source,omitempty"             json:"source,omitempty"`
	TemplateID string             `bson:"template_id"                  json:"template_id"`
	Timeout    int                `bson:"timeout"                      json:"timeout"`
	// 在任一编译配置模板中只能出现一次
	// 对于k8s部署是传入容器名称
	// 对于物理机部署是服务名称
	Targets      []*ServiceModuleTarget `bson:"targets"                       json:"targets"`
	TargetRepos  []*TargetRepo          `bson:"-"                             json:"target_repos"`
	Description  string                 `bson:"desc,omitempty"                json:"desc"`
	UpdateTime   int64                  `bson:"update_time"                   json:"update_time"`
	UpdateBy     string                 `bson:"update_by"                     json:"update_by"`
	Repos        []*types.Repository    `bson:"repos"                         json:"repos"`
	PreBuild     *PreBuild              `bson:"pre_build"                     json:"pre_build"`
	JenkinsBuild *JenkinsBuild          `bson:"jenkins_build,omitempty"       json:"jenkins_build,omitempty"`
	Scripts      string                 `bson:"scripts"                       json:"scripts"`
	PostBuild    *PostBuild             `bson:"post_build,omitempty"          json:"post_build"`

	// TODO: Deprecated.
	Caches          []string `bson:"caches"                        json:"caches"`
	ProductName     string   `bson:"product_name"                  json:"product_name"`
	SSHs            []string `bson:"sshs"                          json:"sshs"`
	PMDeployScripts string   `bson:"pm_deploy_scripts"             json:"pm_deploy_scripts"`

	// New since V1.10.0.
	CacheEnable  bool               `bson:"cache_enable"   json:"cache_enable"`
	CacheDirType types.CacheDirType `bson:"cache_dir_type" json:"cache_dir_type"`
	CacheUserDir string             `bson:"cache_user_dir" json:"cache_user_dir"`
	// New since V1.10.0. Only to tell the webpage should the advanced settings be displayed
	AdvancedSettingsModified bool `bson:"advanced_setting_modified" json:"advanced_setting_modified"`
}

// PreBuild prepares an environment for a job
type PreBuild struct {
	// TODO: Deprecated.
	CleanWorkspace bool `bson:"clean_workspace"            json:"clean_workspace"`
	// ResReq defines job requested resources
	ResReq     setting.Request     `bson:"res_req"                json:"res_req"`
	ResReqSpec setting.RequestSpec `bson:"res_req_spec"           json:"res_req_spec"`
	// BuildOS defines job image OS, it supports 18.04 and 20.04
	BuildOS   string `bson:"build_os"                      json:"build_os"`
	ImageFrom string `bson:"image_from"                    json:"image_from"`
	ImageID   string `bson:"image_id"                      json:"image_id"`
	// Installs defines apps to be installed for build
	Installs []*Item `bson:"installs,omitempty"    json:"installs"`
	// Envs stores user defined env key val for build
	Envs []*KeyVal `bson:"envs,omitempty"              json:"envs"`
	// EnableProxy
	EnableProxy bool `bson:"enable_proxy,omitempty"        json:"enable_proxy"`
	// Parameters
	Parameters []*Parameter `bson:"parameters,omitempty"   json:"parameters"`
	// UploadPkg uploads package to s3
	UploadPkg bool   `bson:"upload_pkg"                      json:"upload_pkg"`
	ClusterID string `bson:"cluster_id"                      json:"cluster_id"`

	// TODO: Deprecated.
	Namespace string `bson:"namespace"                       json:"namespace"`
}

type BuildObj struct {
	Targets     []string
	Description string
	Repos       []*types.Repository
	PreBuild    *PreBuild
	Scripts     string
	MainFile    string
	PostBuild   *PostBuild
	Caches      []string
}

type PostBuild struct {
	DockerBuild         *DockerBuild         `bson:"docker_build,omitempty" json:"docker_build"`
	ObjectStorageUpload *ObjectStorageUpload `bson:"object_storage_upload"  json:"object_storage_upload"`
	FileArchive         *FileArchive         `bson:"file_archive,omitempty" json:"file_archive,omitempty"`
	Scripts             string               `bson:"scripts"                json:"scripts"`
}

type FileArchive struct {
	FileLocation string `bson:"file_location" json:"file_location"`
}

type ObjectStorageUpload struct {
	Enabled         bool                             `bson:"enabled"           json:"enabled"`
	ObjectStorageID string                           `bson:"object_storage_id" json:"object_storage_id"`
	UploadDetail    []*types.ObjectStoragePathDetail `bson:"upload_detail"     json:"upload_detail"`
}

type DockerBuild struct {
	// WorkDir docker run path
	WorkDir string `bson:"work_dir"                  json:"work_dir"`
	// DockerFile name, default is Dockerfile
	DockerFile string `bson:"docker_file"            json:"docker_file"`
	// BuildArgs docker build args
	BuildArgs string `bson:"build_args,omitempty"    json:"build_args"`
	// Source whether dockerfile comes from template or existing file
	Source string `bson:"source"                     json:"source"`
	// TemplateId is the id of the template dockerfile
	TemplateID string `bson:"template_id"            json:"template_id"`
	// TemplateName is the name of the template dockerfile
	TemplateName string `bson:"template_name"        json:"template_name"`
}

type JenkinsBuild struct {
	JenkinsID         string                     `bson:"jenkins_id"          json:"jenkins_id"`
	JobName           string                     `bson:"job_name"            json:"job_name"`
	JenkinsBuildParam []*types.JenkinsBuildParam `bson:"jenkins_build_param" json:"jenkins_build_params"`
}

type Parameter struct {
	Name         string      `bson:"name"                   json:"name"`
	DefaultValue string      `bson:"default_value"          json:"default_value"`
	ParamVal     []*ParamVal `bson:"param_val"              json:"param_val"`
}

// ParamVal 参数化过程服务配置值
type ParamVal struct {
	Target string `bson:"target"                 json:"target"`
	Value  string `bson:"value"                  json:"value"`
}

type ServiceModuleTarget struct {
	ProductName   string              `bson:"product_name"                  json:"product_name"`
	ServiceName   string              `bson:"service_name"                  json:"service_name"`
	ServiceModule string              `bson:"service_module"                json:"service_module"`
	BuildName     string              `bson:"build_name"                    json:"build_name"`
	Repos         []*types.Repository `bson:"repos,omitempty"               json:"repos,omitempty"`
	Envs          []*KeyVal           `bson:"envs,omitempty"                json:"envs"`
}

type ServiceModuleTargetBase struct {
	ProductName   string `json:"product_name"`
	ServiceName   string `json:"service_name"`
	ServiceModule string `json:"service_module"`
}

type TargetRepo struct {
	Service *ServiceModuleTargetBase `json:"service"`
	Repos   []*types.Repository      `json:"repos"`
	Envs    []*KeyVal                `json:"envs"`
}

type KeyVal struct {
	Key          string               `bson:"key"                       json:"key"                         yaml:"key"`
	Value        string               `bson:"value"                     json:"value"                       yaml:"value"`
	Type         ParameterSettingType `bson:"type,omitempty"            json:"type,omitempty"              yaml:"type"`
	ChoiceOption []string             `bson:"choice_option,omitempty"   json:"choice_option,omitempty"     yaml:"choice_option,omitempty"`
	IsCredential bool                 `bson:"is_credential"             json:"is_credential"               yaml:"is_credential"`
}

type Item struct {
	Name    string `bson:"name"                   json:"name"`
	Version string `bson:"version"                json:"version"`
}

func (build *Build) SafeRepos() []*types.Repository {
	if len(build.Repos) == 0 {
		return []*types.Repository{}
	}
	return build.Repos
}

func (build *Build) SafeReposDeepCopy() []*types.Repository {
	if len(build.Repos) == 0 {
		return []*types.Repository{}
	}
	resp := make([]*types.Repository, 0)
	for _, repo := range build.Repos {
		tmpRepo := *repo
		resp = append(resp, &tmpRepo)
	}
	return resp
}

func (Build) TableName() string {
	return "module_build"
}
