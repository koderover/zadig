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
	"reflect"

	templatemodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	commontypes "github.com/koderover/zadig/pkg/microservice/aslan/core/common/types"
)

// RenderSet ...
type RenderSet struct {
	// Name = EnvName == "" ? ProductTmpl : (EnvName + "-" + ProductTempl)
	Name     string `bson:"name"                     json:"name"`
	Revision int64  `bson:"revision"                 json:"revision"`
	// 可以为空，空时为产品模板默认的渲染集，非空时为环境的渲染集
	EnvName     string `bson:"env_name,omitempty"             json:"env_name,omitempty"`
	ProductTmpl string `bson:"product_tmpl"                   json:"product_tmpl"`
	Team        string `bson:"team,omitempty"                 json:"team,omitempty"`
	UpdateTime  int64  `bson:"update_time"                    json:"update_time"`
	UpdateBy    string `bson:"update_by"                      json:"update_by"`
	IsDefault   bool   `bson:"is_default"                     json:"is_default"`
	// yaml content, used as 'global variables' for helm projects, DEPRECATED for k8s since 1.18.0
	DefaultValues string `bson:"default_values,omitempty"       json:"default_values,omitempty"`
	// current only used for k8s
	GlobalVariables  []*commontypes.GlobalVariableKV `bson:"global_variables,omitempty"     json:"global_variables,omitempty"` // new since 1.18.0 replace DefaultValues
	YamlData         *templatemodels.CustomYaml      `bson:"yaml_data,omitempty"            json:"yaml_data,omitempty"`
	ServiceVariables []*templatemodels.ServiceRender `bson:"service_variables,omitempty"    json:"service_variables,omitempty"` // new since 1.16.0 replace kvs
	ChartInfos       []*templatemodels.ServiceRender `bson:"chart_infos,omitempty"          json:"chart_infos,omitempty"`
	Description      string                          `bson:"description,omitempty"          json:"description,omitempty"`
}

func (RenderSet) TableName() string {
	return "render_set"
}

func (m *RenderSet) Diff(target *RenderSet) bool {
	//if m.IsDefault != target.IsDefault || reflect.DeepEqual(m.KVs, target.KVs) {
	if m.DefaultValues == target.DefaultValues {
		return false
	}
	return true
}

func (m *RenderSet) HelmRenderDiff(target *RenderSet) bool {
	return !m.Diff(target) || !reflect.DeepEqual(m.ChartInfos, target.ChartInfos)
}

func (m *RenderSet) K8sServiceRenderDiff(target *RenderSet) bool {
	return !m.Diff(target) || !reflect.DeepEqual(m.ServiceVariables, target.ServiceVariables)
}

func (m *RenderSet) GetServiceRenderMap() map[string]*templatemodels.ServiceRender {
	serviceRenderMap := make(map[string]*templatemodels.ServiceRender)
	for _, render := range m.ServiceVariables {
		serviceRenderMap[render.ServiceName] = render
	}
	return serviceRenderMap
}

func (m *RenderSet) GetChartRenderMap() map[string]*templatemodels.ServiceRender {
	serviceRenderMap := make(map[string]*templatemodels.ServiceRender)
	for _, render := range m.ChartInfos {
		serviceRenderMap[render.ServiceName] = render
	}
	return serviceRenderMap
}
