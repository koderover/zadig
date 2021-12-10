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
	"encoding/json"
	"fmt"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
)

type ImageData struct {
	ImageUrl       string `bson:"image_url" json:"image_url"`
	ImageName      string `bson:"image_name" json:"image_name"`
	ImageNamespace string `bson:"image_namespace" json:"image_namespace"`
	ImageTag       string `bson:"image_tag" json:"image_tag"`
}

type ServicePackageResult struct {
	ServiceName string       `json:"service_name"`
	Result      string       `json:"result"`
	ErrorMsg    string       `json:"error_msg"`
	ImageData   []*ImageData `json:"image_data"`
}

type ArtifactPackage struct {
	TaskType   config.TaskType `bson:"type"                           json:"type"`
	TaskStatus config.Status   `bson:"status"                         json:"status"`
	Enabled    bool            `bson:"enabled"                        json:"enabled"`
	Timeout    int             `bson:"timeout,omitempty"              json:"timeout,omitempty"`
	Error      string          `bson:"error,omitempty"                json:"error,omitempty"`
	StartTime  int64           `bson:"start_time,omitempty"           json:"start_time,omitempty"`
	EndTime    int64           `bson:"end_time,omitempty"             json:"end_time,omitempty"`
	LogFile    string          `bson:"log_file"                       json:"log_file"`
	Progress   string          `bson:"progress"                       json:"progress"`

	// source images
	Images []*models.ImagesByService `bson:"images" json:"images"`
	// source registries need auth to pull images
	SourceRegistries []string `bson:"source_registries" json:"source_registries"`
	// target registries to push images
	TargetRegistries []string `bson:"target_registries" json:"target_registries"`
}

func (ri *ArtifactPackage) ToSubTask() (map[string]interface{}, error) {
	var task map[string]interface{}
	if err := IToi(ri, &task); err != nil {
		return nil, fmt.Errorf("convert ReleaseImageTask to interface error: %v", err)
	}
	return task, nil
}

func (ri *ArtifactPackage) GetProgress() ([]*ServicePackageResult, error) {
	if len(ri.Progress) == 0 {
		return nil, nil
	}
	ret := make([]*ServicePackageResult, 0)
	err := json.Unmarshal([]byte(ri.Progress), &ret)
	return ret, err
}
