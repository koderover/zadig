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

import (
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/koderover/zadig/v2/pkg/setting"
)

type WorkflowtemplatePreView struct {
	ID           primitive.ObjectID       `json:"id"`
	TemplateName string                   `json:"template_name"`
	CreateBy     string                   `json:"create_by"`
	CreateTime   int64                    `json:"create_time"`
	UpdateBy     string                   `json:"update_by"`
	UpdateTime   int64                    `json:"update_time"`
	Stages       []string                 `json:"stages"`
	StageDetails []*WorkflowTemplateStage `json:"stage_details"`
	Description  string                   `json:"description"`
	Category     setting.WorkflowCategory `json:"category"`
	BuildIn      bool                     `json:"build_in"`
}

type WorkflowTemplateStage struct {
	Name string                 `json:"name"`
	Jobs []*WorkflowTemplateJob `json:"jobs"`
}

type WorkflowTemplateJob struct {
	Name    string `json:"name"`
	JobType string `json:"job_type"`
}
