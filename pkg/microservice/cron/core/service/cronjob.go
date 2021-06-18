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

type CronjobPayload struct {
	Name        string      `json:"name"`
	ProductName string      `json:"product_name"`
	Action      string      `json:"action"`
	JobType     string      `json:"job_type"`
	DeleteList  []string    `json:"delete_list,omitempty"`
	JobList     []*Schedule `json:"job_list,omitempty"`
}

type Cronjob struct {
	ID           string            `json:"_id,omitempty"`
	Name         string            `json:"name"`
	Type         string            `json:"type"`
	Number       uint64            `json:"number"`
	Frequency    string            `json:"frequency"`
	Time         string            `json:"time"`
	Cron         string            `json:"cron"`
	ProductName  string            `json:"product_name,omitempty"`
	MaxFailure   int               `json:"max_failures,omitempty"`
	TaskArgs     *TaskArgs         `json:"task_args,omitempty"`
	WorkflowArgs *WorkflowTaskArgs `json:"workflow_args,omitempty"`
	TestArgs     *TestTaskArgs     `json:"test_args,omitempty"`
	JobType      string            `json:"job_type"`
	Enabled      bool              `json:"enabled"`
}

// param type: cronjob的执行内容类型
// param name: 当type为workflow的时候 代表workflow名称， 当type为test的时候，为test名称
type DisableCronjobReq struct {
	Name string `json:"name"`
	Type string `json:"type"`
}
