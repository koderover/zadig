/*
Copyright 2021 The KodeRover Authors.

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

package task

import (
	"fmt"

	"github.com/koderover/zadig/lib/microservice/warpdrive/config"
)

type DockerBuild struct {
	TaskType   config.TaskType `bson:"type"                    json:"type"`
	Enabled    bool            `bson:"enabled"                 json:"enabled"`
	TaskStatus config.Status   `bson:"status"                  json:"status"`
	WorkDir    string          `bson:"work_dir"                json:"work_dir"`             // Dockerfile 路径
	DockerFile string          `bson:"docker_file"             json:"docker_file"`          // Dockerfile 名称, 默认为Dockerfile
	BuildArgs  string          `bson:"build_args,omitempty"    json:"build_args,omitempty"` // Docker build可变参数
	Image      string          `bson:"image"                   json:"image"`
	OnSetup    string          `bson:"setup,omitempty"         json:"setup,omitempty"`
	Timeout    int             `bson:"timeout"                 json:"timeout,omitempty"`
	Error      string          `bson:"error"                   json:"error,omitempty"`
	StartTime  int64           `bson:"start_time"              json:"start_time,omitempty"`
	EndTime    int64           `bson:"end_time"                json:"end_time,omitempty"`
	JobName    string          `bson:"job_name"                json:"job_name"`
	LogFile    string          `bson:"log_file"                json:"log_file"`
}

func (db *DockerBuild) ToSubTask() (map[string]interface{}, error) {
	var task map[string]interface{}
	if err := IToi(db, &task); err != nil {
		return nil, fmt.Errorf("convert DockerBuildTask to interface error: %v", err)
	}
	return task, nil
}
