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

	"github.com/koderover/zadig/pkg/microservice/aslan/core/collaboration/config"
)

type CollaborationMode struct {
	ID primitive.ObjectID `bson:"_id,omitempty"             json:"id,omitempty"`
	//foreign key:template_product/product_name
	ProjectName string           `bson:"project_name"              json:"project_name"`
	CreateTime  int64            `bson:"create_time"               json:"create_time"`
	UpdateTime  int64            `bson:"update_time"               json:"update_time"`
	Name        string           `bson:"name"                      json:"name"`
	Revision    int64            `bson:"revision"                  json:"revision"`
	Members     []string         `bson:"members"                   json:"members"`
	IsDeleted   bool             `bson:"is_deleted"                json:"is_deleted"`
	DeployType  string           `bson:"deploy_type"               json:"deploy_type"`
	RecycleDay  int64            `bson:"recycle_day"               json:"recycle_day"`
	Workflows   []WorkflowCMItem `bson:"workflows"                 json:"workflows" `
	Products    []ProductCMItem  `bson:"products"                  json:"products"`
	CreateBy    string           `bson:"create_by"                 json:"create_by"`
	UpdateBy    string           `bson:"update_by"                 json:"update_by"`
}

type ProductCMItem struct {
	Name              string                   `bson:"name" json:"name"`
	CollaborationType config.CollaborationType `bson:"collaboration_type" json:"collaboration_type"`
	RecycleDay        int64                    `bson:"recycle_day" json:"recycle_day"`
	Verbs             []string                 `bson:"verbs" json:"verbs"`
}

type WorkflowCMItem struct {
	WorkflowType      string                   `bson:"workflow_type" json:"workflow_type"`
	Name              string                   `bson:"name" json:"name"`
	// workflow display name, it can be modified by users, so we don't save it.
	DisplayName       string                   `bson:"-"    json:"display_name"`
	CollaborationType config.CollaborationType `bson:"collaboration_type" json:"collaboration_type"`
	Verbs             []string                 `bson:"verbs" json:"verbs"`
}

func (CollaborationMode) TableName() string {
	return "collaboration_mode"
}
