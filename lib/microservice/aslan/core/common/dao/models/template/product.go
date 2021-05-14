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

package template

import "strings"

// Vars不做保存，只做input参数
// product_name 当前项目主键
type Product struct {
	ProjectName    string          `bson:"project_name"              json:"project_name"`
	ProductName    string          `bson:"product_name"              json:"product_name"`
	Revision       int64           `bson:"revision"                  json:"revision"`
	CreateTime     int64           `bson:"create_time"               json:"create_time"`
	Teams          []*Team         `bson:"teams"                     json:"teams"`
	Team           string          `bson:"team"                      json:"team"`
	UpdateTime     int64           `bson:"update_time"               json:"update_time"`
	UpdateBy       string          `bson:"update_by"                 json:"update_by"`
	Enabled        bool            `bson:"enabled"                   json:"enabled"`
	Visibility     string          `bson:"visibility"                json:"visibility"`
	Timeout        int             `bson:"timeout,omitempty"         json:"timeout,omitempty"`
	Services       [][]string      `bson:"services"                  json:"services"`
	Vars           []*RenderKV     `bson:"vars"                      json:"vars"`
	EnvVars        []*EnvRenderKV  `bson:"-"                         json:"env_vars,omitempty"`
	ChartInfos     []*RenderChart  `bson:"-"                         json:"chart_infos,omitempty"`
	UserIDs        []int           `bson:"user_ids"                  json:"user_ids"`
	TeamID         int             `bson:"team_id"                   json:"team_id"`
	Description    string          `bson:"description,omitempty"     json:"desc,omitempty"`
	ProductFeature *ProductFeature `bson:"product_feature,omitempty" json:"product_feature,omitempty"`
	// onboarding状态，0表示onboarding完成，1、2、3、4代表当前onboarding所在的步骤
	OnboardingStatus int `bson:"onboarding_status"         json:"onboarding_status"`
	// CI场景的onboarding流程创建的ci工作流id，用于前端跳转
	CiPipelineId               string   `bson:"-"                         json:"ci_pipeline_id"`
	Role                       string   `bson:"-"                         json:"role,omitempty"`
	PermissionUUIDs            []string `bson:"-"                         json:"permissionUUIDs"`
	TotalServiceNum            int      `bson:"-"                         json:"total_service_num"`
	LatestServiceUpdateTime    int64    `bson:"-"                         json:"latest_service_update_time"`
	LatestServiceUpdateBy      string   `bson:"-"                         json:"latest_service_update_by"`
	TotalBuildNum              int      `bson:"-"                         json:"total_build_num"`
	LatestBuildUpdateTime      int64    `bson:"-"                         json:"latest_build_update_time"`
	LatestBuildUpdateBy        string   `bson:"-"                         json:"latest_build_update_by"`
	TotalTestNum               int      `bson:"-"                         json:"total_test_num"`
	LatestTestUpdateTime       int64    `bson:"-"                         json:"latest_test_update_time"`
	LatestTestUpdateBy         string   `bson:"-"                         json:"latest_test_update_by"`
	TotalEnvNum                int      `bson:"-"                         json:"total_env_num"`
	LatestEnvUpdateTime        int64    `bson:"-"                         json:"latest_env_update_time"`
	LatestEnvUpdateBy          string   `bson:"-"                         json:"latest_env_update_by"`
	TotalWorkflowNum           int      `bson:"-"                         json:"total_workflow_num"`
	LatestWorkflowUpdateTime   int64    `bson:"-"                         json:"latest_workflow_update_time"`
	LatestWorkflowUpdateBy     string   `bson:"-"                         json:"latest_workflow_update_by"`
	TotalEnvTemplateServiceNum int      `bson:"-"                         json:"total_env_template_service_num"`
	ShowProject                bool     `bson:"-"                         json:"show_project"`
	IsOpensource               bool     `bson:"is_opensource"             json:"is_opensource"`
}

type Team struct {
	Id   int    `bson:"id" json:"id"`
	Name string `bson:"name" json:"name"`
}

type RenderKV struct {
	Key      string   `bson:"key"               json:"key"`
	Value    string   `bson:"value"             json:"value"`
	Alias    string   `bson:"alias"             json:"alias"`
	State    string   `bson:"state"             json:"state"`
	Services []string `bson:"services"          json:"services"`
}

type EnvRenderKV struct {
	EnvName string      `json:"env_name"`
	Vars    []*RenderKV `json:"vars"`
}

// RenderChart ...
type RenderChart struct {
	ServiceName  string `bson:"service_name,omitempty"    json:"service_name,omitempty"`
	ChartVersion string `bson:"chart_version,omitempty"   json:"chart_version,omitempty"`
	// ChartProject string `bson:"chart_project,omitempty"   json:"chart_project,omitempty"`
	ValuesYaml string `bson:"values_yaml,omitempty"     json:"values_yaml,omitempty"`
}

type ProductFeature struct {
	// 方案，CI/CD 或者 k8s 或者 not_k8s
	KodeScheme string `bson:"kode_scheme"                  json:"kode_scheme"`
	// 基础设施，kubernetes 或者 cloud_host
	BasicFacility string `bson:"basic_facility"            json:"basic_facility"`
	// 部署方式，basic_facility=kubernetes时填写，k8s 或者 helm
	DeployType string `bson:"deploy_type"                  json:"deploy_type"`
	// 技术架构，micro_service 或者 monomer_application
	TechArch string `bson:"tech_arch"                      json:"tech_arch"`
	// 开发习惯，interface_mode 或者 yaml
	DevelopHabit string `bson:"develop_habit"              json:"develop_habit"`
}

type ForkProject struct {
	EnvName      string         `json:"env_name"`
	WorkflowName string         `json:"workflow_name"`
	ValuesYamls  []*RenderChart `json:"values_yamls"`
	ProductName  string         `json:"product_name"`
}

func (Product) TableName() string {
	return "template_product"
}

func (r *RenderKV) SetAlias() {
	r.Alias = "{{." + r.Key + "}}"
}

func (r *RenderKV) SetKeys() {
	key := r.Alias
	key = strings.Replace(key, "{{", "", -1)
	key = strings.Replace(key, "}}", "", -1)
	key = strings.TrimSpace(key)
	if strings.HasPrefix(key, ".") && len(key) >= 1 {
		key = key[1:]
	}
	r.Key = key
}

func (r *RenderKV) RemoveDupServices() {
	dic := make(map[string]bool)
	var result []string // 存放结果
	for _, key := range r.Services {
		if !dic[key] {
			dic[key] = true
			result = append(result, key)
		}
	}
	r.Services = result
}
