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

type Workflow struct {
	ID              primitive.ObjectID `bson:"_id,omitempty"                json:"id,omitempty"`
	Name            string             `bson:"name"                         json:"name"`
	Enabled         bool               `bson:"enabled"                      json:"enabled"`
	ProductTmplName string             `bson:"product_tmpl_name"            json:"product_tmpl_name"`
	Team            string             `bson:"team,omitempty"               json:"team,omitempty"`
	EnvName         string             `bson:"env_name"                     json:"env_name,omitempty"`
	Description     string             `bson:"description,omitempty"        json:"description,omitempty"`
	UpdateBy        string             `bson:"update_by"                    json:"update_by,omitempty"`
	CreateBy        string             `bson:"create_by"                    json:"create_by,omitempty"`
	UpdateTime      int64              `bson:"update_time"                  json:"update_time"`
	CreateTime      int64              `bson:"create_time"                  json:"create_time"`
	Schedules       *ScheduleCtrl      `bson:"schedules,omitempty"          json:"schedules,omitempty"`
	ScheduleEnabled bool               `bson:"schedule_enabled"             json:"schedule_enabled"`
	// TODO: Deprecated.
	NotifyCtl *NotifyCtl `bson:"notify_ctl,omitempty"         json:"notify_ctl,omitempty"`
	// New since V1.12.0.
	NotifyCtls []*NotifyCtl `bson:"notify_ctls"        json:"notify_ctls"`
	BaseName   string       `bson:"base_name" json:"base_name"`

	// ResetImage indicate whether reset image to original version after completion
	ResetImage bool `bson:"reset_image"                  json:"reset_image"`
	// IsParallel 控制单一工作流的任务是否支持并行处理
	IsParallel bool `json:"is_parallel" bson:"is_parallel"`
}

func (Workflow) TableName() string {
	return "workflow"
}
