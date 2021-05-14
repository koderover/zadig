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

import "go.mongodb.org/mongo-driver/bson/primitive"

type Favorite struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"         json:"id,omitempty"`
	UserID      int                `bson:"user_id"                json:"user_id"`
	ProductName string             `bson:"product_name"           json:"product_name"`
	Name        string             `bson:"name"                   json:"name"`
	Type        string             `bson:"type"                   json:"type"`
	CreateTime  int64              `bson:"create_time"            json:"create_time"`
}

func (Favorite) TableName() string {
	return "favorite"
}
