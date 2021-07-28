package codehub

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// EventType represents a Codehub event type.
type EventType string

// List of available event types.
const (
	EventTypeMergeRequest EventType = "Merge Request Hook"
	EventTypePush         EventType = "Push Hook"
	EventTypeTagPush      EventType = "Tag Push Hook"
)

const eventTypeHeader = "X-Codehub-Event"

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

type User struct {
	Name      string `json:"name"`
	Username  string `json:"username"`
	AvatarURL string `json:"avatar_url"`
}

type WebhookProject struct {
	ID                int         `json:"id"`
	Name              string      `json:"name"`
	Description       string      `json:"description"`
	WebURL            string      `json:"web_url"`
	AvatarURL         interface{} `json:"avatar_url"`
	GitSSHURL         string      `json:"git_ssh_url"`
	GitHTTPURL        string      `json:"git_http_url"`
	Namespace         string      `json:"namespace"`
	VisibilityLevel   int         `json:"visibility_level"`
	PathWithNamespace string      `json:"path_with_namespace"`
	DefaultBranch     string      `json:"default_branch"`
	CiConfigPath      interface{} `json:"ci_config_path"`
	Homepage          string      `json:"homepage"`
	URL               string      `json:"url"`
	SSHURL            string      `json:"ssh_url"`
	HTTPURL           string      `json:"http_url"`
}

type MergeParams struct {
	ForceRemoveSourceBranch bool `json:"force_remove_source_branch"`
}

type Source struct {
	ID                int         `json:"id"`
	Name              string      `json:"name"`
	Description       string      `json:"description"`
	WebURL            string      `json:"web_url"`
	AvatarURL         interface{} `json:"avatar_url"`
	GitSSHURL         string      `json:"git_ssh_url"`
	GitHTTPURL        string      `json:"git_http_url"`
	Namespace         string      `json:"namespace"`
	VisibilityLevel   int         `json:"visibility_level"`
	PathWithNamespace string      `json:"path_with_namespace"`
	DefaultBranch     string      `json:"default_branch"`
	CiConfigPath      interface{} `json:"ci_config_path"`
	Homepage          string      `json:"homepage"`
	URL               string      `json:"url"`
	SSHURL            string      `json:"ssh_url"`
	HTTPURL           string      `json:"http_url"`
}

type Target struct {
	ID                int         `json:"id"`
	Name              string      `json:"name"`
	Description       string      `json:"description"`
	WebURL            string      `json:"web_url"`
	AvatarURL         interface{} `json:"avatar_url"`
	GitSSHURL         string      `json:"git_ssh_url"`
	GitHTTPURL        string      `json:"git_http_url"`
	Namespace         string      `json:"namespace"`
	VisibilityLevel   int         `json:"visibility_level"`
	PathWithNamespace string      `json:"path_with_namespace"`
	DefaultBranch     string      `json:"default_branch"`
	CiConfigPath      interface{} `json:"ci_config_path"`
	Homepage          string      `json:"homepage"`
	URL               string      `json:"url"`
	SSHURL            string      `json:"ssh_url"`
	HTTPURL           string      `json:"http_url"`
}

type LastCommit struct {
	ID        string    `json:"id"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	URL       string    `json:"url"`
	Author    Author    `json:"author"`
}

type ObjectAttributes struct {
	AssigneeID                interface{} `json:"assignee_id"`
	AuthorID                  int         `json:"author_id"`
	CreatedAt                 string      `json:"created_at"`
	Description               string      `json:"description"`
	HeadPipelineID            interface{} `json:"head_pipeline_id"`
	ID                        int         `json:"id"`
	IID                       int         `json:"iid"`
	LastEditedAt              interface{} `json:"last_edited_at"`
	LastEditedByID            interface{} `json:"last_edited_by_id"`
	MergeCommitSha            interface{} `json:"merge_commit_sha"`
	MergeError                interface{} `json:"merge_error"`
	MergeParams               MergeParams `json:"merge_params"`
	MergeStatus               string      `json:"merge_status"`
	MergeUserID               interface{} `json:"merge_user_id"`
	MergeWhenPipelineSucceeds bool        `json:"merge_when_pipeline_succeeds"`
	MilestoneID               interface{} `json:"milestone_id"`
	SourceBranch              string      `json:"source_branch"`
	SourceProjectID           int         `json:"source_project_id"`
	State                     string      `json:"state"`
	TargetBranch              string      `json:"target_branch"`
	TargetProjectID           int         `json:"target_project_id"`
	TimeEstimate              int         `json:"time_estimate"`
	Title                     string      `json:"title"`
	UpdatedAt                 string      `json:"updated_at"`
	UpdatedByID               interface{} `json:"updated_by_id"`
	URL                       string      `json:"url"`
	Source                    Source      `json:"source"`
	Target                    Target      `json:"target"`
	LastCommit                LastCommit  `json:"last_commit"`
	WorkInProgress            bool        `json:"work_in_progress"`
	TotalTimeSpent            int         `json:"total_time_spent"`
	HumanTotalTimeSpent       interface{} `json:"human_total_time_spent"`
	HumanTimeEstimate         interface{} `json:"human_time_estimate"`
	Action                    string      `json:"action"`
}

type MergeEvent struct {
	ObjectKind       string           `json:"object_kind"`
	EventType        string           `json:"event_type"`
	User             User             `json:"user"`
	Project          WebhookProject   `json:"project"`
	ObjectAttributes ObjectAttributes `json:"object_attributes"`
	Labels           []interface{}    `json:"labels"`
}

type PushEvent struct {
	ObjectKind        string            `json:"object_kind"`
	EventName         string            `json:"event_name"`
	Before            string            `json:"before"`
	After             string            `json:"after"`
	Ref               string            `json:"ref"`
	CheckoutSha       string            `json:"checkout_sha"`
	Message           interface{}       `json:"message"`
	UserID            int               `json:"user_id"`
	UserName          string            `json:"user_name"`
	UserUsername      string            `json:"user_username"`
	UserEmail         string            `json:"user_email"`
	UserAvatar        string            `json:"user_avatar"`
	ProjectID         int               `json:"project_id"`
	Project           WebhookProject    `json:"project"`
	Commits           []PushEventCommit `json:"commits"`
	TotalCommitsCount int               `json:"total_commits_count"`
}

type PushEventCommit struct {
	ID        string    `json:"id"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	URL       string    `json:"url"`
	Author    Author    `json:"author"`
	Added     []string  `json:"added"`
	Modified  []string  `json:"modified"`
	Removed   []string  `json:"removed"`
}

type Author struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}
