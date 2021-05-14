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
)

type Cronjob struct {
	ID           primitive.ObjectID `bson:"_id,omitempty"`
	Name         string             `bson:"name"`
	Type         string             `bson:"type"`
	Number       uint64             `bson:"number"`
	Frequency    string             `bson:"frequency"`
	Time         string             `bson:"time"`
	Cron         string             `bson:"cron"`
	ProductName  string             `bson:"product_name,omitempty"`
	MaxFailure   int                `bson:"max_failures,omitempty"`
	TaskArgs     *TaskArgs          `bson:"task_args,omitempty"`
	WorkflowArgs *WorkflowTaskArgs  `bson:"workflow_args,omitempty"`
	TestArgs     *TestTaskArgs      `bson:"test_args,omitempty"`
	JobType      string             `bson:"job_type"`
	Enabled      bool               `bson:"enabled"`
}

func (Cronjob) TableName() string {
	return "cronjob"
}
