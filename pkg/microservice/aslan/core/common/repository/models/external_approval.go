/*
 * Copyright 2022 The KodeRover Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type IMApp struct {
	ID   primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Type string             `json:"type" bson:"type"`
	Name string             `json:"name" bson:"name"`
	// Lark fields
	AppID                   string `json:"app_id" bson:"app_id"`
	AppSecret               string `json:"app_secret" bson:"app_secret"`
	EncryptKey              string `json:"encrypt_key" bson:"encrypt_key"`
	LarkDefaultApprovalCode string `json:"-" bson:"lark_default_approval_code"`

	UpdateTime int64 `json:"update_time" bson:"update_time"`
}

func (IMApp) TableName() string {
	return "im_app"
}
