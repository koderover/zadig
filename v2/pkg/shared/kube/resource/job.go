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

package resource

import v1 "k8s.io/api/core/v1"

// Job ...
type Job struct {
	Name       string            `json:"name"`
	Age        string            `json:"age"`
	Labels     map[string]string `json:"labels"`
	CreateTime int64             `json:"create_time,omitempty"`
	Active     int32             `json:"active,omitempty"`
	Succeeded  int32             `json:"succeeded,omitempty"`
	Failed     int32             `json:"failed,omitempty"`
	Containers []v1.Container    `json:"-"`
}
