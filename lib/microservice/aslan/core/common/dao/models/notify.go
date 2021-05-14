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

	"github.com/koderover/zadig/lib/microservice/aslan/config"
)

type Notify struct {
	ID         primitive.ObjectID `bson:"_id,omitempty"           json:"id,omitempty"`  // 主键
	Type       config.NotifyType  `bson:"type"                      json:"type"`        // 消息类型
	Receiver   string             `bson:"receiver"                  json:"receiver"`    // 发送者
	Content    interface{}        `bson:"content"                   json:"content"`     // 消息内容
	CreateTime int64              `bson:"create_time"               json:"create_time"` // 消息创建时间
	IsRead     bool               `bson:"is_read"                   json:"is_read"`     // 是否已读
}

type AnnouncementCtx struct {
	Title     string `bson:"title"                 json:"title"`      // 公告标题
	Priority  int    `bson:"priority"              json:"priority"`   // 公告级别
	Content   string `bson:"content"               json:"content"`    // 公告内容
	StartTime int64  `bson:"start_time"            json:"start_time"` // 公告开始时间
	EndTime   int64  `bson:"end_time"              json:"end_time"`   // 公告结束时间
}

type PipelineStatusCtx struct {
	TaskID       int64               `bson:"task_id"                   json:"task_id"`
	ProductName  string              `bson:"product_name"              json:"product_name"`
	PipelineName string              `bson:"pipeline_name"             json:"pipeline_name"`
	Type         config.PipelineType `bson:"type"                      json:"type"`
	Status       config.Status       `bson:"status"                    json:"status,omitempty"`
	TeamName     string              `bson:"team"                      json:"team"`
}

type MessageCtx struct {
	ReqID   string `bson:"req_id"                json:"req_id"`
	Title   string `bson:"title"                 json:"title"`   // 消息标题
	Content string `bson:"content"               json:"content"` // 消息内容
}

func (Notify) TableName() string {
	return "notify"
}
