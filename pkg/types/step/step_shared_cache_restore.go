/*
Copyright 2026 The KodeRover Authors.

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

type StepSharedCacheRestoreSpec struct {
	CacheDir     string `bson:"cache_dir"     json:"cache_dir"     yaml:"cache_dir"`
	StoreDir     string `bson:"store_dir"     json:"store_dir"     yaml:"store_dir"`
	MetadataFile string `bson:"metadata_file" json:"metadata_file" yaml:"metadata_file"`
	IgnoreErr    bool   `bson:"ignore_err"    json:"ignore_err"    yaml:"ignore_err"`
}
