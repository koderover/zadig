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

package config

type Stage struct {
	// 注意: 同一个stage暂时不能运行不同类型的Task
	TaskType    TaskType                          `bson:"type"               json:"type"`
	Status      Status                            `bson:"status"             json:"status"`
	RunParallel bool                              `bson:"run_parallel"       json:"run_parallel"`
	Desc        string                            `bson:"desc,omitempty"     json:"desc,omitempty"`
	SubTasks    map[string]map[string]interface{} `bson:"sub_tasks"          json:"sub_tasks"`
	AfterAll    bool                              `json:"after_all" bson:"after_all"`
}
