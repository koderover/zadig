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

package step

type StepJunitReportSpec struct {
	SourceWorkflow string `bson:"source_workflow"           json:"source_workflow"                   yaml:"source_workflow"`
	// no stage name is recorded since the job name is unique, for now (version 2.1.0)
	SourceJobKey  string `bson:"source_job_key"             json:"source_job_key"                    yaml:"source_job_key"`
	JobTaskName   string `bson:"job_task_name"              json:"job_task_name"                     yaml:"job_task_name"`
	TaskID        int64  `bson:"task_id"                    json:"task_id"                           yaml:"task_id"`
	ServiceName   string `bson:"service_name"               json:"service_name"                      yaml:"service_name"`
	ServiceModule string `bson:"service_module"             json:"service_module"                    yaml:"service_module"`
	ReportDir     string `bson:"report_dir"                 json:"report_dir"                        yaml:"report_dir"`
	DestDir       string `bson:"dest_dir"                   json:"dest_dir"                          yaml:"dest_dir"`
	S3DestDir     string `bson:"s3_dest_dir"                json:"s3_dest_dir"                       yaml:"s3_dest_dir"`
	FileName      string `bson:"file_name"                  json:"file_name"                         yaml:"file_name"`
	TestName      string `bson:"test_name"                  json:"test_name"                         yaml:"test_name"`
	TestProject   string `bson:"test_project"               json:"test_project"                      yaml:"test_project"`
	S3Storage     *S3    `bson:"s3_storage"                 json:"s3_storage"                        yaml:"s3_storage"`
}
