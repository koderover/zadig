/*
Copyright 2023 The KodeRover Authors.

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

import "github.com/koderover/zadig/pkg/util"

type OpenAPIStatV2 struct {
	Total        int64        `json:"total"`
	SuccessCount int64        `json:"success_count"`
	DailyStat    []*DailyStat `json:"daily_stat"`
}

type DailyStat struct {
	Date         string `json:"date"`
	Total        int64  `json:"total"`
	SuccessCount int64  `json:"success_count"`
	FailCount    int64  `json:"fail_count"`
}

type StatDashboardConfig struct {
	Type      string     `json:"type"`
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Source    string     `json:"source"`
	APIConfig *APIConfig `json:"api_config,omitempty"`
	Function  string     `json:"function"`
	Weight    int64      `json:"weight"`
}

type APIConfig struct {
	ExternalSystemId string           `json:"external_system_id"`
	ApiPath          string           `json:"api_path"`
	Queries          []*util.KeyValue `json:"queries"`
}

type StatDashboardByProject struct {
	ProjectKey  string               `json:"project_key"`
	ProjectName string               `json:"project_name"`
	Score       float64              `json:"score"`
	Facts       []*StatDashboardItem `json:"facts"`
}

type StatDashboardItem struct {
	Type     string      `json:"type"`
	ID       string      `json:"id"`
	Data     interface{} `json:"data"`
	Score    float64     `json:"score"`
	Error    string      `json:"error,omitempty"`
	HasValue bool        `json:"has_value"`
}

type StatDashboardBasicData struct {
	BuildTotal    int64 `json:"build_total"`
	BuildSuccess  int64 `json:"build_success"`
	TestTotal     int64 `json:"test_total"`
	TestSuccess   int64 `json:"test_success"`
	DeployTotal   int64 `json:"deploy_total"`
	DeploySuccess int64 `json:"deploy_success"`
}

// UserPromptParseInput use to parse user prompt
type UserPromptParseInput struct {
	ProjectList []string `json:"project_list"`
	JobList     []string `json:"job_list"`
	StartTime   int64    `json:"start_time"`
	EndTime     int64    `json:"end_time"`
}

type AiAnalysisReq struct {
	Prompt      string   `json:"prompt"`
	ProjectList []string `json:"project_list"`
	StartTime   int64    `json:"start_time"`
	EndTime     int64    `json:"end_time"`
}

type AiReqData struct {
	StartTime   int64          `json:"start_time"`
	EndTime     int64          `json:"end_time"`
	ProjectList []*ProjectData `json:"project_list"`
}

type ProjectData struct {
	ProjectName                    string      `json:"project_name"`
	ProjectDataDetail              *DataDetail `json:"project_data_detail"`
	SystemInternalEvaluationResult string      `json:"system_internal_evaluation_result"`
}

type DataDetail struct {
	BuildInfo   *BuildData   `json:"build_info"`
	DeployInfo  *DeployData  `json:"deploy_info"`
	TestInfo    *TestData    `json:"test_info"`
	ReleaseInfo *ReleaseData `json:"release_info"`
}

type BuildData struct {
	BuildTotal         int   `json:"build_total"`
	BuildSuccessTotal  int   `json:"build_success_total"`
	BuildFailureTotal  int   `json:"build_failure_total"`
	BuildTotalDuration int64 `json:"build_total_duration"`
}

type DeployData struct {
	DeployTotal         int   `json:"deploy_total"`
	DeploySuccessTotal  int   `json:"deploy_success_total"`
	DeployFailureTotal  int   `json:"deploy_failure_total"`
	DeployTotalDuration int64 `json:"deploy_total_duration"`
}

type TestData struct {
	TestTotal         int   `json:"test_total"`
	TestPass          int   `json:"test_pass_total"`
	TestFail          int   `json:"test_fail_total"`
	TestTotalDuration int64 `json:"test_total_duration"`
}

type ReleaseData struct {
	ReleaseTotal         int   `json:"release_total"`
	ReleaseSuccessTotal  int   `json:"release_success_total"`
	ReleaseFailureTotal  int   `json:"release_failure_total"`
	ReleaseTotalDuration int64 `json:"release_total_duration"`
}
