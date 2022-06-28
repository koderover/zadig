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

type StepHelmDeploySpec struct {
	Env              string                   `bson:"env"                              json:"env"                                 yaml:"env"`
	ServiceName      string                   `bson:"service_name"                     json:"service_name"                        yaml:"service_name"`
	ServiceType      string                   `bson:"service_type"                     json:"service_type"                        yaml:"service_type"`
	ImageAndModules  []*ImageAndServiceModule `bson:"image_and_service_modules"        json:"image_and_service_modules"           yaml:"image_and_service_modules"`
	ClusterID        string                   `bson:"cluster_id"                       json:"cluster_id"                          yaml:"cluster_id"`
	ReleaseName      string                   `bson:"release_name"                     json:"release_name"                        yaml:"release_name"`
	Timeout          int                      `bson:"timeout"                          json:"timeout"                             yaml:"timeout"`
	ReplaceResources []Resource               `bson:"replace_resources"                json:"replace_resources"                   yaml:"replace_resources"`
}

type ImageAndServiceModule struct {
	ServiceModule string `bson:"service_module"                     json:"service_module"                        yaml:"service_module"`
	Image         string `bson:"image"                              json:"image"                                 yaml:"image"`
}
