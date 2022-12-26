/*
Copyright 2022 The KodeRover Authors.

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

type UserSetting struct {
	ID           primitive.ObjectID `bson:"_id,omitempty"      json:"id,omitempty"`
	UID          string             `bson:"uid"                json:"uid"`
	Theme        string             `bson:"theme"              json:"theme"`
	LogBgColor   string             `bson:"log_bg_color"       json:"log_bg_color"`
	LogFontColor string             `bson:"log_font_color"     json:"log_font_color"`
}

func (UserSetting) TableName() string {
	return "user_setting"
}
