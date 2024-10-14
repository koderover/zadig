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

type SprintWorkItem struct {
	ID          primitive.ObjectID    `bson:"_id,omitempty"    yaml:"-"                  json:"id"`
	Title       string                `bson:"title"            yaml:"title"              json:"title"`
	Description string                `bson:"description"      yaml:"description"        json:"description"`
	Owners      []types.UserBriefInfo `bson:"owners"           yaml:"owners"             json:"owners"`
	SprintID    string                `bson:"sprint_id"        yaml:"sprint_id"          json:"sprint_id"`
	StageID     string                `bson:"stage_id"         yaml:"stage_id"           json:"stage_id"`
	CreateTime  int64                 `bson:"create_time"      yaml:"create_time"        json:"create_time"`
	UpdateTime  int64                 `bson:"update_time"      yaml:"update_time"        json:"update_time"`
}

func (SprintWorkItem) TableName() string {
	return "sprint_workitem"
}
