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

package task

import (
	"fmt"

	"github.com/koderover/zadig/lib/microservice/warpdrive/config"
	"github.com/koderover/zadig/lib/setting"
)

type Build struct {
	TaskType   config.TaskType `bson:"type"                       json:"type"`
	Enabled    bool            `bson:"enabled"                    json:"enabled"`
	TaskStatus config.Status   `bson:"status"                     json:"status"`
	// 新增一个service表示服务名称
	Service string `bson:"service"                    json:"service"`
	// 该名称实际为服务组件名称
	ServiceName       string               `bson:"service_name"               json:"service_name"`
	ServiceType       string               `bson:"service_type"               json:"service_type"`
	EnvName           string               `bson:"env_name"                   json:"env_name"`
	Namespace         string               `bson:"namespace"                  json:"namespace"`
	Timeout           int                  `bson:"timeout"                    json:"timeout,omitempty"`
	Error             string               `bson:"error,omitempty"            json:"error,omitempty"`
	StartTime         int64                `bson:"start_time"                 json:"start_time,omitempty"`
	EndTime           int64                `bson:"end_time"                   json:"end_time,omitempty"`
	JobCtx            JobCtx               `bson:"job_ctx"                    json:"job_ctx"`
	DockerBuild       *DockerBuild         `bson:"docker_build,omitempty"     json:"docker_build,omitempty"`
	InstallItems      []*Item              `bson:"install_items"              json:"install_items"`
	BuildOS           string               `bson:"build_os"                   json:"build_os,omitempty"`
	ImageFrom         string               `bson:"image_from"                 json:"image_from,omitempty"`
	ImageID           string               `bson:"image_id"                   json:"image_id"`
	ResReq            setting.Request      `bson:"res_req"                    json:"res_req"`
	LogFile           string               `bson:"log_file"                   json:"log_file"`
	InstallCtx        []*Install           `bson:"-"                          json:"install_ctx,omitempty"`
	Registries        []*RegistryNamespace `bson:"-"                   json:"registries"`
	StaticCheckStatus *StaticCheckStatus   `bson:"static_check_status,omitempty" json:"static_check_status,omitempty"`
	UTStatus          *UTStatus            `bson:"ut_status,omitempty" json:"ut_status,omitempty"`
	DockerBuildStatus *DockerBuildStatus   `bson:"docker_build_status,omitempty" json:"docker_build_status,omitempty"`
	BuildStatus       *BuildStatus         `bson:"build_status,omitempty" json:"build_status,omitempty"`
	IsRestart         bool                 `bson:"is_restart"                      json:"is_restart"`
}

type Item struct {
	Name    string `bson:"name"                   json:"name"`
	Version string `bson:"version"                json:"version"`
}

type Install struct {
	ObjectIdHex  string   `bson:"-"                      json:"-"`
	Name         string   `bson:"name"                   json:"name"`
	Version      string   `bson:"version"                json:"version"`
	Scripts      string   `bson:"scripts"                json:"scripts"`
	UpdateTime   int64    `bson:"update_time"            json:"update_time"`
	UpdateBy     string   `bson:"update_by"              json:"update_by"`
	Envs         []string `bson:"env"                    json:"env"`
	BinPath      string   `bson:"bin_path"               json:"bin_path"`
	Enabled      bool     `bson:"enabled"                json:"enabled"`
	DownloadPath string   `bson:"download_path"          json:"download_path"`
}

type RegistryNamespace struct {
	//ID               primitive.ObjectID `bson:"_id"                         json:"id"`
	OrgID            int    `bson:"org_id"                      json:"org_id"`
	RegAddr          string `bson:"reg_addr"                    json:"reg_addr"`
	RegType          string `bson:"reg_type"                    json:"reg_type"`
	RegProvider      string `bson:"reg_provider"                json:"reg_provider"`
	IsDefault        bool   `bson:"is_default"                  json:"is_default"`
	Namespace        string `bson:"namespace"                   json:"namespace"`
	AccessKey        string `bson:"access_key"                  json:"access_key"`
	SecretyKey       string `bson:"secret_key"                  json:"secret_key"`
	TencentSecretID  string `bson:"tencent_secret_id"           json:"tencent_secret_id"`
	TencentSecretKey string `bson:"tencent_secret_key"          json:"tencent_secret_key"`
	UpdateTime       int64  `bson:"update_time"                 json:"update_time"`
	UpdateBy         string `bson:"update_by"                   json:"update_by"`
}

type StepStatus struct {
	StartTime int64         `bson:"start_time"                 json:"start_time"`
	EndTime   int64         `bson:"end_time"                   json:"end_time"`
	Status    config.Status `bson:"status"                     json:"status"`
}

type BuildStatus struct {
	StepStatus
}

type StaticCheckStatus struct {
	StepStatus
	Repos []RepoStaticCheck `bson:"repos" json:"repos"`
}

type Repo struct {
	Source  string `bson:"source" json:"source"`
	Address string `bson:"address" json:"address"`
	Owner   string `bson:"owner" json:"owner"`
	Name    string `bson:"name" json:"name"`
}

type RepoStaticCheck struct {
	Repo
	SecurityMeasureCount int `bson:"security_measure_count" json:"security_measure_count"`
	IssueMeasureCount    int `bson:"issue_measure_count" json:"issue_measure_count"`
}

type UTStatus struct {
	StepStatus

	Repos []RepoCoverage `bson:"repos" json:"repos"`
}

type RepoCoverage struct {
	Repo

	NoStmt       int `bson:"no_stmt" json:"no_stmt"`
	NoMissedStmt int `bson:"no_missed_stmt" json:"no_missed_stmt"`
}

type DockerBuildStatus struct {
	StepStatus
	ImageName    string `bson:"image_name" json:"image_name"`
	RegistryRepo string `json:"registry_repo" json:"registry_repo"`
}

type JobCtx struct {
	EnableProxy    bool   `bson:"enable_proxy"                   json:"enable_proxy"`
	Proxy          *Proxy `bson:"proxy"                          json:"proxy"`
	CleanWorkspace bool   `bson:"clean_workspace"                json:"clean_workspace"`

	// BuildJobCtx
	Builds     []*Repository `bson:"builds"                         json:"builds"`
	BuildSteps []*BuildStep  `bson:"build_steps,omitempty"          json:"build_steps"`
	SSHs       []*SSH        `bson:"sshs,omitempty"                 json:"sshs"`
	// Envs stores user defined env key val for build
	// TODO: 之后可以不用keystore, 将用户敏感信息保存在此字段
	EnvVars     []*KeyVal `bson:"envs,omitempty"                 json:"envs"`
	UploadPkg   bool      `bson:"upload_pkg"                     json:"upload_pkg"`
	PackageFile string    `bson:"package_file,omitempty"         json:"package_file,omitempty"`
	Image       string    `bson:"image,omitempty"                json:"image,omitempty"`

	// TestJobCtx
	TestThreshold  int    `bson:"test_threshold"                 json:"test_threshold"`
	TestResultPath string `bson:"test_result_path,omitempty"     json:"test_result_path,omitempty"`
	TestJobName    string `bson:"test_job_name,omitempty"        json:"test_job_name,omitempty"`
	// DockerBuildCtx
	DockerBuildCtx *DockerBuildCtx `bson:"docker_build_ctx,omitempty" json:"docker_build_ctx,omitempty"`
	FileArchiveCtx *FileArchiveCtx `bson:"file_archive_ctx,omitempty" json:"file_archive_ctx,omitempty"`
	// TestType
	TestType string `bson:"test_type"                       json:"test_type"`
	// Caches
	Caches        []string `bson:"caches" json:"caches"`
	ArtifactPaths []string `bson:"artifact_paths,omitempty" json:"artifact_paths,omitempty"`
	IsHasArtifact bool     `bson:"is_has_artifact" json:"is_has_artifact"`
	// StorageUri is used for qbox release-candidates
	//StorageUri string `bson:"storage_uri,omitempty" json:"storage_uri,omitempty"`

	// ClassicBuild used by qbox build
	ClassicBuild bool   `bson:"classic_build"             json:"classic_build"`
	PostScripts  string `bson:"post_scripts,omitempty"    json:"post_scripts"`
}

type SSH struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	UserName   string `json:"user_name"`
	IP         string `json:"ip"`
	IsProd     bool   `json:"is_prod"`
	Label      string `json:"label"`
	PrivateKey string `json:"private_key"`
}

// DockerBuildCtx ...
// Docker build参数
// WorkDir: docker build执行路径
// DockerFile: dockerfile名称, 默认为Dockerfile
// ImageBuild: build image镜像全称, e.g. xxx.com/release-candidates/image:tag
type DockerBuildCtx struct {
	WorkDir         string `yaml:"work_dir" bson:"work_dir" json:"work_dir"`
	DockerFile      string `yaml:"docker_file" bson:"docker_file" json:"docker_file"`
	ImageName       string `yaml:"image_name" bson:"image_name" json:"image_name"`
	BuildArgs       string `yaml:"build_args" bson:"build_args" json:"build_args"`
	ImageReleaseTag string `yaml:"image_release_tag,omitempty" bson:"image_release_tag,omitempty" json:"image_release_tag"`
}

type FileArchiveCtx struct {
	FileLocation string `yaml:"file_location" bson:"file_location" json:"file_location"`
	FileName     string `yaml:"file_name" bson:"file_name" json:"file_name"`
	//StorageUri   string `yaml:"storage_uri" bson:"storage_uri" json:"storage_uri"`
}

// GetDockerFile ...
func (buildCtx *DockerBuildCtx) GetDockerFile() string {
	if buildCtx.DockerFile == "" {
		return "Dockerfile"
	}
	return buildCtx.DockerFile
}

type KeyVal struct {
	Key          string `bson:"key"                 json:"key"`
	Value        string `bson:"value"               json:"value"`
	IsCredential bool   `bson:"is_credential"       json:"is_credential"`
}

type Repository struct {
	// Source is github, gitlab
	Source        string `bson:"source,omitempty"          json:"source,omitempty"`
	RepoOwner     string `bson:"repo_owner"                json:"repo_owner"`
	RepoName      string `bson:"repo_name"                 json:"repo_name"`
	RemoteName    string `bson:"remote_name,omitempty"     json:"remote_name,omitempty"`
	Branch        string `bson:"branch"                    json:"branch"`
	PR            int    `bson:"pr,omitempty"              json:"pr,omitempty"`
	Tag           string `bson:"tag,omitempty"             json:"tag,omitempty"`
	CommitID      string `bson:"commit_id,omitempty"       json:"commit_id,omitempty"`
	CommitMessage string `bson:"commit_message,omitempty"  json:"commit_message,omitempty"`
	CheckoutPath  string `bson:"checkout_path,omitempty"   json:"checkout_path,omitempty"`
	SubModules    bool   `bson:"submodules,omitempty"      json:"submodules,omitempty"`
	// UseDefault defines if the repo can be configured in start pipeline task page
	UseDefault bool `bson:"use_default,omitempty"          json:"use_default,omitempty"`
	// IsPrimary used to generated image and package name, each build has one primary repo
	IsPrimary  bool `bson:"is_primary"                     json:"is_primary"`
	CodehostID int  `bson:"codehost_id"                    json:"codehost_id"`
	// add
	OauthToken  string `bson:"oauth_token"                  json:"oauth_token"`
	Address     string `bson:"address"                      json:"address"`
	AuthorName  string `bson:"author_name,omitempty"        json:"author_name,omitempty"`
	CheckoutRef string `bson:"checkout_ref,omitempty"       json:"checkout_ref,omitempty"`
}

type BuildStep struct {
	BuildType  string `bson:"type"                         json:"type"`
	Scripts    string `bson:"scripts"                      json:"scripts"`
	MainGoFile string `bson:"main,omitempty"               json:"main,omitempty"`
}

//type SSH struct {
//	ID         string `json:"id"`
//	Name       string `json:"name"`
//	UserName   string `json:"user_name"`
//	IP         string `json:"ip"`
//	IsProd     bool   `json:"is_prod"`
//	Label      string `json:"label"`
//	PrivateKey string `json:"private_key"`
//}

//type KeyVal struct {
//	Key          string `bson:"key"                 json:"key"`
//	Value        string `bson:"value"               json:"value"`
//	IsCredential bool   `bson:"is_credential"       json:"is_credential"`
//}

//const MaskValue = "********"

//// Repository struct
//type Repository struct {
//	// Source is github, gitlab
//	Source        string `bson:"source,omitempty"          json:"source,omitempty"`
//	RepoOwner     string `bson:"repo_owner"                json:"repo_owner"`
//	RepoName      string `bson:"repo_name"                 json:"repo_name"`
//	RemoteName    string `bson:"remote_name,omitempty"     json:"remote_name,omitempty"`
//	Branch        string `bson:"branch"                    json:"branch"`
//	PR            int    `bson:"pr,omitempty"              json:"pr,omitempty"`
//	Tag           string `bson:"tag,omitempty"             json:"tag,omitempty"`
//	CommitID      string `bson:"commit_id,omitempty"       json:"commit_id,omitempty"`
//	CommitMessage string `bson:"commit_message,omitempty"  json:"commit_message,omitempty"`
//	CheckoutPath  string `bson:"checkout_path,omitempty"   json:"checkout_path,omitempty"`
//	SubModules    bool   `bson:"submodules,omitempty"      json:"submodules,omitempty"`
//	// UseDefault defines if the repo can be configured in start pipeline task page
//	UseDefault bool `bson:"use_default,omitempty"          json:"use_default,omitempty"`
//	// IsPrimary used to generated image and package name, each build has one primary repo
//	IsPrimary  bool `bson:"is_primary"                     json:"is_primary"`
//	CodehostID int  `bson:"codehost_id"                    json:"codehost_id"`
//	// add
//	OauthToken  string `bson:"oauth_token"                  json:"oauth_token"`
//	Address     string `bson:"address"                      json:"address"`
//	AuthorName  string `bson:"author_name,omitempty"        json:"author_name,omitempty"`
//	CheckoutRef string `bson:"checkout_ref,omitempty"       json:"checkout_ref,omitempty"`
//}

// ToSubTask ...
func (b *Build) ToSubTask() (map[string]interface{}, error) {
	var task map[string]interface{}
	if err := IToi(b, &task); err != nil {
		return nil, fmt.Errorf("convert BuildTaskV2 to interface error: %v", err)
	}
	return task, nil
}
