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
	// 构建名称
	Name string `json:"name" binding:"required"`
	// 项目标识
	ProjectKey string `json:"project_key" binding:"required"`
	// 绑定的服务
	Services []*types.OpenAPIServiceWithModule `json:"services"`

	// 基础设施，kubernetes 为 kubernetes 集群，vm 为主机
	Infrastructure string `json:"infrastructure" binding:"required"`
	// 构建操作系统
	BuildOS string `json:"build_os" binding:"required"`
	// 依赖的软件包
	Installs []*types.OpenAPIToolItem `json:"installs"`

	// 代码信息，不支持 PR 和 Perforce 配置
	RepoInfo []*types.OpenAPIRepoInput `json:"repo_info"`
	// 自定义变量
	Parameters []*types.ParameterSetting `json:"parameters"`

	// 脚本类型
	ScriptType types.ScriptType `json:"script_type" binding:"required"`
	// 脚本内容
	BuildScript string `json:"build_script"`

	// 镜像构建步骤
	DockerBuildStep *types.OpenAPIDockerBuildStep `json:"docker_build_step"`

	// 高级设置
	AdvancedSetting *types.OpenAPIAdvancedSetting `json:"advanced_settings"`
}

func (req *OpenAPIBuildCreationReq) Validate() (bool, error) {
	if req.Name == "" {
		return false, fmt.Errorf("build name cannot be empty")
	}
	if req.ProjectKey == "" {
		return false, fmt.Errorf("project key cannot be empty")
	}
	return true, nil
}

type OpenAPIBuildCreationFromTemplateReq struct {
	// 构建名称
	Name string `json:"name" binding:"required"`
	// 项目标识
	ProjectKey string `json:"project_key" binding:"required"`
	// 模版名称
	TemplateName string `json:"template_name" binding:"required"`
	// 绑定的服务
	TargetServices []*types.OpenAPIServiceBuildArgs `json:"target_services" binding:"required"`
}

func (req *OpenAPIBuildCreationFromTemplateReq) Validate() (bool, error) {
	if req.Name == "" {
		return false, fmt.Errorf("build name cannot be empty")
	}
	if req.ProjectKey == "" {
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
	Name           string                            `json:"name"`
	ProjectName    string                            `json:"project_key"`
	Source         string                            `json:"source"`
	UpdateBy       string                            `json:"update_by"`
	UpdateTime     int64                             `json:"update_time"`
	TargetServices []*commonmodels.ServiceWithModule `json:"target_services"`
}

type OpenAPIBuildDetailResp struct {
	ProjectName     string                            `json:"project_key"`
	Name            string                            `json:"name"`
	Source          string                            `json:"source"`
	TargetServices  []*commonmodels.ServiceWithModule `json:"target_services"`
	TemplateName    string                            `json:"template_name"`
	UpdateBy        string                            `json:"update_by"`
	UpdateTime      int64                             `json:"update_time"`
	Repos           []*OpenAPIRepo                    `json:"repos"`
	BuildEnv        *OpenAPIBuildEnv                  `json:"build_env"`
	AdvancedSetting *types.OpenAPIAdvancedSetting     `json:"advanced_settings"`
	BuildScript     string                            `json:"build_script"`
	Parameters      []*commonmodels.ServiceKeyVal     `json:"parameters"`
	Outputs         []*commonmodels.Output            `json:"outputs"`
	PostBuild       *commonmodels.PostBuild           `json:"post_build"`
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
