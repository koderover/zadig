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
	"github.com/koderover/zadig/v2/pkg/types"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type LarkPluginWorkitemTask struct {
	ID                  primitive.ObjectID  `bson:"_id,omitempty"        yaml:"-"                    json:"id"`
	LarkWorkItemTypeKey string              `bson:"lark_workitem_type_key" yaml:"lark_workitem_type_key" json:"lark_workitem_type_key"`
	LarkWorkItemID      string              `bson:"lark_workitem_id"     yaml:"lark_workitem_id"     json:"lark_workitem_id"`
	WorkflowName        string              `bson:"workflow_name"        yaml:"workflow_name"        json:"workflow_name"`
	WorkflowTaskID      int64               `bson:"workflow_task_id"     yaml:"workflow_task_id"     json:"workflow_task_id"`
	Hash                string              `bson:"hash"                 yaml:"hash"                 json:"hash"`
	Creator             types.UserBriefInfo `bson:"creator"              yaml:"creator"              json:"creator"`
	CreateTime          int64               `bson:"create_time"          yaml:"create_time"          json:"create_time"`
}

func (LarkPluginWorkitemTask) TableName() string {
	return "lark_plugin_workitem_task"
}
