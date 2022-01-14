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

type PolicyType string

const (
	PolicyTypeSystem     PolicyType = "System"
	PolicyTypeUserDefine PolicyType = "UserDefine"
)

type PolicyDefine struct {
	Name           string     `bson:"name"                 json:"name"`
	Namespace      string     `bson:"namespace"            json:"namespace"`
	Alias          string     `bson:"alias"                json:"alias"`
	BindUserIDs    []string   `bson:"bind_user_ids"        json:"bind_user_ids"`
	BindUserGroups []string   `bson:"bind_user_groups"     json:"bind_user_groups"`
	Rules          []Rule     `bson:"rules"                json:"rules"`
	PolicyType     PolicyType `bson:"policy_type"          json:"policy_type"`
	CreateTime     int64      `bson:"create_time"          json:"create_time"`
	CreateBy       string     `bson:"create_by"            json:"create_by"`
}

func (PolicyDefine) TableName() string {
	return "policy_define"
}
