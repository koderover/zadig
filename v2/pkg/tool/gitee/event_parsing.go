/*
Copyright 2022 The KodeRover Authors.

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

package gitee

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// EventType represents a gitee event type.
type EventType string

// List of available event types.
const (
	EventTypeMergeRequest EventType = "Merge Request Hook"
	EventTypePush         EventType = "Push Hook"
	EventTypeTagPush      EventType = "Tag Push Hook"
)

const eventTypeHeader = "X-Gitee-Event"

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
		event = &PullRequestEvent{}
	case EventTypePush:
		event = &PushEvent{}
	case EventTypeTagPush:
		event = &TagPushEvent{}
	default:
		return nil, fmt.Errorf("unexpected event type: %s", eventType)
	}

	if err := json.Unmarshal(payload, event); err != nil {
		return nil, err
	}

	return event, nil
}

type TagPushEvent struct {
	Ref                string                 `json:"ref"`
	Before             string                 `json:"before"`
	After              string                 `json:"after"`
	Created            bool                   `json:"created"`
	Deleted            bool                   `json:"deleted"`
	Compare            string                 `json:"compare"`
	Commits            []TagPushEventCommit   `json:"commits"`
	HeadCommit         TagPushEventHeadCommit `json:"head_commit"`
	TotalCommitsCount  int                    `json:"total_commits_count"`
	CommitsMoreThanTen bool                   `json:"commits_more_than_ten"`
	Repository         EventRepo              `json:"repository"`
	Project            TagPushEventProject    `json:"project"`
	UserID             int                    `json:"user_id"`
	UserName           string                 `json:"user_name"`
	User               EventUser              `json:"user"`
	Pusher             EventUser              `json:"pusher"`
	Sender             EventUser              `json:"sender"`
	Enterprise         EventEnterprise        `json:"enterprise"`
	HookName           string                 `json:"hook_name"`
	HookID             int                    `json:"hook_id"`
	HookURL            string                 `json:"hook_url"`
	Password           string                 `json:"password"`
	Timestamp          interface{}            `json:"timestamp"`
	Sign               string                 `json:"sign"`
}

type TagPushEventCommit struct {
	ID        string      `json:"id"`
	TreeID    string      `json:"tree_id"`
	ParentIds []string    `json:"parent_ids"`
	Distinct  bool        `json:"distinct"`
	Message   string      `json:"message"`
	Timestamp time.Time   `json:"timestamp"`
	URL       string      `json:"url"`
	Author    EventUser   `json:"author"`
	Committer EventUser   `json:"committer"`
	Added     interface{} `json:"added"`
	Removed   interface{} `json:"removed"`
	Modified  []string    `json:"modified"`
}

type TagPushEventHeadCommit struct {
	ID        string      `json:"id"`
	TreeID    string      `json:"tree_id"`
	ParentIds []string    `json:"parent_ids"`
	Distinct  bool        `json:"distinct"`
	Message   string      `json:"message"`
	Timestamp time.Time   `json:"timestamp"`
	URL       string      `json:"url"`
	Author    EventUser   `json:"author"`
	Committer EventUser   `json:"committer"`
	Added     interface{} `json:"added"`
	Removed   interface{} `json:"removed"`
	Modified  []string    `json:"modified"`
}

type TagPushEventProject struct {
	ID                int         `json:"id"`
	Name              string      `json:"name"`
	Path              string      `json:"path"`
	FullName          string      `json:"full_name"`
	Owner             EventUser   `json:"owner"`
	Private           bool        `json:"private"`
	HTMLURL           string      `json:"html_url"`
	URL               string      `json:"url"`
	Description       string      `json:"description"`
	Fork              bool        `json:"fork"`
	CreatedAt         time.Time   `json:"created_at"`
	UpdatedAt         time.Time   `json:"updated_at"`
	PushedAt          time.Time   `json:"pushed_at"`
	GitURL            string      `json:"git_url"`
	SSHURL            string      `json:"ssh_url"`
	CloneURL          string      `json:"clone_url"`
	SvnURL            string      `json:"svn_url"`
	GitHTTPURL        string      `json:"git_http_url"`
	GitSSHURL         string      `json:"git_ssh_url"`
	GitSvnURL         string      `json:"git_svn_url"`
	Homepage          interface{} `json:"homepage"`
	StargazersCount   int         `json:"stargazers_count"`
	WatchersCount     int         `json:"watchers_count"`
	ForksCount        int         `json:"forks_count"`
	Language          string      `json:"language"`
	HasIssues         bool        `json:"has_issues"`
	HasWiki           bool        `json:"has_wiki"`
	HasPages          bool        `json:"has_pages"`
	License           interface{} `json:"license"`
	OpenIssuesCount   int         `json:"open_issues_count"`
	DefaultBranch     string      `json:"default_branch"`
	Namespace         string      `json:"namespace"`
	NameWithNamespace string      `json:"name_with_namespace"`
	PathWithNamespace string      `json:"path_with_namespace"`
}

type PushEvent struct {
	Ref                string            `json:"ref"`
	Before             string            `json:"before"`
	After              string            `json:"after"`
	Created            bool              `json:"created"`
	Deleted            bool              `json:"deleted"`
	Compare            string            `json:"compare"`
	Commits            []PushEventCommit `json:"commits"`
	TotalCommitsCount  int               `json:"total_commits_count"`
	CommitsMoreThanTen bool              `json:"commits_more_than_ten"`
	Repository         EventRepo         `json:"repository"`
	UserID             int               `json:"user_id"`
	UserName           string            `json:"user_name"`
	Pusher             EventUser         `json:"pusher"`
	Enterprise         EventEnterprise   `json:"enterprise"`
	HookName           string            `json:"hook_name"`
	HookID             int               `json:"hook_id"`
	HookURL            string            `json:"hook_url"`
	Password           string            `json:"password"`
	Timestamp          interface{}       `json:"timestamp"`
	Sign               string            `json:"sign"`
}

type PushEventCommit struct {
	ID        string    `json:"id"`
	TreeID    string    `json:"tree_id"`
	ParentIds []string  `json:"parent_ids"`
	Distinct  bool      `json:"distinct"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	URL       string    `json:"url"`
	Author    EventUser `json:"author"`
	Committer EventUser `json:"committer"`
	Added     []string  `json:"added,omitempty"`
	Removed   []string  `json:"removed,omitempty"`
	Modified  []string  `json:"modified,omitempty"`
}

type PullRequestEvent struct {
	Action         string                       `json:"action"`
	ActionDesc     string                       `json:"action_desc"`
	Project        PullRequestEventProject      `json:"project"`
	PullRequest    *PullRequestEventPullRequest `json:"pull_request"`
	Number         int                          `json:"number"`
	Iid            int                          `json:"iid"`
	Title          string                       `json:"title"`
	Body           interface{}                  `json:"body"`
	Languages      []string                     `json:"languages"`
	State          string                       `json:"state"`
	MergeStatus    string                       `json:"merge_status"`
	MergeCommitSha string                       `json:"merge_commit_sha"`
	URL            string                       `json:"url"`
	SourceBranch   string                       `json:"source_branch"`
	TargetBranch   string                       `json:"target_branch"`
	Repository     PullRequestEventRepository   `json:"repository"`
	Author         EventUser                    `json:"author"`
	UpdatedBy      EventUser                    `json:"updated_by"`
	Sender         EventUser                    `json:"sender"`
	TargetUser     EventUser                    `json:"target_user"`
	Enterprise     EventEnterprise              `json:"enterprise"`
	HookName       string                       `json:"hook_name"`
	HookID         int                          `json:"hook_id"`
	HookURL        string                       `json:"hook_url"`
	Password       string                       `json:"password"`
	Timestamp      interface{}                  `json:"timestamp"`
	Sign           string                       `json:"sign"`
}

type PullRequestEventProject struct {
	ID                int         `json:"id"`
	Language          interface{} `json:"language"`
	License           interface{} `json:"license"`
	Name              string      `json:"name"`
	NameWithNamespace string      `json:"name_with_namespace"`
	Namespace         string      `json:"namespace"`
	OpenIssuesCount   int         `json:"open_issues_count"`
	Path              string      `json:"path"`
	PathWithNamespace string      `json:"path_with_namespace"`
	Private           bool        `json:"private"`
	PushedAt          time.Time   `json:"pushed_at"`
	SSHURL            string      `json:"ssh_url"`
	StargazersCount   int         `json:"stargazers_count"`
	SvnURL            string      `json:"svn_url"`
	UpdatedAt         time.Time   `json:"updated_at"`
	URL               string      `json:"url"`
	WatchersCount     int         `json:"watchers_count"`
}

type PullRequestEventPullRequest struct {
	ID             int                   `json:"id"`
	Number         int                   `json:"number"`
	State          string                `json:"state"`
	HTMLURL        string                `json:"html_url"`
	DiffURL        string                `json:"diff_url"`
	PatchURL       string                `json:"patch_url"`
	Title          string                `json:"title"`
	Body           interface{}           `json:"body"`
	Languages      []string              `json:"languages"`
	CreatedAt      time.Time             `json:"created_at"`
	UpdatedAt      time.Time             `json:"updated_at"`
	ClosedAt       interface{}           `json:"closed_at"`
	MergedAt       interface{}           `json:"merged_at"`
	MergeCommitSha string                `json:"merge_commit_sha"`
	User           EventUser             `json:"user"`
	Head           *PullRequestEventBase `json:"head"`
	Base           PullRequestEventBase  `json:"base"`
	Merged         bool                  `json:"merged"`
	Mergeable      interface{}           `json:"mergeable"`
	MergeStatus    string                `json:"merge_status"`
	Comments       int                   `json:"comments"`
	Commits        int                   `json:"commits"`
	Additions      int                   `json:"additions"`
	Deletions      int                   `json:"deletions"`
	ChangedFiles   int                   `json:"changed_files"`
}

type PullRequestEventBase struct {
	Label string    `json:"label"`
	Ref   string    `json:"ref"`
	Sha   string    `json:"sha"`
	User  EventUser `json:"user"`
	Repo  EventRepo `json:"repo"`
}

type EventRepo struct {
	ID                int         `json:"id"`
	Name              string      `json:"name"`
	Path              string      `json:"path"`
	FullName          string      `json:"full_name"`
	Owner             EventUser   `json:"owner"`
	Private           bool        `json:"private"`
	HTMLURL           string      `json:"html_url"`
	URL               string      `json:"url"`
	Description       string      `json:"description"`
	Fork              bool        `json:"fork"`
	CreatedAt         time.Time   `json:"created_at"`
	UpdatedAt         time.Time   `json:"updated_at"`
	PushedAt          time.Time   `json:"pushed_at"`
	GitURL            string      `json:"git_url"`
	SSHURL            string      `json:"ssh_url"`
	CloneURL          string      `json:"clone_url"`
	SvnURL            string      `json:"svn_url"`
	GitHTTPURL        string      `json:"git_http_url"`
	GitSSHURL         string      `json:"git_ssh_url"`
	GitSvnURL         string      `json:"git_svn_url"`
	Homepage          interface{} `json:"homepage"`
	StargazersCount   int         `json:"stargazers_count"`
	WatchersCount     int         `json:"watchers_count"`
	ForksCount        int         `json:"forks_count"`
	Language          string      `json:"language"`
	HasIssues         bool        `json:"has_issues"`
	HasWiki           bool        `json:"has_wiki"`
	HasPages          bool        `json:"has_pages"`
	License           interface{} `json:"license"`
	OpenIssuesCount   int         `json:"open_issues_count"`
	DefaultBranch     string      `json:"default_branch"`
	Namespace         string      `json:"namespace"`
	NameWithNamespace string      `json:"name_with_namespace"`
	PathWithNamespace string      `json:"path_with_namespace"`
}

type PullRequestEventRepository struct {
	ID                int         `json:"id"`
	Name              string      `json:"name"`
	Path              string      `json:"path"`
	FullName          string      `json:"full_name"`
	Owner             EventUser   `json:"owner"`
	Private           bool        `json:"private"`
	HTMLURL           string      `json:"html_url"`
	URL               string      `json:"url"`
	Description       string      `json:"description"`
	Fork              bool        `json:"fork"`
	CreatedAt         time.Time   `json:"created_at"`
	UpdatedAt         time.Time   `json:"updated_at"`
	PushedAt          time.Time   `json:"pushed_at"`
	GitURL            string      `json:"git_url"`
	SSHURL            string      `json:"ssh_url"`
	CloneURL          string      `json:"clone_url"`
	SvnURL            string      `json:"svn_url"`
	GitHTTPURL        string      `json:"git_http_url"`
	GitSSHURL         string      `json:"git_ssh_url"`
	GitSvnURL         string      `json:"git_svn_url"`
	Homepage          interface{} `json:"homepage"`
	StargazersCount   int         `json:"stargazers_count"`
	WatchersCount     int         `json:"watchers_count"`
	ForksCount        int         `json:"forks_count"`
	Language          string      `json:"language"`
	HasIssues         bool        `json:"has_issues"`
	HasWiki           bool        `json:"has_wiki"`
	HasPages          bool        `json:"has_pages"`
	License           interface{} `json:"license"`
	OpenIssuesCount   int         `json:"open_issues_count"`
	DefaultBranch     string      `json:"default_branch"`
	Namespace         string      `json:"namespace"`
	NameWithNamespace string      `json:"name_with_namespace"`
	PathWithNamespace string      `json:"path_with_namespace"`
}

type EventEnterprise struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type EventUser struct {
	Login     string `json:"login"`
	AvatarURL string `json:"avatar_url"`
	HTMLURL   string `json:"html_url"`
	Type      string `json:"type"`
	SiteAdmin bool   `json:"site_admin"`
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Username  string `json:"username"`
	UserName  string `json:"user_name"`
	URL       string `json:"url"`
}
