/*
Copyright 2024 The KodeRover Authors.

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

type WeeklyRollbackStat struct {
	ProjectKey string `bson:"project_key"                       json:"project_key,omitempty"`
	Production bool   `bson:"production"                        json:"production"`
	Rollback   int    `bson:"rollback"                          json:"rollback"`
	Date       string `bson:"date"                              json:"date"`
	CreateTime int64  `bson:"create_time"                       json:"create_time"`
	UpdateTime int64  `bson:"update_time"                       json:"update_time"`
}

func (WeeklyRollbackStat) TableName() string {
	return "rollback_stat_weekly"
}
