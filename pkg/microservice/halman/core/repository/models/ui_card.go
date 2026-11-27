package models

import (
	"encoding/json"
	"time"
)

type UIAction struct {
	ID     string       `json:"id"`
	Label  string       `json:"label"`
	Type   string       `json:"type"`
	Prompt string       `json:"prompt,omitempty"`
	Target *PageContext `json:"target,omitempty"`
}
type UIOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}
type UIField struct {
	ID            string     `json:"id"`
	Type          string     `json:"type,omitempty"`
	Label         string     `json:"label"`
	Required      bool       `json:"required,omitempty"`
	Default       any        `json:"default,omitempty"`
	Min           *float64   `json:"min,omitempty"`
	Max           *float64   `json:"max,omitempty"`
	MinLength     *int       `json:"min_length,omitempty"`
	MaxLength     *int       `json:"max_length,omitempty"`
	Options       []UIOption `json:"options,omitempty"`
	ResourceType  string     `json:"resource_type,omitempty"`
	Multiple      bool       `json:"multiple,omitempty"`
	ParentFieldID string     `json:"parent_field_id,omitempty"`
	Production    bool       `json:"production,omitempty"`
}
type UIQuestion struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	Type        string     `json:"type"`
	Required    bool       `json:"required,omitempty"`
	Options     []UIOption `json:"options,omitempty"`
	AllowOther  bool       `json:"allow_other,omitempty"`
}
type UIData struct {
	Fields    []UIField    `json:"fields,omitempty"`
	Questions []UIQuestion `json:"questions,omitempty"`
}
type UICard struct {
	ResourceSelections map[string]any      `json:"-"`
	DraftFields        bool                `json:"-"`
	SchemaVersion      int                 `json:"schema_version"`
	ID                 string              `json:"id"`
	ConversationID     string              `json:"conversation_id"`
	RunID              string              `json:"run_id"`
	AnswerVersion      int                 `json:"answer_version"`
	Revision           int                 `json:"revision"`
	Sequence           int                 `json:"sequence"`
	SourceKey          string              `json:"-"`
	Type               string              `json:"type"`
	Title              string              `json:"title"`
	Status             string              `json:"status"`
	CreatedAt          time.Time           `json:"created_at"`
	RunCreatedAt       time.Time           `json:"run_created_at"`
	ExpiresAt          *time.Time          `json:"expires_at,omitempty"`
	Data               json.RawMessage     `json:"data" swaggertype:"object"`
	Actions            []UIAction          `json:"actions"`
	Submission         *UISubmissionResult `json:"submission,omitempty"`
}
type UISubmissionRequest struct {
	RequestID string         `json:"request_id"`
	Revision  int            `json:"revision"`
	ActionID  string         `json:"action_id"`
	Values    map[string]any `json:"values"`
}
type UISubmissionResult struct {
	ActionID             string `json:"action_id,omitempty"`
	CurrentAttemptStatus string `json:"current_attempt_status,omitempty"`

	Card                *UICard `json:"card,omitempty"`
	SubmissionID        string  `json:"submission_id"`
	ContinuationRunID   string  `json:"continuation_run_id"`
	CurrentAttemptRunID string  `json:"current_attempt_run_id"`
}
type UICardInput struct {
	Title   string     `json:"title"`
	Data    UIData     `json:"data"`
	Actions []UIAction `json:"actions,omitempty"`
}
type UIResource struct {
	ProjectKey  string `json:"project_key"`
	EnvName     string `json:"env_name,omitempty"`
	ServiceName string `json:"service_name,omitempty"`
	Production  bool   `json:"production"`
}
type UIResourceOption struct {
	Label    string     `json:"label"`
	Value    string     `json:"value"`
	Resource UIResource `json:"resource"`
}

type UIContinuation struct {
	OnboardingPatch *OnboardingInput
	ResourceScopes  []ResourceScope
	// Input is built from the authorized card and validated values; never accepted from JSON.
	Input        json.RawMessage
	CardID       string
	Revision     int
	SubmissionID string
	ActionID     string
	Values       map[string]any
	Hash         string
	SourceRun    *Run
	Retry        bool
}

type UICardReply struct {
	CardID   string `json:"card_id"`
	Revision int    `json:"revision"`
}

func AllQuestionsOptional(questions []UIQuestion) bool {
	for _, question := range questions {
		if question.Required {
			return false
		}
	}
	return true
}
