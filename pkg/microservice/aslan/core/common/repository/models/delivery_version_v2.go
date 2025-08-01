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

package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/setting"
)

type DeliveryVersionV2 struct {
	ID                    primitive.ObjectID            `bson:"_id,omitempty"            json:"id,omitempty"`
	Version               string                        `bson:"version"                  json:"version"`
	ProjectName           string                        `bson:"project_name"             json:"project_name"`
	EnvName               string                        `bson:"env_name"                 json:"env_name"`
	Production            bool                          `bson:"production"               json:"production"`
	Type                  setting.DeliveryVersionType   `bson:"type"                     json:"type"`
	Source                setting.DeliveryVersionSource `bson:"source"                   json:"source"`
	Desc                  string                        `bson:"desc"                     json:"desc"`
	Labels                []string                      `bson:"labels"                   json:"labels"`
	ImageRegistryID       string                        `bson:"image_registry_id"        json:"image_registry_id"`
	OriginalChartRepoName string                        `bson:"original_chart_repo_name" json:"original_chart_repo_name"`
	ChartRepoName         string                        `bson:"chart_repo_name"          json:"chart_repo_name"`
	Services              []*DeliveryVersionService     `bson:"services"                 json:"services"`
	Status                setting.DeliveryVersionStatus `bson:"status"                   json:"status"`
	WorkflowName          string                        `bson:"workflow_name"            json:"workflow_name"`
	TaskID                int64                         `bson:"task_id"                  json:"task_id"`
	Error                 string                        `bson:"error"                    json:"error"`
	CreatedBy             string                        `bson:"created_by"               json:"created_by"`
	CreatedAt             int64                         `bson:"created_at"               json:"created_at"`
	DeletedAt             int64                         `bson:"deleted_at"               json:"deleted_at"`
}

type DeliveryVersionService struct {
	ServiceName          string                  `bson:"service_name"            json:"service_name"`
	ChartName            string                  `bson:"chart_name"              json:"chart_name"`
	OriginalChartVersion string                  `bson:"original_chart_version"  json:"original_chart_version"`
	ChartVersion         string                  `bson:"chart_version"           json:"chart_version"`
	ChartStatus          config.Status           `bson:"chart_status"            json:"chart_status"`
	YamlContent          string                  `bson:"yaml_content"            json:"yaml_content"`
	Images               []*DeliveryVersionImage `bson:"images"                  json:"images"`
	Error                string                  `bson:"error"                   json:"error"`
}

type DeliveryVersionImage struct {
	ContainerName  string         `bson:"container_name"        json:"container_name"`
	ImageName      string         `bson:"image_name"            json:"image_name"`
	ImagePath      *ImagePathSpec `bson:"image_path"            json:"image_path"`
	SourceImage    string         `bson:"source_image"          json:"source_image"`
	SourceImageTag string         `bson:"source_image_tag"      json:"source_image_tag"`
	TargetImage    string         `bson:"target_image"          json:"target_image"`
	TargetImageTag string         `bson:"target_image_tag"      json:"target_image_tag"`
	PushImage      bool           `bson:"push_image"            json:"push_image"`
	Status         config.Status  `bson:"status"                json:"status"`
	Error          string         `bson:"error"                 json:"error"`
}

func (DeliveryVersionV2) TableName() string {
	return "delivery_version_v2"
}
