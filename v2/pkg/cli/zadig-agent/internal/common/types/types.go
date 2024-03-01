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

package types

import (
	"fmt"
	"strings"

	"github.com/koderover/zadig/v2/pkg/setting"
)

type ZadigJobTask struct {
	ID            string   `bson:"_id"                    json:"id"`
	ProjectName   string   `bson:"project_name"           json:"project_name"`
	WorkflowName  string   `bson:"workflow_name"          json:"workflow_name"`
	TaskID        int64    `bson:"task_id"                json:"task_id"`
	JobOriginName string   `bson:"job_origin_name"        json:"job_origin_name"`
	JobName       string   `bson:"job_name"               json:"job_name"`
	Status        string   `bson:"status"                 json:"status"`
	VMID          string   `bson:"vm_id"                  json:"vm_id"`
	StartTime     int64    `bson:"start_time"             json:"start_time"`
	EndTime       int64    `bson:"end_time"               json:"end_time"`
	Error         string   `bson:"error"                  json:"error"`
	VMLabels      []string `bson:"vm_labels"              json:"vm_labels"`
	VMName        []string `bson:"vm_name"                json:"vm_name"`
	JobCtx        string   `bson:"job_ctx"                json:"job_ctx"`
}

type Step struct {
	Name      string      `json:"name"`
	StepType  string      `json:"type"`
	Onfailure bool        `json:"on_failure"`
	Spec      interface{} `json:"spec"`
}

type EnvVar []string

type ReportJobParameters struct {
	Seq       int    `json:"seq"`
	Token     string `json:"token"`
	JobID     string `json:"job_id"`
	JobStatus string `json:"job_status"`
	JobLog    string `json:"job_log"`
	JobError  string `json:"job_error"`
	JobOutput []byte `json:"job_output"`
}

func GetJobOutputKey(key, outputName string) string {
	return fmt.Sprintf(setting.RenderValueTemplate, strings.Join([]string{"job", key, "output", outputName}, "."))
}

type ReportAgentJobResp struct {
	JobID     string `json:"job_id"`
	JobStatus string `json:"job_status"`
}

type AgentWorkDirs struct {
	WorkDir       string
	Workspace     string
	JobLogPath    string
	JobOutputsDir string
	JobScriptDir  string
	CacheDir      string
}
