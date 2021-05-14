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

type Resource struct {
	Name      string `json:"name" bson:"name"`
	Kind      string `json:"kind" bson:"kind"`
	Container string `json:"container" bson:"container"`
	Origin    string `json:"origin" bson:"origin"`
}

// Deploy 容器部署任务
type Deploy struct {
	TaskType        config.TaskType `bson:"type"                          json:"type"`
	Enabled         bool            `bson:"enabled"                       json:"enabled"`
	TaskStatus      config.Status   `bson:"status"                        json:"status"`
	Namespace       string          `bson:"namespace"                     json:"namespace"`
	EnvName         string          `bson:"env_name"                      json:"env_name"`
	ProductName     string          `bson:"product_name"                  json:"product_name"`
	ServiceName     string          `bson:"service_name"                  json:"service_name"`
	ServiceType     string          `bson:"service_type"                  json:"service_type"`
	ServiceRevision int64           `bson:"service_revision,omitempty"    json:"service_revision,omitempty"`
	ContainerName   string          `bson:"container_name"                json:"container_name"`
	Image           string          `bson:"image"                         json:"image"`
	Timeout         int             `bson:"timeout,omitempty"             json:"timeout,omitempty"`
	Error           string          `bson:"error,omitempty"               json:"error,omitempty"`
	StartTime       int64           `bson:"start_time,omitempty"          json:"start_time,omitempty"`
	EndTime         int64           `bson:"end_time,omitempty"            json:"end_time,omitempty"`

	ReplaceResources []Resource `bson:"replace_resources"       json:"replace_resources"`
	SkipWaiting      bool       `bson:"skipWaiting"             json:"skipWaiting"`
	IsRestart        bool       `bson:"is_restart"              json:"is_restart"`
	ResetImage       bool       `bson:"reset_image"             json:"reset_image"`
}

// SetNamespace ...
func (d *Deploy) SetNamespace(namespace string) {
	if namespace != "" {
		d.Namespace = namespace
	}
}

// SetImage ...
func (d *Deploy) SetImage(image string) {
	if image != "" {
		d.Image = image
	}
}

// ToSubTask ...
func (d *Deploy) ToSubTask() (map[string]interface{}, error) {
	var task map[string]interface{}
	if err := IToi(d, &task); err != nil {
		return nil, fmt.Errorf("convert DeployTask to interface error: %v", err)
	}
	return task, nil
}
