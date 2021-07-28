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

type WebHook struct {
	ID      primitive.ObjectID `bson:"_id,omitempty"       json:"id,omitempty"`
	Owner   string             `bson:"owner"               json:"owner"`
	Repo    string             `bson:"repo"                json:"repo"`
	Address string             `bson:"address"             json:"address"`
	HookID  string             `bson:"hook_id,omitempty"   json:"hook_id,omitempty"`
	// References is a record to store all the workflows/services who are using this webhook
	References []string `bson:"references"    json:"references"`
}

func (WebHook) TableName() string {
	return "webhook"
}
