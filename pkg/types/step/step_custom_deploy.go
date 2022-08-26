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

type StepCustomDeploySpec struct {
	Namespace          string     `bson:"namespace"              json:"namespace"             yaml:"namespace"`
	ClusterID          string     `bson:"cluster_id"             json:"cluster_id"            yaml:"cluster_id"`
	Timeout            int64      `bson:"timeout"                json:"timeout"               yaml:"timeout"`
	WorkloadType       string     `bson:"workload_type"          json:"workload_type"         yaml:"workload_type"`
	WorkloadName       string     `bson:"workload_name"          json:"workload_name"         yaml:"workload_name"`
	ContainerName      string     `bson:"container_name"         json:"container_name"        yaml:"container_name"`
	Image              string     `bson:"image"                  json:"image"                 yaml:"image"`
	SkipCheckRunStatus bool       `bson:"skip_check_run_status"  json:"skip_check_run_status" yaml:"skip_check_run_status"`
	ReplaceResources   []Resource `bson:"replace_resources"      json:"replace_resources"     yaml:"replace_resources"`
}
