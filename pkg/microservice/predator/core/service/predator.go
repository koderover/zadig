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
	JobType        string          `yaml:"job_type"`
	DockerRegistry *DockerRegistry `yaml:"docker_registry"`
	DockerBuildCtx *DockerBuildCtx `yaml:"build_ctx"`
	OnSetup        string          `yaml:"setup,omitempty"`
	ReleaseImages  []RepoImage     `yaml:"release_images"`
}

type RepoImage struct {
	//RepoID    string `json:"repo_id"   bson:"repo_id"`
	Name      string `yaml:"name"`
	Username  string `yaml:"username"`
	Password  string `yaml:"password"`
	Host      string `yaml:"host"`
	Namespace string `yaml:"namespace"`
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
