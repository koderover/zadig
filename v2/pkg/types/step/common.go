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
	Type                   string `bson:"type"                              json:"type"                                 yaml:"type"`
	Address                string `bson:"address"                           json:"address"                              yaml:"address"`
	Port                   int    `bson:"port"                              json:"port"                                 yaml:"port"`
	NeedPassword           bool   `bson:"need_password"                     json:"need_password"                        yaml:"need_password"`
	Username               string `bson:"username"                          json:"username"                             yaml:"username"`
	Password               string `bson:"password"                          json:"password"                             yaml:"password"`
	EnableRepoProxy        bool   `bson:"enable_repo_proxy"                 json:"enable_repo_proxy"                    yaml:"enable_repo_proxy"`
	EnableApplicationProxy bool   `bson:"enable_application_proxy"          json:"enable_application_proxy"             yaml:"enable_application_proxy"`
}

type S3 struct {
	Ak        string `bson:"ak"                              json:"ak"                                 yaml:"ak"`
	Sk        string `bson:"sk"                              json:"sk"                                 yaml:"sk"`
	Endpoint  string `bson:"endpoint"                        json:"endpoint"                           yaml:"endpoint"`
	Bucket    string `bson:"bucket"                          json:"bucket"                             yaml:"bucket"`
	Subfolder string `bson:"subfolder"                       json:"subfolder"                          yaml:"subfolder"`
	Insecure  bool   `bson:"insecure"                        json:"insecure"                           yaml:"insecure"`
	Provider  int8   `bson:"provider"                        json:"provider"                           yaml:"provider"`
	Protocol  string `bson:"protocol"                        json:"protocol"                           yaml:"protocol"`
	Region    string `bson:"region"                          json:"region"                             yaml:"region"`
}
