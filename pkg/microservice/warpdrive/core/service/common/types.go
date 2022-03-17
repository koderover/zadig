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

package common

import "github.com/koderover/zadig/pkg/microservice/warpdrive/config"

type Stage struct {
	//Note: The same stage cannot temporarily run different types of tasks
	TaskType    config.TaskType                   `bson:"type"               json:"type"`
	Status      config.Status                     `bson:"status"             json:"status"`
	RunParallel bool                              `bson:"run_parallel"       json:"run_parallel"`
	Desc        string                            `bson:"desc,omitempty"     json:"desc,omitempty"`
	SubTasks    map[string]map[string]interface{} `bson:"sub_tasks"          json:"sub_tasks"`
	AfterAll    bool                              `bson:"after_all"          json:"after_all"`
}
