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

package step

type StepDownloadArchiveSpec struct {
	FileName   string `bson:"file_name"                          json:"file_name"                                 yaml:"file_name"`
	ObjectPath string `bson:"object_path"                        json:"object_path"                               yaml:"object_path"`
	DestDir    string `bson:"dest_dir"                           json:"dest_dir"                                  yaml:"dest_dir"`
	UnTar      bool   `bson:"untar"                              json:"untar"                                     yaml:"untar"`
	IgnoreErr  bool   `bson:"ignore_err"                         json:"ignore_err"                                yaml:"ignore_err"`
	S3         *S3    `bson:"s3_storage"                         json:"s3_storage"                                yaml:"s3_storage"`
}
