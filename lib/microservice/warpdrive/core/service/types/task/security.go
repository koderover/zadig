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

type Security struct {
	TaskType   config.TaskType `bson:"type"                          json:"type"`
	Enabled    bool            `bson:"enabled"                       json:"enabled"`
	TaskStatus config.Status   `bson:"status"                        json:"status"`
	ImageName  string          `bson:"image_name"                    json:"image_name"`
	ImageID    string          `bson:"image_id"                      json:"image_id"`
	Timeout    int             `bson:"timeout,omitempty"             json:"timeout,omitempty"`
	Error      string          `bson:"error,omitempty"               json:"error,omitempty"`
	StartTime  int64           `bson:"start_time,omitempty"          json:"start_time,omitempty"`
	EndTime    int64           `bson:"end_time,omitempty"            json:"end_time,omitempty"`
	LogFile    string          `bson:"log_file"                      json:"log_file"`
	Summary    map[string]int  `bson:"summary"                       json:"summary"`
}

func (s *Security) SetImageName(imageName string) {
	if imageName != "" {
		s.ImageName = imageName
	}
}

func (s *Security) ToSubTask() (map[string]interface{}, error) {
	var task map[string]interface{}
	if err := IToi(s, &task); err != nil {
		return nil, fmt.Errorf("convert SecurityTask to interface error: %v", err)
	}
	return task, nil
}
