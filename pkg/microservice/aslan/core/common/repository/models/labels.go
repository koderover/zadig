/*
Copyright 2024 The KodeRover Authors.

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

type Label struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"         json:"id,omitempty"`
	Key         string             `bson:"key"                   json:"key"`
	Description string             `bson:"description"           json:"description"`
	CreatedAt   int64              `bson:"created_at"            json:"created_at"`
	UpdatedAt   int64              `bson:"updated_at"            json:"updated_at"`
}

func (l Label) TableName() string {
	return "label"
}
