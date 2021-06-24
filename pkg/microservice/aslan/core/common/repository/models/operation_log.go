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

type OperationLog struct {
	ID             primitive.ObjectID `bson:"_id,omitempty"               json:"id,omitempty"`
	Username       string             `bson:"username"                    json:"username"`
	ProductName    string             `bson:"product_name"                json:"product_name"`
	Method         string             `bson:"method"                      json:"method"`
	PermissionUUID string             `bson:"permission_uuid"             json:"permission_uuid"`
	Function       string             `bson:"function"                    json:"function"`
	Name           string             `bson:"name"                        json:"name"`
	RequestBody    string             `bson:"request_body"                json:"request_body"`
	Status         int                `bson:"status"                      json:"status"`
	CreatedAt      int64              `bson:"created_at"                  json:"created_at"`
}

func (OperationLog) TableName() string {
	return "operation_log"
}
