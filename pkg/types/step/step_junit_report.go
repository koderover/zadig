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
	ReportDir string `bson:"report_dir"                 json:"report_dir"                        yaml:"report_dir"`
	DestDir   string `bson:"dest_dir"                   json:"dest_dir"                          yaml:"dest_dir"`
	S3DestDir string `bson:"s3_dest_dir"                json:"s3_dest_dir"                       yaml:"s3_dest_dir"`
	FileName  string `bson:"file_name"                  json:"file_name"                         yaml:"file_name"`
	TestName  string `bson:"test_name"                  json:"test_name"                         yaml:"test_name"`
	S3Storage *S3    `bson:"s3_storage"                 json:"s3_storage"                        yaml:"s3_storage"`
}
