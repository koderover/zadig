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

package jira

import (
	"strings"
	"time"
)

// IssueFields is jira issue fields
type IssueFields []string

// NormalIssueFields is normal issue fields
// customfield_10001 represents epic link value
var NormalIssueFields = IssueFields{
	"issuetype",
	"project",
	"created",
	"updated",
	"priority",
	"issuelinks",
	"status",
	"resolution",
	"summary",
	"description",
	"creator",
	"assignee",
	"reporter",
	"priority",
	"components",
	"customfield_10001",
}

// TODO define sprint issue fields

// String join issue fields
func (ifs IssueFields) String() string {
	return strings.Join(ifs, ",")
}

// Issue IssueResult:Issues
type Issue struct {
	ID     string  `bson:"id,omitempty"     json:"id,omitempty"`
	Key    string  `bson:"key,omitempty"    json:"key,omitempty"`
	Self   string  `bson:"self,omitempty"   json:"self,omitempty"`
	Fields *Fields `bson:"fields,omitempty" json:"fields,omitempty"`
}

// IssueType is issue type
type IssueType struct {
	ID          string `bson:"id,omitempty"          json:"id,omitempty"`
	Self        string `bson:"self,omitempty"        json:"self,omitempty"`
	Name        string `bson:"name,omitempty"        json:"name,omitempty"`
	Description string `bson:"description,omitempty" json:"description,omitempty"`
	IconURL     string `bson:"iconUrl,omitempty"     json:"iconUrl,omitempty"`
}

// Fields IssueResult:Issues:Fields
type Fields struct {
	Summary          string       `bson:"summary,omitempty"               json:"summary,omitempty"`
	Description      string       `bson:"description,omitempty"           json:"description,omitempty"`
	IssueType        *IssueType   `bson:"issuetype,omitempty"             json:"issuetype,omitempty"`
	IssueLinks       []*LinkIssue `bson:"issuelinks,omitempty"            json:"issuelinks,omitempty"`
	Project          *Project     `bson:"project,omitempty"               json:"project,omitempty"`
	CustomField10001 string       `bson:"customfield_10001,omitempty"     json:"customfield_10001,omitempty"` //epic link
	Creator          *User        `bson:"creator,omitempty"               json:"creator,omitempty"`
	Assignee         *User        `bson:"assignee,omitempty"              json:"assignee,omitempty"`
	Reporter         *User        `bson:"reporter,omitempty"              json:"reporter,omitempty"`
	Priority         *Priority    `bson:"priority,omitempty"              json:"priority,omitempty"`
	Status           *Status      `bson:"status,omitempty"                json:"status,omitempty"`
	Components       []*Component `bson:"components,omitempty"            json:"components,omitempty"`
}

// LinkIssue IssueResult:Issues:Field:LinkIssues
type LinkIssue struct {
	ID           string `bson:"id,omitempty"           json:"id,omitempty"`
	InwardIssue  *Issue `bson:"inwardIssue,omitempty"  json:"inwardIssue,omitempty"`
	OutwardIssue *Issue `bson:"outwardIssue,omitempty" json:"outwardIssue,omitempty"`
}

// Status ...
type Status struct {
	ID             string          `bson:"id,omitempty"                 json:"id,omitempty"`
	Name           string          `bson:"name,omitempty"               json:"name,omitempty"`
	StatusCategory *StatusCategory `bson:"statusCategory,omitempty"     json:"statusCategory,omitempty"`
}

// StatusCategory ...
type StatusCategory struct {
	ID  int    `bson:"id,omitempty"                       json:"id,omitempty"`
	Key string `bson:"key,omitempty"                      json:"key,omitempty"`
}

// Priority is jira Priority
type Priority struct {
	ID          string `bson:"id,omitempty"          json:"id,omitempty"`
	Name        string `bson:"name,omitempty"        json:"name,omitempty"`
	Self        string `bson:"self,omitempty"        json:"self,omitempty"`
	Description string `bson:"description,omitempty" json:"description,omitempty"`
}

// User is jira user
type User struct {
	Name        string `bson:"name,omitempty"        json:"name,omitempty"`
	Key         string `bson:"key,omitempty"         json:"key,omitempty"`
	Self        string `bson:"self,omitempty"        json:"self,omitempty"`
	DisplayName string `bson:"displayName,omitempty" json:"displayName,omitempty"`
}

// BoardsList ...
type BoardsList struct {
	MaxResults int      `json:"maxResults"`
	StartAt    int      `json:"startAt"`
	Total      int      `json:"total"`
	IsLast     bool     `json:"isLast"`
	Values     []*Board `json:"values"`
}

// Board ...
type Board struct {
	ID   int    `json:"id,omitempty"`
	Self string `json:"self,omitempty"`
	Name string `json:"name,omitempty"`
	Type string `json:"type,omitempty"`
}

// SprintsList ...
type SprintsList struct {
	MaxResults int       `json:"maxResults"`
	StartAt    int       `json:"startAt"`
	IsLast     bool      `json:"isLast"`
	Values     []*Sprint `json:"values"`
}

// Sprint represents a JIRA agile board
type Sprint struct {
	ID            int       `json:"id,omitempty"`
	Self          string    `json:"self,omitempty"`
	State         string    `json:"state,omitempty"`
	Name          string    `json:"name,omitempty"`
	StartDate     time.Time `json:"startDate,omitempty"`
	EndDate       time.Time `json:"endDate,omitempty"`
	CompleteDate  time.Time `json:"completeDate,omitempty"`
	OriginBoardID int       `json:"originBoardId,omitempty"`
}

// IssueResult ...
type IssueResult struct {
	Total      int      `bson:"total,omitempty"      json:"total,omitempty"`
	MaxResults int      `bson:"maxResults,omitempty" json:"maxResults,omitempty"`
	Issues     []*Issue `bson:"issues,omitempty"     json:"issues,omitempty"`
}

// Component ...
type Component struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}
