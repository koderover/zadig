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

type StepDeploySpec struct {
	Env              string     `yaml:"env"`
	ServiceName      string     `yaml:"service_name"`
	ServiceType      string     `yaml:"service_type"`
	ServiceModule    string     `yaml:"service_module"`
	Image            string     `yaml:"image"`
	ClusterID        string     `yaml:"cluster_id"`
	Timeout          int        `yaml:"timeout"`
	ReplaceResources []Resource `yaml:"replace_resources"`
}

type Resource struct {
	Name      string `yaml:"name"`
	Kind      string `yaml:"kind"`
	Container string `yaml:"container"`
	Origin    string `yaml:"origin"`
}
