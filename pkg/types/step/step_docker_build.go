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

package step

import (
	"fmt"

	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/types"
)

type StepDockerBuildSpec struct {
	Source                string              `bson:"source"                              json:"source"                                 yaml:"source"`
	WorkDir               string              `bson:"work_dir"                            json:"work_dir"                               yaml:"work_dir"`
	RegistryHost          string              `bson:"registry_host"                       json:"registry_host"                          yaml:"registry_host"`
	EnableBuildkit        bool                `bson:"enable_buildkit"                     json:"enable_buildkit"                        yaml:"enable_buildkit"`
	Platform              string              `bson:"platform"                            json:"platform"                               yaml:"platform"`
	BuildKitImage         string              `bson:"build_kit_image"                     json:"build_kit_image"                        yaml:"build_kit_image"`
	DockerFile            string              `bson:"docker_file"                         json:"docker_file"                            yaml:"docker_file"`
	ImageName             string              `bson:"image_name"                          json:"image_name"                             yaml:"image_name"`
	BuildArgs             string              `bson:"build_args"                          json:"build_args"                             yaml:"build_args"`
	ImageReleaseTag       string              `bson:"image_release_tag"                   json:"image_release_tag"                      yaml:"image_release_tag"`
	DockerTemplateContent string              `bson:"docker_template_content"             json:"docker_template_content"                yaml:"docker_template_content"`
	Proxy                 *Proxy              `bson:"proxy"                               json:"proxy"                                  yaml:"proxy"`
	IgnoreCache           bool                `bson:"ignore_cache"                        json:"ignore_cache"                           yaml:"ignore_cache"`
	DockerRegistry        *DockerRegistry     `bson:"docker_registry"                     json:"docker_registry"                        yaml:"docker_registry"`
	Repos                 []*types.Repository `bson:"repos"                               json:"repos"`
}

type DockerRegistry struct {
	DockerRegistryID string `bson:"docker_registry_id"                json:"docker_registry_id"                   yaml:"docker_registry_id"`
	Host             string `bson:"host"                              json:"host"                                 yaml:"host"`
	Namespace        string `bson:"namespace"                         json:"namespace"                            yaml:"namespace"`
	UserName         string `bson:"username"                          json:"username"                             yaml:"username"`
	Password         string `bson:"password"                          json:"password"                             yaml:"password"`
}

func (s *StepDockerBuildSpec) GetDockerFile() string {
	// if the source of the dockerfile is from template, we write our own dockerfile
	if s.Source == setting.DockerfileSourceTemplate {
		return fmt.Sprintf("/%s", setting.ZadigDockerfilePath)
	}
	if s.DockerFile == "" {
		return "Dockerfile"
	}
	return s.DockerFile
}
