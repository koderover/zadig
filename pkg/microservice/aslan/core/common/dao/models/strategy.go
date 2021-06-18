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

// CapacityTarget 系统配额的配置对象
type CapacityTarget string

const (
	// 工作流任务的留存
	WorkflowTaskRetention CapacityTarget = "WorkflowTaskRetention"
)

// RetentionConfig 资源留存相关的配置
type RetentionConfig struct {
	MaxDays  int `bson:"max_days"      json:"max_days"`  // 最多几天
	MaxItems int `bson:"max_items"     json:"max_items"` // 最多几条
}

// CapacityStrategy 系统配额策略
type CapacityStrategy struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"  json:"id,omitempty"`
	Target    CapacityTarget     `bson:"target"         json:"target"` // 配额策略的对象，不重复
	Retention *RetentionConfig   `bson:"retention"      json:"retention,omitempty"`
}

func (CapacityStrategy) TableName() string {
	return "syscap_strategy"
}
