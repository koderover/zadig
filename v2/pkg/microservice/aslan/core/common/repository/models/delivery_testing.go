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

package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type DeliveryTest struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"                json:"id,omitempty"`
	ReleaseID   primitive.ObjectID `bson:"release_id"           json:"releaseId"`
	TestReports []TestReportObject `bson:"test_reports"         json:"testReports"`
	StartTime   int64              `bson:"start_time,omitempty" json:"start_time,omitempty"`
	EndTime     int64              `bson:"end_time,omitempty"   json:"end_time,omitempty"`
	CreatedAt   int64              `bson:"created_at"           json:"created_at"`
	DeletedAt   int64              `bson:"deleted_at"           json:"deleted_at"`
}

// TestReportObject ...
type TestReportObject struct {
	TestResultPath        string                  `bson:"test_result_path"       json:"testResultPath"`
	WorkflowName          string                  `bson:"workflow_name"          json:"workflowName"`
	TaskID                int64                   `bson:"task_id"                json:"taskId"`
	TestName              string                  `bson:"test_name"              json:"testName"`
	FunctionTestSuite     *TestSuite              `bson:"test_suite"             json:"testSuite"`
	PerformanceTestSuites []*PerformanceTestSuite `bson:"performance_test_suite" json:"performanceTestSuite"`
	StartTime             int64                   `bson:"start_time,omitempty"   json:"start_time,omitempty"`
	EndTime               int64                   `bson:"end_time,omitempty"     json:"end_time,omitempty"`
}

func (DeliveryTest) TableName() string {
	return "delivery_test"
}
