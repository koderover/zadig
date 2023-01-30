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

package types

// RenderSet ...
type RenderSet struct {
	// Name = EnvName == "" ? ProductTmpl : (EnvName + "-" + ProductTempl)
	Name     string `bson:"name"                     json:"name"`
	Revision int64  `bson:"revision"                 json:"revision"`
	// 可以为空，空时为产品模板默认的渲染集，非空时为环境的渲染集
	EnvName       string `bson:"env_name,omitempty"       json:"env_name,omitempty"`
	ProductTmpl   string `bson:"product_tmpl"             json:"product_tmpl"`
	Team          string `bson:"team,omitempty"           json:"team,omitempty"`
	UpdateTime    int64  `bson:"update_time"              json:"update_time"`
	UpdateBy      string `bson:"update_by"                json:"update_by"`
	IsDefault     bool   `bson:"is_default"               json:"is_default"`                          // 是否是默认配置
	DefaultValues string `bson:"default_values,omitempty"            json:"default_values,omitempty"` //环境默认变量 ›yaml content
	//KVs           []*RenderKV    `bson:"kvs,omitempty"            json:"kvs,omitempty"`
	ChartInfos  []*RenderChart `bson:"chart_infos,omitempty"    json:"chart_infos,omitempty"`
	Description string         `bson:"description,omitempty"    json:"description,omitempty"`
}
