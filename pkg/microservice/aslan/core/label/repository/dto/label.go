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

package dto

type LabelFilter struct {
	Key    string   `json:"key"`
	Values []string `json:"values"`
}

type ResourceLabel struct {
	ResourceID   string  `json:"resource_id"`
	ResourceType string  `json:"resource_type"`
	Labels       []Label `json:"labels"`
}

type Label struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type LabelResource struct {
	Key        string `json:"key"`
	Value      string `json:"value"`
	ResourceID string `json:"resource_id"`
}
