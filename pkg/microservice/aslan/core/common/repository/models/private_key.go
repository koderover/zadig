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

type PrivateKey struct {
	ID         primitive.ObjectID `bson:"_id,omitempty"          json:"id,omitempty"`
	Name       string             `bson:"name"                   json:"name"`
	UserName   string             `bson:"user_name"              json:"user_name"`
	IP         string             `bson:"ip"                     json:"ip"`
	Label      string             `bson:"label"                  json:"label"`
	IsProd     bool               `bson:"is_prod"                json:"is_prod"`
	PrivateKey string             `bson:"private_key"            json:"private_key"`
	CreateTime int64              `bson:"create_time"            json:"create_time"`
	UpdateTime int64              `bson:"update_time"            json:"update_time"`
	UpdateBy   string             `bson:"update_by"              json:"update_by"`
}

func (PrivateKey) TableName() string {
	return "private_key"
}

//type SSH struct {
//	ID         string `json:"id"`
//	Name       string `json:"name"`
//	UserName   string `json:"user_name"`
//	IP         string `json:"ip"`
//	IsProd     bool   `json:"is_prod"`
//	Label      string `json:"label"`
//	PrivateKey string `json:"private_key"`
//}
