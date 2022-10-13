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
)

type DeliveryVersionProgress struct {
	SuccessChartCount   int    `json:"successChartCount"`
	TotalChartCount     int    `json:"totalChartCount"`
	PackageUploadStatus string `json:"packageStatus"`
	Error               string `json:"error"`
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

func (DeliveryVersion) TableName() string {
	return "delivery_version"
}
