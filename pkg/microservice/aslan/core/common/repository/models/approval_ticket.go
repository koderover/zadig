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

import (
	"fmt"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"k8s.io/apimachinery/pkg/util/sets"
)

type ApprovalTicket struct {
	ID primitive.ObjectID `bson:"_id,omitempty"                json:"id,omitempty"`

	ApprovalID string `bson:"approval_id" json:"approval_id"`
	Status     int    `bson:"status"      json:"status"`

	ProjectKey           string                `bson:"project_key" json:"project_key"`
	Envs                 []string              `bson:"envs" json:"envs"`
	Services             []*ServiceWithModule  `bson:"services" json:"services"`
	Users                []*ApprovalTicketUser `bson:"users" json:"users"`
	ExecutionWindowStart int64                 `bson:"execution_window_start" json:"execution_window_start"`
	ExecutionWindowEnd   int64                 `bson:"execution_window_end" json:"execution_window_end"`

	CreateTime int64 `bson:"create_time" json:"create_time"`
}

type ApprovalTicketUser struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (ApprovalTicket) TableName() string {
	return "approval_ticket"
}

func (ticket *ApprovalTicket) Validate() (bool, error) {
	if len(ticket.ApprovalID) == 0 {
		return false, fmt.Errorf("approval_id cannot be empty")
	}

	if len(ticket.ProjectKey) == 0 {
		return false, fmt.Errorf("project_key cannot be empty")
	}

	if ticket.ExecutionWindowStart > ticket.ExecutionWindowEnd && ticket.ExecutionWindowEnd != 0 {
		return false, fmt.Errorf("invalid time window: start time greater than end time")
	}

	return true, nil
}

func (ticket *ApprovalTicket) IsAllowedEnv(projectName, envName string) bool {
	if ticket == nil {
		return true
	}

	if len(ticket.Envs) == 0 {
		return true
	}

	if len(ticket.ProjectKey) != 0 && ticket.ProjectKey != projectName {
		return false
	}

	allowedSets := sets.NewString(ticket.Envs...)
	return allowedSets.Has(envName)
}

func (ticket *ApprovalTicket) IsAllowedService(projectName, serviceName, serviceModule string) bool {
	if ticket == nil {
		return true
	}

	if len(ticket.Services) == 0 {
		return true
	}

	if len(ticket.ProjectKey) != 0 && ticket.ProjectKey != projectName {
		return false
	}

	svcSets := sets.NewString()
	for _, allowedSvc := range ticket.Services {
		key := fmt.Sprintf("%s++%s", allowedSvc.ServiceName, allowedSvc.ServiceModule)
		svcSets.Insert(key)
	}

	return svcSets.Has(fmt.Sprintf("%s++%s", serviceName, serviceModule))
}
