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

package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type CustomWorkflowTestReport struct {
	ID               primitive.ObjectID `bson:"_id,omitempty"`
	WorkflowName     string             `bson:"workflow_name"`
	JobName          string             `bson:"job_name"`
	JobTaskName      string             `bson:"job_task_name"`
	TaskID           int64              `bson:"task_id"`
	ServiceName      string             `bson:"service_name"`
	ServiceModule    string             `bson:"service_module"`
	ZadigTestName    string             `bson:"zadig_test_name"`
	ZadigTestProject string             `bson:"zadig_test_project"`
	TestName         string             `bson:"test_name"`
	TestCaseNum      int                `bson:"test_case_num"`
	SuccessCaseNum   int                `bson:"success_case_num"`
	SkipCaseNum      int                `bson:"skip_case_num"`
	FailedCaseNum    int                `bson:"failed_case_num"`
	ErrorCaseNum     int                `bson:"error_case_num"`
	TestTime         float64            `bson:"test_time"`
	TestCases        []TestCase         `bson:"test_cases"`
}

func (CustomWorkflowTestReport) TableName() string {
	return "customer_workflow_test_report"
}
