/*
Copyright 2023 The KodeRover Authors.

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

package service

import "github.com/koderover/zadig/pkg/util"

type OpenAPIStatV2 struct {
	Total        int64        `json:"total"`
	SuccessCount int64        `json:"success_count"`
	DailyStat    []*DailyStat `json:"daily_stat"`
}

type DailyStat struct {
	Date         string `json:"date"`
	Total        int64  `json:"total"`
	SuccessCount int64  `json:"success_count"`
	FailCount    int64  `json:"fail_count"`
}

type StatDashboardConfig struct {
	Type      string     `json:"type"`
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Source    string     `json:"source"`
	APIConfig *APIConfig `json:"api_config,omitempty"`
	Function  string     `json:"function"`
	Weight    int64      `json:"weight"`
}

type APIConfig struct {
	ExternalSystemId string           `json:"external_system_id"`
	ApiPath          string           `json:"api_path"`
	Queries          []*util.KeyValue `json:"queries"`
}

type StatDashboardByProject struct {
	ProjectKey  string               `json:"project_key"`
	ProjectName string               `json:"project_name"`
	Score       float64              `json:"score"`
	Facts       []*StatDashboardItem `json:"facts"`
}

type StatDashboardItem struct {
	Type     string      `json:"type"`
	ID       string      `json:"id"`
	Data     interface{} `json:"data"`
	Score    float64     `json:"score"`
	Error    string      `json:"error,omitempty"`
	HasValue bool        `json:"has_value"`
}

type StatDashboardBasicData struct {
	BuildTotal    int64 `json:"build_total"`
	BuildSuccess  int64 `json:"build_success"`
	TestTotal     int64 `json:"test_total"`
	TestSuccess   int64 `json:"test_success"`
	DeployTotal   int64 `json:"deploy_total"`
	DeploySuccess int64 `json:"deploy_success"`
}

type JobInfo struct {
	//ID                  primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	//Type                string             `bson:"type" json:"type"`
	//WorkflowName        string             `bson:"workflow_name" json:"workflow_name"`
	//WorkflowDisplayName string             `bson:"workflow_display_name" json:"workflow_display_name"`
	//TaskID              int64              `bson:"task_id" json:"task_id"`
	//ProductName         string             `bson:"product_name" json:"product_name"`
	//Status              string             `bson:"status" json:"status"`
	//StartTime           int64              `bson:"start_time" json:"start_time"`
	//EndTime             int64              `bson:"end_time" json:"end_time"`
	//Duration            int64              `bson:"duration" json:"duration"`
	//ServiceType         string             `bson:"service_type" json:"service_type"`
	//ServiceName         string             `bson:"service_name" json:"service_name"`
	//ServiceModule       string             `bson:"service_module" json:"service_module"`
	//Production          bool               `bson:"production" json:"production"`
	//TargetEnv           string             `bson:"target_env" json:"target_env"`
	WorkflowName        string
	WorkflowDisplayName string
	ProductName         string
	Production          bool
	ServiceType         string
	ServiceName         string
	ServiceModule       string
	TargetEnv           string
	Type                string
	StartTime           int64
	EndTime             int64
	Duration            int64
	Status              string
}
