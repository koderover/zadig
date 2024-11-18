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

import "github.com/koderover/zadig/v2/pkg/types"

type StepP4Spec struct {
	Repos        []*types.Repository `bson:"repos"          json:"repos"         yaml:"repos"`
	WorkflowName string              `bson:"workflow_name"  json:"workflow_name" yaml:"workflow_name"`
	ProjectKey   string              `bson:"project_key"    json:"project_key"   yaml:"project_key"`
	TaskID       int64               `bson:"task_id"        json:"task_id"       yaml:"task_id"`
	JobName      string              `bson:"job_name"       json:"job_name"      yaml:"job_name"`
	// P4 is unable to use http/https proxy since it does not use HTTP protocol
	// Proxy *Proxy              `bson:"proxy"          json:"proxy"    yaml:"proxy"`
}
