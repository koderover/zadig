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

	"github.com/koderover/zadig/lib/microservice/aslan/config"
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

// ToSubTask ...
func (db *DockerBuild) ToSubTask() (map[string]interface{}, error) {
	var task map[string]interface{}
	if err := IToi(db, &task); err != nil {
		return nil, fmt.Errorf("convert DockerBuildTask to interface error: %v", err)
	}
	return task, nil
}

// DockerBuildCtx ...
// Docker build参数
// WorkDir: docker build执行路径
// DockerFile: dockerfile名称, 默认为Dockerfile
// ImageBuild: build image镜像全称, e.g. xxx.com/release-candidates/image:tag
type DockerBuildCtx struct {
	WorkDir         string `yaml:"work_dir" bson:"work_dir" json:"work_dir"`
	DockerFile      string `yaml:"docker_file" bson:"docker_file" json:"docker_file"`
	ImageName       string `yaml:"image_name" bson:"image_name" json:"image_name"`
	BuildArgs       string `yaml:"build_args" bson:"build_args" json:"build_args"`
	ImageReleaseTag string `yaml:"image_release_tag,omitempty" bson:"image_release_tag,omitempty" json:"image_release_tag"`
}

type FileArchiveCtx struct {
	FileLocation string `yaml:"file_location" bson:"file_location" json:"file_location"`
	FileName     string `yaml:"file_name" bson:"file_name" json:"file_name"`
	//StorageUri   string `yaml:"storage_uri" bson:"storage_uri" json:"storage_uri"`
}

// GetDockerFile ...
func (buildCtx *DockerBuildCtx) GetDockerFile() string {
	if buildCtx.DockerFile == "" {
		return "Dockerfile"
	}
	return buildCtx.DockerFile
}
