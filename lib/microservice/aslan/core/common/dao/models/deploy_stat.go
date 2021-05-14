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

type DeployStat struct {
	ProductName                 string `bson:"product_name"                      json:"productName"`
	TotalTaskSuccess            int    `bson:"total_task_success"                json:"totalTaskSuccess"`
	TotalTaskFailure            int    `bson:"total_task_failure"                json:"totalTaskFailure"`
	TotalDeploySuccess          int    `bson:"total_deploy_success"              json:"totalDeploySuccess"`
	TotalDeployFailure          int    `bson:"total_deploy_failure"              json:"totalDeployFailure"`
	MaxDeployServiceNum         int    `bson:"max_deploy_service_num"            json:"maxDeployServiceNum"`
	MaxDeployServiceFailureNum  int    `bson:"max_deploy_service_failure_num"    json:"maxDeployServiceFailureNum"`
	MaxDeployServiceName        string `bson:"max_deploy_service_name"           json:"maxDeployServiceName"`
	MaxDeployFailureServiceNum  int    `bson:"max_deploy_failure_service_num"    json:"maxDeployFailureServiceNum"`
	MaxDeployFailureServiceName string `bson:"max_deploy_failure_service_name"   json:"maxDeployFailureServiceName"`
	Date                        string `bson:"date"                              json:"date"`
	CreateTime                  int64  `bson:"create_time"                       json:"createTime"`
	UpdateTime                  int64  `bson:"update_time"                       json:"updateTime"`
}

func (DeployStat) TableName() string {
	return "deploy_stat"
}
