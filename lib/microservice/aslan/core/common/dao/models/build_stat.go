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

type BuildStat struct {
	ProductName         string        `bson:"product_name"            json:"productName"`
	TotalSuccess        int           `bson:"total_success"           json:"totalSuccess"`
	TotalFailure        int           `bson:"total_failure"           json:"totalFailure"`
	TotalTimeout        int           `bson:"total_timeout"           json:"totalTimeout"`
	TotalDuration       int64         `bson:"total_duration"          json:"totalDuration"`
	TotalBuildCount     int           `bson:"total_build_count"       json:"totalBuildCount"`
	MaxDuration         int64         `bson:"max_duration"            json:"maxDuration"`
	MaxDurationPipeline *PipelineInfo `bson:"max_duration_pipeline"   json:"maxDurationPipeline"`
	Date                string        `bson:"date"                    json:"date"`
	CreateTime          int64         `bson:"create_time"             json:"createTime"`
	UpdateTime          int64         `bson:"update_time"             json:"updateTime"`
}

// PipelineInfo
type PipelineInfo struct {
	TaskID       int64  `bson:"task_id"                 json:"taskId"`
	PipelineName string `bson:"pipeline_name"           json:"pipelineName"`
	Type         string `bson:"type"                    json:"type"`
	MaxDuration  int64  `bson:"max_duration"            json:"maxDuration"`
}

func (BuildStat) TableName() string {
	return "build_stat"
}
