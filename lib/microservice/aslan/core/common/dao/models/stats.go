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

type Stats struct {
	ReqID     string            `bson:"req_id"         json:"req_id"`
	User      string            `bson:"user"           json:"user"`
	Event     string            `bson:"event"          json:"event"`
	StartTime int64             `bson:"start_time"     json:"start_time"`
	EndTime   int64             `bson:"end_time"       json:"end_time"`
	Context   map[string]string `bson:"ctx"            json:"ctx"`
}

func (Stats) TableName() string {
	return "statistics"
}
