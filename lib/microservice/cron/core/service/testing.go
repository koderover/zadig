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

type TestingOpt struct {
	Name        string        `json:"name"`
	ProductName string        `json:"product_name"`
	Desc        string        `json:"desc"`
	UpdateTime  int64         `json:"update_time"`
	UpdateBy    string        `json:"update_by"`
	TestCaseNum int           `json:"test_case_num,omitempty"`
	ExecuteNum  int           `json:"execute_num,omitempty"`
	PassRate    float64       `json:"pass_rate,omitempty"`
	AvgDuration float64       `json:"avg_duration,omitempty"`
	Workflows   []*Workflow   `json:"workflows,omitempty"`
	Schedules   *ScheduleCtrl `json:"schedules,omitempty"`
}
