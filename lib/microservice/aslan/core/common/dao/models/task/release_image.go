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
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
)

//type RepoImage struct {
//	RepoId    string `json:"repo_id" bson:"repo_id"`
//	Name      string `json:"name" bson:"name" yaml:"name"`
//	Username  string `json:"-" yaml:"username"`
//	Password  string `json:"-" yaml:"password"`
//	Host      string `json:"host" yaml:"host"`
//	Namespace string `json:"namespace" yaml:"namespace"`
//}

type ReleaseImageItem struct {
	RepoId string `bson:"id" json:"id"`
	Image  string `json:"image" json:"image"`
}

// ReleaseImage 线上镜像分发任务
type ReleaseImage struct {
	TaskType     config.TaskType `bson:"type"                           json:"type"`
	Enabled      bool            `bson:"enabled"                        json:"enabled"`
	TaskStatus   config.Status   `bson:"status"                         json:"status"`
	ImageTest    string          `bson:"image_test"                     json:"image_test"`
	ImageRelease string          `bson:"image_release"                  json:"image_release"`
	ImageRepo    string          `bson:"image_repo"                     json:"image_repo"`
	Timeout      int             `bson:"timeout,omitempty"              json:"timeout,omitempty"`
	Error        string          `bson:"error,omitempty"                json:"error,omitempty"`
	StartTime    int64           `bson:"start_time,omitempty"           json:"start_time,omitempty"`
	EndTime      int64           `bson:"end_time,omitempty"             json:"end_time,omitempty"`
	LogFile      string          `bson:"log_file"                       json:"log_file"`

	// destinations to distribute images
	Releases []models.RepoImage `bson:"releases"                 json:"releases"`
}

// SetImage ...
func (ri *ReleaseImage) SetImage(image string) {
	if image != "" {
		ri.ImageTest = image
	}
}

// ToSubTask ...
func (ri *ReleaseImage) ToSubTask() (map[string]interface{}, error) {
	var task map[string]interface{}
	if err := IToi(ri, &task); err != nil {
		return nil, fmt.Errorf("convert ReleaseImageTask to interface error: %v", err)
	}
	return task, nil
}
