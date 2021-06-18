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

type Pod struct {
	Kind              string            `json:"kind"`
	Name              string            `json:"name"`
	Status            string            `json:"status"`
	Age               string            `json:"age"`
	CreateTime        int64             `json:"createtime"`
	IP                string            `json:"ip"`
	Labels            map[string]string `json:"labels"`
	ContainerStatuses []Container       `json:"containers"`
}

type Container struct {
	Name         string `json:"name"`
	Image        string `json:"image"`
	RestartCount int32  `json:"restart_count"`
	Status       string `json:"status"`
	// Message regarding the last termination of the container
	Message string `json:"message"`
	// reason from the last termination of the container
	Reason string `json:"reason"`
	// Time at which previous execution of the container started
	StartedAt int64 `json:"started_at,omitempty"`
	// Time at which the container last terminated
	FinishedAt int64 `json:"finished_at,omitempty"`
}
