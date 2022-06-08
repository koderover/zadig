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

	"github.com/koderover/zadig/pkg/setting"
)

type StepDockerBuildSpec struct {
	Source                string          `yaml:"source"`
	WorkDir               string          `yaml:"work_dir"`
	RegistryHost          string          `yaml:"registry_host"`
	DockerFile            string          `yaml:"docker_file"`
	ImageName             string          `yaml:"image_name"`
	BuildArgs             string          `yaml:"build_args"`
	ImageReleaseTag       string          `yaml:"image_release_tag"`
	DockerTemplateContent string          `yaml:"docker_template_content"`
	Proxy                 *Proxy          `yaml:"proxy"`
	IgnoreCache           bool            `yaml:"ignore_cache"`
	DockerRegistry        *DockerRegistry `yaml:"docker_registry"`
}

type DockerRegistry struct {
	Host      string `yaml:"host"`
	Namespace string `yaml:"namespace"`
	UserName  string `yaml:"username"`
	Password  string `yaml:"password"`
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
