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

package vm

import (
	"github.com/koderover/zadig/v2/pkg/types/job"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type VMJob struct {
	ID            primitive.ObjectID `bson:"_id,omitempty"          json:"id,omitempty"`
	ProjectName   string             `bson:"project_name"           json:"project_name"`
	WorkflowName  string             `bson:"workflow_name"          json:"workflow_name"`
	TaskID        int64              `bson:"task_id"                json:"task_id"`
	JobOriginName string             `bson:"job_origin_name"        json:"job_origin_name"`
	JobName       string             `bson:"job_name"               json:"job_name"`
	JobType       string             `bson:"job_type"               json:"job_type"`
	Status        string             `bson:"status"                 json:"status"`
	IsDeleted     bool               `bson:"is_deleted"             json:"is_deleted"`
	VMID          string             `bson:"vm_id"                  json:"-"`
	CreatedTime   int64              `bson:"created_time"           json:"created_time"`
	StartTime     int64              `bson:"start_time"             json:"-"`
	EndTime       int64              `bson:"end_time"               json:"-"`
	Error         string             `bson:"error"                  json:"-"`
	VMLabels      []string           `bson:"vm_labels"              json:"-"`
	VMName        []string           `bson:"vm_name"                json:"-"`
	JobCtx        string             `bson:"job_ctx"                json:"job_ctx"`
	LogFile       string             `bson:"log_file"               json:"log_file"`
	Outputs       []*job.JobOutput   `bson:"outputs"                json:"outputs"`
}

type ReportJobParameters struct {
	JobID     string
	Status    string
	Error     error
	Log       string
	StartTime int64
	EndTime   int64
}

func (VMJob) TableName() string {
	return "vm_job"
}
