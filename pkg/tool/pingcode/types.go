/*
Copyright 2025 The KodeRover Authors.

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

package pingcode

type GetTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
}

type PaginationResponse struct {
	PageSize  int           `json:"page_size"`
	PageIndex int           `json:"page_index"`
	Total     int           `json:"total"`
	Values    []interface{} `json:"values"`
}

type ProjectInfo struct {
	ID          string `json:"id"`
	URL         string `json:"url"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Visibility  string `json:"visibility"`
	Description string `json:"description"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
	IsArchived  int    `json:"is_archived"`
	IsDeleted   int    `json:"is_deleted"`
}

type BoardInfo struct {
	ID   string `json:"id"`
	URL  string `json:"url"`
	Name string `json:"name"`
}

type SprintInfo struct {
	ID          string `json:"id"`
	URL         string `json:"url"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	Description string `json:"description"`
	StartAt     int64  `json:"start_at"`
	EndAt       int64  `json:"end_at"`
	CreateAt    int64  `json:"create_at"`
	UpdateAt    int64  `json:"update_at"`
}

type WorkItemType struct {
	ID    string `json:"id"`
	URL   string `json:"url"`
	Name  string `json:"name"`
	Group string `json:"group"`
}

type WorkItemStatePlan struct {
	ID           string `json:"id"`
	URL          string `json:"url"`
	WorkItemType string `json:"work_item_type"`
	ProjectType  string `json:"project_type"`
}

type WorkItemState struct {
	ID    string `json:"id"`
	URL   string `json:"url"`
	Name  string `json:"name"`
	Type  string `json:"type"`
	Color string `json:"color"`
}

type WorkItem struct {
	ID          string         `json:"id"`
	URL         string         `json:"url"`
	Identifier  string         `json:"identifier"`
	Title       string         `json:"title"`
	Type        string         `json:"type"`
	State       *WorkItemState `json:"state"`
	StartAt     int64          `json:"start_at"`
	EndAt       int64          `json:"end_at"`
	CompletedAt int64          `json:"completed_at"`
	Description string         `json:"description"`
}

type ListWorkItemStateFlows struct {
	ID        string             `json:"id"`
	URL       string             `json:"url"`
	StatePlan *WorkItemStatePlan `json:"state_plan"`
	FromState *WorkItemState     `json:"from_state"`
	ToState   *WorkItemState     `json:"to_state"`
}

type UpdateWorkItemStateRequest struct {
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	StateID     string `json:"state_id,omitempty"`
}
