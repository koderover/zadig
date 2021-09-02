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

type Environment struct {
	ID          string      `json:"id"`
	ProductName string      `json:"product_name"`
	Namespace   string      `json:"namespace"`
	Status      string      `json:"status"`
	Error       string      `json:"error"`
	EnvName     string      `json:"env_name"`
	UpdateBy    string      `json:"update_by"`
	UpdateTime  int         `json:"update_time"`
	Services    [][]string  `json:"services"`
	Render      *RenderInfo `json:"render"`
	Vars        []*RenderKV `json:"vars,omitempty"`
	IsPublic    bool        `json:"IsPublic"`
	ClusterID   string      `json:"cluster_id,omitempty"`
	RecycleDay  int         `json:"recycle_day"`
	IsProd      bool        `json:"is_prod"`
	Source      string      `json:"source"`
}

type RenderInfo struct {
	Name        string `json:"name"`
	Revision    int    `json:"revision"`
	ProductTmpl string `json:"product_tmpl"`
	Description string `json:"description"`
}

type RenderKV struct {
	Key      string   `json:"key"`
	Value    string   `json:"value"`
	Alias    string   `json:"alias"`
	State    string   `json:"state"`
	Services []string `json:"services"`
}

type ServiceDetail struct {
	ServiceName string      `json:"service_name"`
	Scales      []*Workload `json:"scales"`
	EnvName     string      `json:"env_name"`
	ProductName string      `json:"product_name"`
	GroupName   string      `json:"group_name"`
}

type Workload struct {
	Name     string            `json:"name"`
	Type     string            `json:"type"`
	Images   []*ContainerImage `json:"images"`
	Pods     []*Pod            `json:"pods"`
	Replicas int32             `json:"replicas"`
}

type ContainerImage struct {
	Name  string `json:"name"`
	Image string `json:"image"`
}

type Pod struct {
	Kind              string            `json:"kind"`
	Name              string            `json:"name"`
	Status            string            `json:"status"`
	Age               string            `json:"age"`
	CreateTime        int64             `json:"createtime"`
	IP                string            `json:"ip"`
	Labels            map[string]string `json:"labels"`
	ContainerStatuses []*Container      `json:"containers"`
}

type Container struct {
	Name         string `json:"name"`
	Image        string `json:"image"`
	RestartCount int32  `json:"restart_count"`
	Status       string `json:"status"`
	Message      string `json:"message"`
	Reason       string `json:"reason"`
	StartedAt    int64  `json:"started_at,omitempty"`
	FinishedAt   int64  `json:"finished_at,omitempty"`
}

type ServicesResp struct {
	Services []*Service `json:"services"`
}

type Service struct {
	ServiceName string   `json:"service_name"`
	Type        string   `json:"type"`
	Status      string   `json:"status"`
	Images      []string `json:"images"`
	ProductName string   `json:"product_name"`
	EnvName     string   `json:"env_name"`
	Ready       string   `json:"ready"`
}

type ServiceStatus struct {
	ServiceName string `json:"service_name"`
	Status      string `json:"status"`
}
