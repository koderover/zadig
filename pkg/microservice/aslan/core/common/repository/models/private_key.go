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
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/types"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type PrivateKey struct {
	ID           primitive.ObjectID   `bson:"_id,omitempty"          json:"id,omitempty"`
	Name         string               `bson:"name"                   json:"name"`
	UserName     string               `bson:"user_name"              json:"user_name"`
	IP           string               `bson:"ip"                     json:"ip"`
	Port         int64                `bson:"port"                   json:"port"`
	Status       setting.PMHostStatus `bson:"status"                 json:"status"`
	Label        string               `bson:"label"                  json:"label"`
	IsProd       bool                 `bson:"is_prod"                json:"is_prod"`
	PrivateKey   string               `bson:"private_key"            json:"private_key"`
	CreateTime   int64                `bson:"create_time"            json:"create_time"`
	UpdateTime   int64                `bson:"update_time"            json:"update_time"`
	UpdateBy     string               `bson:"update_by"              json:"update_by"`
	Provider     int8                 `bson:"provider"               json:"provider"`
	Probe        *types.Probe         `bson:"probe"                  json:"probe"`
	ProjectName  string               `bson:"project_name,omitempty" json:"project_name"`
	UpdateStatus bool                 `bson:"-"                      json:"update_status"`
}

func (PrivateKey) TableName() string {
	return "private_key"
}
