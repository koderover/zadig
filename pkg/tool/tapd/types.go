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

package tapd

type TokenData struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
	Now         string `json:"now"`
}

type Response struct {
	Status int         `json:"status"`
	Info   string      `json:"info"`
	Data   interface{} `json:"data"`
}

type GetCountResponse struct {
	Count int `json:"count"`
}

type ProjectInfo struct {
	Workspace *Workspace `json:"workspace"`
}

type Workspace struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	Category    string `json:"category"`
	ParentID    string `json:"parent_id"`
	BeginDate   string `json:"begin_date"`
	EndDate     string `json:"end_date"`
	Created     string `json:"created"`
	CreatorID   string `json:"creator_id"`
	Creator     string `json:"creator"`
	MemberCount int    `json:"member_count"`
}

type IterationInfo struct {
	Iteration *Iteration `json:"iteration"`
}

type Iteration struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	WorkspaceID    string `json:"workspace_id"`
	Description    string `json:"description"`
	StartDate      string `json:"startdate"`
	EndDate        string `json:"enddate"`
	Status         string `json:"status"`
	Creator        string `json:"creator"`
	Created        string `json:"created"`
	Modified       string `json:"modified"`
	Completed      string `json:"completed"`
	LockInfo       string `json:"lock_info"`
	Locker         string `json:"locker"`
	WorkitemTypeID string `json:"workitem_type_id"`
	PlanAppID      string `json:"plan_app_id"`
}

type UpdateIterationStatusRequest struct {
	WorkspaceID string `json:"workspace_id"`
	ID          string `json:"id"`
	CurrentUser string `json:"current_user"`
	Status      string `json:"status"`
}
