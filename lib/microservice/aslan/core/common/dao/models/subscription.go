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

import "github.com/koderover/zadig/lib/microservice/aslan/config"

type Subscription struct {
	Subscriber     string            `bson:"subscriber"                   json:"subscriber,omitempty"`     // 订阅人
	Type           config.NotifyType `bson:"type"                         json:"type"`                     // 消息类型
	CreateTime     int64             `bson:"create_time"                  json:"create_time,omitempty"`    // 创建时间
	PipelineStatus config.Status     `bson:"pipelinestatus"               json:"pipelinestatus,omitempty"` // pipeline 状态关注
}

func (Subscription) TableName() string {
	return "nofity_subscription"
}
