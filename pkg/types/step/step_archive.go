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

import "github.com/koderover/zadig/v2/pkg/types"

type StepArchiveSpec struct {
	UploadDetail    []*Upload           `bson:"upload_detail"                      json:"upload_detail"                             yaml:"upload_detail"`
	ObjectStorageID string              `bson:"object_storage_id"                  json:"object_storage_id"                         yaml:"object_storage_id"`
	S3              *S3                 `bson:"s3_storage"                         json:"s3_storage"                                yaml:"s3_storage"`
	Repos           []*types.Repository `bson:"repos"                                 json:"repos"`
}

type Upload struct {
	Name                string `bson:"name"                                  json:"name"                                         yaml:"name"`
	IsFileArchive       bool   `bson:"is_file_archive"                       json:"is_file_archive"                            yaml:"is_file_archive"`
	ServiceName         string `bson:"service_name"                          json:"service_name"                                yaml:"service_name"`
	ServiceModule       string `bson:"service_module"                        json:"service_module"                                yaml:"service_module"`
	JobTaskName         string `bson:"job_task_name"                         json:"job_task_name"                               yaml:"job_task_name"`
	PackageFileLocation string `bson:"package_file_location"                 json:"package_file_location"                       yaml:"package_file_location"`
	FilePath            string `bson:"file_path"                             json:"file_path"                                 yaml:"file_path"`
	AbsFilePath         string `bson:"abs_file_path"                         json:"aabs_file_pathk"                           yaml:"abs_file_path"`
	DestinationPath     string `bson:"dest_path"                             json:"dest_path"                                 yaml:"dest_path"`
}
