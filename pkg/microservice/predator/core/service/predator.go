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

package service

// Context ...
type Context struct {
	JobType        string            `yaml:"job_type"`
	DockerRegistry *DockerRegistry   `yaml:"docker_registry"`
	DockerBuildCtx *DockerBuildCtx   `yaml:"build_ctx"`
	OnSetup        string            `yaml:"setup,omitempty"`
	ReleaseImages  []RepoImage       `yaml:"release_images"`
	DistributeList []*DistributeInfo `yaml:"distribute_info"`
}

type RepoImage struct {
	//RepoID    string `json:"repo_id"   bson:"repo_id"`
	Name      string `yaml:"name"`
	Username  string `yaml:"username"`
	Password  string `yaml:"password"`
	Host      string `yaml:"host"`
	Namespace string `yaml:"namespace"`
}

// DistributeInfo will be convert into yaml, adding yaml
type DistributeInfo struct {
	Image               string `yaml:"image"`
	ReleaseName         string `yaml:"release_name"`
	DistributeStartTime int64  `yaml:"distribute_start_time"`
	DistributeEndTime   int64  `yaml:"distribute_end_time"`
	DistributeStatus    string `yaml:"distribute_status"`
	DeployEnabled       bool   `yaml:"deploy_enabled"`
	DeployEnv           string `yaml:"deploy_env"`
	DeployNamespace     string `yaml:"deploy_namespace"`
	DeployStartTime     int64  `yaml:"deploy_start_time"`
	DeployEndTime       int64  `yaml:"deploy_end_time"`
	DeployStatus        string `yaml:"deploy_status"`
	RepoID              string `yaml:"repo_id"`
	RepoAK              string `yaml:"repo_ak"`
	RepoSK              string `yaml:"repo_sk"`
}

// DockerBuildCtx ...
// Docker build参数
// WorkDir: docker build执行路径
// DockerFile: dockerfile名称, 默认为Dockerfile
// ImageBuild: build image镜像全称, e.g. xxx.com/release-candidates/image:tag
type DockerBuildCtx struct {
	WorkDir         string `yaml:"work_dir"`
	DockerFile      string `yaml:"docker_file"`
	ImageName       string `yaml:"image_name"`
	BuildArgs       string `yaml:"build_args"`
	ImageReleaseTag string `yaml:"image_release_tag,omitempty"`
}

//DockerRegistry  registry host/user/password
type DockerRegistry struct {
	Host      string `yaml:"host"`
	UserName  string `yaml:"username"`
	Password  string `yaml:"password"`
	Namespace string `yaml:"namespace"`
}

// GetDockerFile ...
func (buildCtx *DockerBuildCtx) GetDockerFile() string {
	if buildCtx.DockerFile == "" {
		return "Dockerfile"
	}
	return buildCtx.DockerFile
}
