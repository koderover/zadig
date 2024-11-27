/*
Copyright 2024 The KodeRover Authors.

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

package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type JobInfo struct {
	ID primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	// job type, this should be same as the job's type with some exceptions:
	Type string `bson:"type" json:"type"`
	// ProductName, WorkflowName, WorkflowDisplayName and TaskID marks the belongings of this job
	WorkflowName        string `bson:"workflow_name" json:"workflow_name"`
	WorkflowDisplayName string `bson:"workflow_display_name" json:"workflow_display_name"`
	TaskID              int64  `bson:"task_id" json:"task_id"`
	ProductName         string `bson:"product_name" json:"product_name"`
	// Status, StartTime, EndTime and Duration: basic information about the job
	Status    string `bson:"status" json:"status"`
	StartTime int64  `bson:"start_time" json:"start_time"`
	EndTime   int64  `bson:"end_time" json:"end_time"`
	Duration  int64  `bson:"duration" json:"duration"`
	// ServiceType, ServiceName and ServiceModule are used exclusively for build & deploy jobs
	ServiceType   string `bson:"service_type" json:"service_type"`
	ServiceName   string `bson:"service_name" json:"service_name"`
	ServiceModule string `bson:"service_module" json:"service_module"`
	// Production marks if this job is used for production environment
	// for now, this is only used for deploy jobs
	Production bool `bson:"production" json:"production"`
	// TargetEnv is the target environment for the deploy job
	TargetEnv string `bson:"target_env" json:"target_env"`
}

func (JobInfo) TableName() string {
	return "job_info"
}

type ServiceDeployCountWithStatus struct {
	Production  bool   `bson:"production"             json:"production"`
	ServiceName string `bson:"service_name"           json:"service_name"`
	ProductName string `bson:"product_name"           json:"project_key"`
	ProjectName string `bson:"project_name,omitempty" json:"project_name,omitempty"`
	Count       int    `bson:"count"                  json:"count"`
	Success     int    `bson:"success"                json:"success"`
	Failed      int    `bson:"failed"                 json:"failed"`
}
