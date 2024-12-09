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
)

type ApprovalTicket struct {
	ID primitive.ObjectID `bson:"_id,omitempty"                json:"id,omitempty"`

	ApprovalID string `json:"approval_id"`
	Status     int    `json:"status"`

	ProjectKey           string                `json:"project_key"`
	Envs                 []string              `json:"envs"`
	Services             []*ServiceWithModule  `json:"services"`
	Users                []*ApprovalTicketUser `json:"users"`
	ExecutionWindowStart int64                 `json:"execution_window_start"`
	ExecutionWindowEnd   int64                 `json:"execution_window_end"`

	CreateTime int64 `json:"create_time"`
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
