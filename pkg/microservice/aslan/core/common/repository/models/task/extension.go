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

package task

import (
	"fmt"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
)

type Extension struct {
	TaskType     config.TaskType  `bson:"type"                       json:"type"`
	Enabled      bool             `bson:"enabled"                    json:"enabled"`
	TaskStatus   config.Status    `bson:"status"                     json:"status"`
	URL          string           `bson:"url"                        json:"url"`
	Path         string           `bson:"path,omitempty"             json:"path,omitempty"`
	IsCallback   bool             `bson:"is_callback"                json:"is_callback"`
	Headers      []*models.KeyVal `bson:"headers,omitempty"          json:"headers"`
	Payload      string           `bson:"payload"                    json:"payload"`
	ResponseBody *string          `bson:"response_body"              json:"response_body"`
	ResponseCode *int             `bson:"response_code"              json:"response_code"`
	ServiceInfos []*ServiceInfo   `bson:"service_infos"              json:"service_infos"`
	Timeout      int              `bson:"timeout"                    json:"timeout,omitempty"`
	IsRestart    bool             `bson:"is_restart"                 json:"is_restart"`
	Error        string           `bson:"error,omitempty"            json:"error,omitempty"`
	StartTime    int64            `bson:"start_time"                 json:"start_time,omitempty"`
	EndTime      int64            `bson:"end_time"                   json:"end_time,omitempty"`
}

type ServiceInfo struct {
	ServiceName   string `bson:"service_name"              json:"service_name"`
	ServiceModule string `bson:"service_module"            json:"service_module"`
	Image         string `bson:"image"                     json:"image"`
}

func (t *Extension) ToSubTask() (map[string]interface{}, error) {
	var task map[string]interface{}
	if err := IToi(t, &task); err != nil {
		return nil, fmt.Errorf("convert extension to interface error: %s", err)
	}
	return task, nil
}
