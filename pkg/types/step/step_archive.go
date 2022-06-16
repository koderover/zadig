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

type StepArchiveSpec struct {
	FilePath        string `yaml:"file_path"`
	AbsFilePath     string `yaml:"abs_file_path"`
	DestinationPath string `yaml:"dest_path"`
	S3StorageID     string `yaml:"s3_storage_id"`
	S3              *S3    `yaml:"s3_storage"`
}
