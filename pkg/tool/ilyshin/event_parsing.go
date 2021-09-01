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

package ilyshin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type ProjectEvent struct {
	ID                int    `json:"id"`
	Name              string `json:"name"`
	Description       string `json:"description"`
	AvatarURL         string `json:"avatar_url"`
	GitSSHURL         string `json:"git_ssh_url"`
	GitHTTPURL        string `json:"git_http_url"`
	Namespace         string `json:"namespace"`
	PathWithNamespace string `json:"path_with_namespace"`
	DefaultBranch     string `json:"default_branch"`
	Homepage          string `json:"homepage"`
	URL               string `json:"url"`
	SSHURL            string `json:"ssh_url"`
	HTTPURL           string `json:"http_url"`
	WebURL            string `json:"web_url"`
}

type Repository struct {
	Name              string `json:"name"`
	Description       string `json:"description"`
	WebURL            string `json:"web_url"`
	AvatarURL         string `json:"avatar_url"`
	GitSSHURL         string `json:"git_ssh_url"`
	GitHTTPURL        string `json:"git_http_url"`
	Namespace         string `json:"namespace"`
	PathWithNamespace string `json:"path_with_namespace"`
	DefaultBranch     string `json:"default_branch"`
	Homepage          string `json:"homepage"`
	URL               string `json:"url"`
	SSHURL            string `json:"ssh_url"`
	HTTPURL           string `json:"http_url"`
}

type ObjectAttributes struct {
	ID                       int         `json:"id"`
	TargetBranch             string      `json:"target_branch"`
	SourceBranch             string      `json:"source_branch"`
	SourceProjectID          int         `json:"source_project_id"`
	AuthorID                 int         `json:"author_id"`
	AssigneeID               int         `json:"assignee_id"`
	AssigneeIDs              []int       `json:"assignee_ids"`
	Title                    string      `json:"title"`
	StCommits                []*Commit   `json:"st_commits"`
	MilestoneID              int         `json:"milestone_id"`
	State                    string      `json:"state"`
	MergeStatus              string      `json:"merge_status"`
	TargetProjectID          int         `json:"target_project_id"`
	IID                      int         `json:"iid"`
	Description              string      `json:"description"`
	Position                 int         `json:"position"`
	LockedAt                 string      `json:"locked_at"`
	UpdatedByID              int         `json:"updated_by_id"`
	MergeError               string      `json:"merge_error"`
	MergeWhenBuildSucceeds   bool        `json:"merge_when_build_succeeds"`
	MergeUserID              int         `json:"merge_user_id"`
	MergeCommitSHA           string      `json:"merge_commit_sha"`
	DeletedAt                string      `json:"deleted_at"`
	ApprovalsBeforeMerge     string      `json:"approvals_before_merge"`
	RebaseCommitSHA          string      `json:"rebase_commit_sha"`
	InProgressMergeCommitSHA string      `json:"in_progress_merge_commit_sha"`
	LockVersion              int         `json:"lock_version"`
	TimeEstimate             int         `json:"time_estimate"`
	Target                   *Repository `json:"target"`
	LastCommit               LastCommit  `json:"last_commit"`
	WorkInProgress           bool        `json:"work_in_progress"`
	URL                      string      `json:"url"`
	Action                   string      `json:"action"`
	OldRev                   string      `json:"oldrev"`
}

type LastCommit struct {
	ID        string     `json:"id"`
	Message   string     `json:"message"`
	Timestamp *time.Time `json:"timestamp"`
	URL       string     `json:"url"`
	Author    Author     `json:"author"`
}

type MergeEvent struct {
	ObjectKind       string           `json:"object_kind"`
	Project          ProjectEvent     `json:"project"`
	ObjectAttributes ObjectAttributes `json:"object_attributes"`
}

type PushEvent struct {
	ObjectKind        string       `json:"object_kind"`
	Before            string       `json:"before"`
	After             string       `json:"after"`
	Ref               string       `json:"ref"`
	CheckoutSHA       string       `json:"checkout_sha"`
	UserID            int          `json:"user_id"`
	UserName          string       `json:"user_name"`
	UserUsername      string       `json:"user_username"`
	UserEmail         string       `json:"user_email"`
	UserAvatar        string       `json:"user_avatar"`
	ProjectID         int          `json:"project_id"`
	Project           ProjectEvent `json:"project"`
	Commits           []Commits    `json:"commits"`
	TotalCommitsCount int          `json:"total_commits_count"`
}

type Commits struct {
	ID        string     `json:"id"`
	Message   string     `json:"message"`
	Timestamp *time.Time `json:"timestamp"`
	URL       string     `json:"url"`
	Author    Author     `json:"author"`
	Added     []string   `json:"added"`
	Modified  []string   `json:"modified"`
	Removed   []string   `json:"removed"`
}

type Author struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// EventType represents a ilyshin event type.
type EventType string

// List of available event types.
const (
	EventTypeMergeRequest EventType = "Merge Request Hook"
	EventTypePush         EventType = "Push Hook"
	EventTypeTagPush      EventType = "Tag Push Hook"
)

const eventTypeHeader = "X-CodeHub-Event"

// HookEventType returns the event type for the given request.
func HookEventType(r *http.Request) EventType {
	return EventType(r.Header.Get(eventTypeHeader))
}

func ParseHook(eventType EventType, payload []byte) (event interface{}, err error) {
	return parseWebhook(eventType, payload)
}

func parseWebhook(eventType EventType, payload []byte) (event interface{}, err error) {
	switch eventType {
	case EventTypeMergeRequest:
		event = &MergeEvent{}
	case EventTypePush:
		event = &PushEvent{}
	default:
		return nil, fmt.Errorf("unexpected event type: %s", eventType)
	}

	if err := json.Unmarshal(payload, event); err != nil {
		return nil, err
	}

	return event, nil
}
