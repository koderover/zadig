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

package service

import (
	"fmt"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/types"
)

type OpenAPIBuildCreationReq struct {
	Name            string                           `json:"name"`
	Description     string                           `json:"description"`
	ProjectName     string                           `json:"project_name"`
	ImageName       string                           `json:"image_name"`
	RepoInfo        []*types.OpenAPIRepoInput        `json:"repo_info"`
	AdvancedSetting *types.OpenAPIAdvancedSetting    `json:"advanced_settings"`
	Addons          []*commonmodels.Item             `json:"addons"`
	TargetServices  []*types.OpenAPIServiceBuildArgs `json:"target_services"`
	Parameters      []*types.ParameterSetting        `json:"parameters"`
	DockerBuildInfo *types.DockerBuildInfo           `json:"docker_build_info"`
	BuildScript     string                           `json:"build_script"`
	PostBuildScript string                           `json:"post_build_script"`
	FileArchivePath string                           `json:"file_archive_path"`
}

func (req *OpenAPIBuildCreationReq) Validate() (bool, error) {
	if req.Name == "" {
		return false, fmt.Errorf("build name cannot be empty")
	}
	if req.ProjectName == "" {
		return false, fmt.Errorf("project name cannot be empty")
	}
	if req.ImageName == "" {
		return false, fmt.Errorf("image name cannot be empty")
	}
	if req.AdvancedSetting == nil {
		return false, fmt.Errorf("advanced settings must be provided")
	}
	return true, nil
}

type OpenAPIBuildCreationFromTemplateReq struct {
	Name           string                           `json:"name"`
	ProjectName    string                           `json:"project_name"`
	TemplateName   string                           `json:"template_name"`
	TargetServices []*types.OpenAPIServiceBuildArgs `json:"target_services"`
}

func (req *OpenAPIBuildCreationFromTemplateReq) Validate() (bool, error) {
	if req.Name == "" {
		return false, fmt.Errorf("build name cannot be empty")
	}
	if req.ProjectName == "" {
		return false, fmt.Errorf("project name cannot be empty")
	}
	if req.TemplateName == "" {
		return false, fmt.Errorf("template name cannot be empty")
	}
	if len(req.TargetServices) < 1 {
		return false, fmt.Errorf("target_service must have at least one item in it")
	}
	return true, nil
}
