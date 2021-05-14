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

package service

type ProductResp struct {
	ID          string      `json:"id"`
	ProductName string      `json:"product_name"`
	Namespace   string      `json:"namespace"`
	Status      string      `json:"status"`
	Error       string      `json:"error"`
	EnvName     string      `json:"env_name"`
	UpdateBy    string      `json:"update_by"`
	UpdateTime  int64       `json:"update_time"`
	Services    [][]string  `json:"services"`
	Render      *RenderInfo `json:"render"`
	Vars        []*RenderKV `json:"vars"`
	IsPublic    bool        `json:"isPublic"`
	ClusterId   string      `json:"cluster_id,omitempty"`
	RecycleDay  int         `json:"recycle_day"`
	IsProd      bool        `json:"is_prod"`
	Source      string      `json:"source"`
}

// RenderInfo ...
type RenderInfo struct {
	Name        string `json:"name"`
	Revision    int64  `json:"revision"`
	ProductTmpl string `json:"product_tmpl"`
	Descritpion string `json:"description"`
}

// RenderKV ...
type RenderKV struct {
	Key      string   `json:"key"`
	Value    string   `json:"value"`
	Alias    string   `json:"alias"`
	State    KeyState `json:"state"`
	Services []string `json:"services"`
}

// KeyState can be new, deleted, normal
type KeyState string

const (
	// KeyStateNew ...
	KeyStateNew = KeyState("new")
	// KeyStateUnused ...
	KeyStateUnused = KeyState("unused")
	// KeyStatePresent ...
	KeyStatePresent = KeyState("present")
)

type EndpointResponse struct {
	ResultCode int    `json:"resultCode"`
	ErrorMsg   string `json:"errorMsg"`
}
