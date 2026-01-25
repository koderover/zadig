/*
Copyright 2023 The KodeRover Authors.

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
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type CloudService struct {
	ID              primitive.ObjectID       `bson:"_id,omitempty"         json:"id,omitempty"`
	Name            string                   `bson:"name"                  json:"name"`
	Type            setting.CloudServiceType `bson:"type"                  json:"type"`
	Region          string                   `bson:"region"                json:"region"`
	AccessKeyId     string                   `bson:"access_key_id"         json:"access_key_id"`
	AccessKeySecret string                   `bson:"access_key_secret"     json:"access_key_secret"`
	UpdateBy        string                   `bson:"update_by"             json:"update_by"`
	CreatedAt       int64                    `bson:"created_at"            json:"created_at"`
	UpdatedAt       int64                    `bson:"updated_at"            json:"updated_at"`
}

func (s CloudService) TableName() string {
	return "cloud_service"
}
