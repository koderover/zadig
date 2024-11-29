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

type StepTarArchiveSpec struct {
	// Source file/dir path
	ResultDirs []string `bson:"result_dirs"                json:"result_dirs"                       yaml:"result_dirs"`
	// Is absolute path in result dir
	AbsResultDir bool `bson:"abs_result_dir"             json:"abs_result_dir"                    yaml:"abs_result_dir"`
	// Tar dest dir
	DestDir string `bson:"dest_dir"                   json:"dest_dir"                          yaml:"dest_dir"`
	// S3 dest dir
	S3DestDir string `bson:"s3_dest_dir"                json:"s3_dest_dir"                       yaml:"s3_dest_dir"`
	// Change tar dir, equal to "-C $TarDir" in tar command
	TarDir string `bson:"tar_dir"                    json:"tar_dir"                           yaml:"tar_dir"`
	// Enable change tar dir, equal to "-C" in tar command
	ChangeTarDir bool `bson:"change_tar_dir"             json:"change_tar_dir"                    yaml:"change_tar_dir"`
	// File name
	FileName  string `bson:"file_name"                  json:"file_name"                         yaml:"file_name"`
	IgnoreErr bool   `bson:"ignore_err"                 json:"ignore_err"                        yaml:"ignore_err"`
	S3Storage *S3    `bson:"s3_storage"                 json:"s3_storage"                        yaml:"s3_storage"`
}
