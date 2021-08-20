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

package aslan

type ListEnvsResp struct {
	Id          string     `json:"id"`
	ProductName string     `json:"product_name"`
	Namespace   string     `json:"namespace"`
	Status      string     `json:"status"`
	Error       string     `json:"error"`
	EnvName     string     `json:"env_name"`
	UpdateBy    string     `json:"update_by"`
	UpdateTime  int        `json:"update_time"`
	Services    []string   `json:"services"`
	Render      RenderInfo `json:"render"`
	Vars        []string   `json:"vars"`
	IsPubulic   bool       `json:"isPubulic"`
	ClusterId   string     `json:"cluster_id"`
	RecycleDay  int        `json:"recycle_day"`
	IsProd      bool       `json:"is_prod"`
	Source      string     `json:"source"`
}

type RenderInfo struct {
	Name        string `json:"name"`
	Revision    int    `json:"revision"`
	ProductTmpl string `json:"product_tmpl"`
	Description string `json:"description"`
}

type EnvProjectDetail struct {
	ServiceName string          `json:"service_name"`
	Scales      []ProjectScales `json:"scales"`
	EnvName     string          `json:"env_name"`
	ProductName string          `json:"product_name"`
	GroupName   string          `json:"group_name"`
}

type ProjectImages struct {
	Name   string `json:"name"`
	Images string `json:"images"`
}

type ProjectContainers struct {
	Name         string `json:"name"`
	Images       string `json:"images"`
	RestartCount int    `json:"restart_count"`
	Status       string `json:"status"`
	Message      string `json:"message"`
	Reason       string `json:"reason"`
	StartedAt    int    `json:"started_at"`
}

type ProjectPods struct {
	Kind       string              `json:"kind"`
	Name       string              `json:"name"`
	Status     string              `json:"status"`
	Age        string              `json:"age"`
	CreateTime int                 `json:"createtime"`
	Ip         string              `json:"ip"`
	Containers []ProjectContainers `json:"containers"`
	Replicas   int                 `json:"replicas"`
}

type ProjectScales struct {
	Name   string          `json:"name"`
	Type   string          `json:"type"`
	Images []ProjectImages `json:"images"`
	Pods   []ProjectPods   `json:"pods"`
}

type ServicesListResp struct {
	Services []ServiceDetail `json:"services"`
}

type ServiceDetail struct {
	ServiceName string   `json:"service_name"`
	Type        string   `json:"type"`
	Status      string   `json:"status"`
	Images      []string `json:"images"`
	ProductName string   `json:"product_name"`
	EnvName     string   `json:"env_name"`
	Ready       string   `json:"ready"`
}

type ServiceStatusListResp struct {
	ServiceName string `json:"service_name"`
	Status      string `json:"status"`
	Pod         string `json:"pod"`
	Container   string `json:"container"`
}

type ServiceStatus struct {
	ServiceName string `json:"service_name"`
	Status      string `json:"status"`
}
