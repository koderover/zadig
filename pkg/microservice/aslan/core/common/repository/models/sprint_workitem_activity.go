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
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/types"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type SprintWorkItemActivity struct {
	ID               primitive.ObjectID                 `bson:"_id,omitempty"      yaml:"-"                  json:"id"`
	SprintWorkItemID string                             `bson:"sprint_workitem_id" yaml:"sprint_workitem_id" json:"sprint_workitem_id"`
	User             types.UserBriefInfo                `bson:"user"               yaml:"user"               json:"user"`
	Type             setting.SprintWorkItemActivityType `bson:"type"               yaml:"type"               json:"type"`
	Content          string                             `bson:"event"              yaml:"event"              json:"event"`
	CreateTime       int64                              `bson:"create_time"        yaml:"create_time"        json:"create_time"`
	UpdateTime       int64                              `bson:"update_time"        yaml:"update_time"        json:"update_time"`
}

func (SprintWorkItemActivity) TableName() string {
	return "sprint_workitem_activity"
}
