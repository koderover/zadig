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

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/types"
)

type OpenAPIPageParamsFromReq struct {
	PageNum  int64 `json:"page_num"  form:"pageNum,default=1"`
	PageSize int64 `json:"page_size" form:"pageSize,default=20"`
}

type OpenAPIBuildCreationReq struct {
	Name            string                           `json:"name"`
	Description     string                           `json:"description"`
	ProjectName     string                           `json:"project_key"`
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
		return false, fmt.Errorf("project key cannot be empty")
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
	ProjectName    string                           `json:"project_key"`
	TemplateName   string                           `json:"template_name"`
	TargetServices []*types.OpenAPIServiceBuildArgs `json:"target_services"`
}

func (req *OpenAPIBuildCreationFromTemplateReq) Validate() (bool, error) {
	if req.Name == "" {
		return false, fmt.Errorf("build name cannot be empty")
	}
	if req.ProjectName == "" {
		return false, fmt.Errorf("project key cannot be empty")
	}
	if req.TemplateName == "" {
		return false, fmt.Errorf("template name cannot be empty")
	}
	if len(req.TargetServices) < 1 {
		return false, fmt.Errorf("target_service must have at least one item in it")
	}
	return true, nil
}

type OpenAPIBuildListResp struct {
	Total  int64                `json:"total"`
	Builds []*OpenAPIBuildBrief `json:"builds"`
}

type OpenAPIBuildBrief struct {
	Name           string           `json:"name"`
	ProjectName    string           `json:"project_key"`
	Source         string           `json:"source"`
	UpdateBy       string           `json:"update_by"`
	UpdateTime     int64            `json:"update_time"`
	TargetServices []*ServiceModule `json:"target_services"`
}

type ServiceModule struct {
	ServiceName   string `json:"service_name"`
	ServiceModule string `json:"service_module"`
}

type OpenAPIBuildDetailResp struct {
	ProjectName     string                        `json:"project_key"`
	Name            string                        `json:"name"`
	Source          string                        `json:"source"`
	TargetServices  []*ServiceModule              `json:"target_services"`
	TemplateName    string                        `json:"template_name"`
	UpdateBy        string                        `json:"update_by"`
	UpdateTime      int64                         `json:"update_time"`
	Repos           []*OpenAPIRepo                `json:"repos"`
	BuildEnv        *OpenAPIBuildEnv              `json:"build_env"`
	AdvancedSetting *types.OpenAPIAdvancedSetting `json:"advanced_settings"`
	BuildScript     string                        `json:"build_script"`
	Parameters      []*commonmodels.ServiceKeyVal `json:"parameters"`
	Outputs         []*commonmodels.Output        `json:"outputs"`
	PostBuild       *commonmodels.PostBuild       `json:"post_build"`
}

type OpenAPIRepo struct {
	Source       string `json:"source"`
	RepoOwner    string `json:"repo_owner"`
	RepoName     string `json:"repo_name"`
	Branch       string `json:"branch"`
	RemoteName   string `json:"remote_name"`
	CheckoutPath string `json:"checkout_path"`
	Submodules   bool   `json:"submodules"`
	Hidden       bool   `json:"hidden"`
}

type OpenAPIBuildEnv struct {
	BasicImageID    string               `json:"basic_image_id"`
	BasicImageLabel string               `json:"basic_image_label"`
	Installs        []*commonmodels.Item `json:"installs"`
}
