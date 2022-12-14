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

package types

type ObjectStorageInfo struct {
	Endpoint string `bson:"endpoint" json:"endpoint" yaml:"endpoint"`
	AK       string `bson:"AK"       json:"AK"       yaml:"AK"`
	SK       string `bson:"SK"       json:"SK"       yaml:"SK"`
	Bucket   string `bson:"bucket"   json:"bucket"   yaml:"bucket"`
	Insecure bool   `bson:"insecure" json:"insecure" yaml:"insecure"`
	Provider int8   `bson:"provider" json:"provider" yaml:"provider"`
	Region   string `bson:"region"   json:"region"   yaml:"region"`
}

type ObjectStoragePathDetail struct {
	FilePath        string `bson:"file_path" json:"file_path" yaml:"file_path"`
	AbsFilePath     string `bson:"abs_file_path" json:"abs_file_path" yaml:"abs_file_path"`
	DestinationPath string `bson:"dest_path" json:"dest_path" yaml:"dest_path"`
}
