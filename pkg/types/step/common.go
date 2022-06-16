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

type Proxy struct {
	Type                   string `yaml:"type"`
	Address                string `yaml:"address"`
	Port                   int    `yaml:"port"`
	NeedPassword           bool   `yaml:"need_password"`
	Username               string `yaml:"username"`
	Password               string `yaml:"password"`
	EnableRepoProxy        bool   `yaml:"enable_repo_proxy"`
	EnableApplicationProxy bool   `yaml:"enable_application_proxy"`
}

type S3 struct {
	Ak        string `yaml:"ak"`
	Sk        string `yaml:"sk"`
	Endpoint  string `yaml:"endpoint"`
	Bucket    string `yaml:"bucket"`
	Subfolder string `yaml:"subfolder"`
	Insecure  bool   `yaml:"insecure"`
	Provider  int    `yaml:"provider"`
	Protocol  string `yaml:"protocol"`
}
