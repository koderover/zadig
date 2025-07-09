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

package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/collaboration/config"
)

type CollaborationInstance struct {
	ID                primitive.ObjectID `bson:"_id,omitempty"            json:"id,omitempty"`
	ProjectName       string             `bson:"project_name"             json:"project_name"`
	CreateTime        int64              `bson:"create_time"              json:"create_time"`
	UpdateTime        int64              `bson:"update_time"              json:"update_time"`
	LastVisitTime     int64              `bson:"last_visit_time"          json:"last_visit_time"`
	CollaborationName string             `bson:"collaboration_name"       json:"collaboration_name"`
	RecycleDay        int64              `bson:"recycle_day"              json:"recycle_day"`
	Revision          int64              `bson:"revision"                 json:"revision"`
	UserUID           string             `bson:"user_uid"                 json:"user_uid"`
	PolicyName        string             `bson:"policy_name"              json:"policy_name"`
	IsDeleted         bool               `bson:"is_deleted"               json:"is_deleted"`
	Workflows         []WorkflowCIItem   `bson:"workflows"                json:"workflows"`
	Products          []ProductCIItem    `bson:"products"                 json:"products"`
}
type ProductCIItem struct {
	Name              string                   `bson:"name"               json:"name"`
	BaseName          string                   `bson:"base_name"          json:"base_name"`
	CollaborationType config.CollaborationType `bson:"collaboration_type" json:"collaboration_type"`
	RecycleDay        int64                    `bson:"recycle_day"        json:"recycle_day"`
	Verbs             []string                 `bson:"verbs"              json:"verbs"`
}

type WorkflowCIItem struct {
	WorkflowType string `bson:"workflow_type" json:"workflow_type"`
	Name         string `bson:"name"               json:"name"`
	// workflow display name, it can be modified by users, so we don't save it.
	DisplayName       string                   `bson:"-"                  json:"display_name"`
	BaseName          string                   `bson:"base_name"          json:"base_name"`
	CollaborationType config.CollaborationType `bson:"collaboration_type" json:"collaboration_type"`
	Verbs             []string                 `bson:"verbs"              json:"verbs"`
}

func (CollaborationInstance) TableName() string {
	return "collaboration_instance"
}
