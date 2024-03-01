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

package ai

import "errors"

var (
	ReturnAnswerWrongFormat = errors.New("AI return wrong answer format")
)

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

type ReleaseData struct {
	Description string          `json:"data_description"`
	Details     *ReleaseDetails `json:"data_details"`
}

type ReleaseDetails struct {
	ReleaseTotal         int   `json:"release_total"`
	ReleaseSuccessTotal  int   `json:"release_success_total"`
	ReleaseFailureTotal  int   `json:"release_failure_total"`
	ReleaseTotalDuration int64 `json:"release_total_duration"`
}
