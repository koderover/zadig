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

type GithubApp struct {
	ID             primitive.ObjectID `bson:"_id,omitempty"   json:"id,omitempty"`
	AppID          int                `bson:"app_id"          json:"app_id"`
	AppKey         string             `bson:"app_key"         json:"app_key"`
	CreatedAt      int64              `bson:"created_at"      json:"created_at"`
	EnableGitCheck bool               `bson:"-"               json:"enable_git_check"`
}

func (GithubApp) TableName() string {
	return "github_app"
}
