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

const (
	ImageFromKoderover = "koderover"
	ImageFromCustom    = "custom"
)

// BasicImage ...
type BasicImage struct {
	ID         primitive.ObjectID `bson:"_id,omitempty"          json:"id,omitempty"`
	Value      string             `bson:"value"                  json:"value"`
	Label      string             `bson:"label"                  json:"label"`
	ImageFrom  string             `bson:"image_from"             json:"image_from"`
	CreateTime int64              `bson:"create_time"            json:"create_time"`
	UpdateTime int64              `bson:"update_time"            json:"update_time"`
	UpdateBy   string             `bson:"update_by"              json:"update_by"`
}

func (BasicImage) TableName() string {
	return "basic_image"
}
