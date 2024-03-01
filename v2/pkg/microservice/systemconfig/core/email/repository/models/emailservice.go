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

type EmailService struct {
	Name        string `json:"name"          bson:"name"`
	Address     string `json:"address"       bson:"address"`
	DisplayName string `json:"display_name"  bson:"display_name"`
	Theme       string `json:"theme"         bson:"theme"`
	CreatedAt   int64  `json:"created_at"    bson:"created_at"`
	UpdatedAt   int64  `json:"updated_at"    bson:"updated_at"`
	DeletedAt   int64  `json:"deleted_at"    bson:"deleted_at"`
}

func (EmailService) TableName() string {
	return "email_service"
}
