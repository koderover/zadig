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

type WebHookUser struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"          json:"id,omitempty"`
	Domain    string             `bson:"domain,omitempty"       json:"domain,omitempty"`
	UserName  string             `bson:"user_name"              json:"user_name"`
	Email     string             `bson:"email"                  json:"email"`
	Source    string             `bson:"source"                 json:"source"`
	CreatedAt int64              `bson:"created_at"             json:"created_at"`
}

//type DomainWebHookUser struct {
//	ID        bson.ObjectId `bson:"_id,omitempty"   json:"id,omitempty"`
//	Domain    string        `bson:"domain"          json:"domain"`
//	UserCount int           `bson:"user_count"      json:"user_count"`
//	CreatedAt int64         `bson:"created_at"      json:"created_at"`
//}

func (WebHookUser) TableName() string {
	return "user_stat"
}
