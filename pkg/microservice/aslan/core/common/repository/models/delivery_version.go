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
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models/template"
)

type DeliveryVersionWorkflowStatus struct {
	JobName       string        `json:"job_name"`
	ServiceName   string        `json:"service_name"`
	ServiceModule string        `json:"service_module"`
	TargetImage   string        `json:"target_image"`
	Status        config.Status `json:"status"`
}

type DeliveryVersionProgress struct {
	SuccessCount                  int                             `json:"successCount"`
	TotalCount                    int                             `json:"totalCount"`
	UploadStatus                  string                          `json:"uploadStatus"`
	DeliveryVersionWorkflowStatus []DeliveryVersionWorkflowStatus `json:"workflowStatus"`
	Error                         string                          `json:"error"`
}

type DeliveryVersion struct {
	ID                  primitive.ObjectID       `bson:"_id,omitempty"           json:"id,omitempty"`
	Version             string                   `bson:"version"                 json:"version"`
	ProductName         string                   `bson:"product_name"            json:"productName"`
	WorkflowName        string                   `bson:"workflow_name"           json:"workflowName"`
	WorkflowDisplayName string                   `bson:"workflow_display_name"   json:"workflowDisplayName"`
	Type                string                   `bson:"type"                    json:"type"`
	TaskID              int                      `bson:"task_id"                 json:"taskId"`
	Desc                string                   `bson:"desc"                    json:"desc"`
	Labels              []string                 `bson:"labels"                  json:"labels"`
	ProductEnvInfo      *Product                 `bson:"product_env_info"        json:"productEnvInfo"`
	Status              string                   `bson:"status"                  json:"status"`
	Error               string                   `bson:"error"                   json:"-"`
	Progress            *DeliveryVersionProgress `bson:"-"                       json:"progress"`
	CreateArgument      interface{}              `bson:"createArgument"          json:"-"`
	CreatedBy           string                   `bson:"created_by"              json:"createdBy"`
	CreatedAt           int64                    `bson:"created_at"              json:"created_at"`
	DeletedAt           int64                    `bson:"deleted_at"              json:"deleted_at"`
}

type DeliveryVersionYamlData struct {
	ImageRegistryID string                              `json:"imageRegistryID"`
	YamlDatas       []*CreateK8SDeliveryVersionYamlData `json:"yamlDatas"`
}

type DeliveryVersionChartData struct {
	GlobalVariables string                                `json:"globalVariables"`
	ChartRepoName   string                                `json:"chartRepoName"`
	ImageRegistryID string                                `json:"imageRegistryID"`
	ChartDatas      []*CreateHelmDeliveryVersionChartData `json:"chartDatas"`
}

type DeliveryChartData struct {
	ChartData      *CreateHelmDeliveryVersionChartData
	ServiceObj     *Service
	ProductService *ProductService
	RenderChart    *template.ServiceRender
	ValuesInEnv    map[string]interface{}
}

type CreateK8SDeliveryVersionYamlData struct {
	ServiceName string                      `json:"serviceName"`
	YamlContent string                      `json:"yamlContent"`
	ImageDatas  []*DeliveryVersionImageData `json:"imageDatas"`
}

type CreateHelmDeliveryVersionChartData struct {
	ServiceName       string                      `json:"serviceName"`
	Version           string                      `json:"version,omitempty"`
	ValuesYamlContent string                      `json:"valuesYamlContent"`
	ImageData         []*DeliveryVersionImageData `json:"imageData"`
}

type DeliveryVersionImageData struct {
	ContainerName string `json:"containerName"`
	Image         string `json:"image"`
	ImageName     string `json:"imageName"`
	ImageTag      string `json:"imageTag"`
	Selected      bool   `json:"selected"`
}

func (DeliveryVersion) TableName() string {
	return "delivery_version"
}
