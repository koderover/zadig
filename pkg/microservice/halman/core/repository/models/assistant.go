package models

import (
	"encoding/json"
	"time"
)

type PageContext struct {
	PageType     string `json:"page_type,omitempty"`
	ProjectKey   string `json:"project_key,omitempty"`
	WorkflowName string `json:"workflow_name,omitempty"`
	TaskID       int64  `json:"task_id,omitempty"`
	JobName      string `json:"job_name,omitempty"`
	EnvName      string `json:"env_name,omitempty"`
	ServiceName  string `json:"service_name,omitempty"`
	Production   bool   `json:"production,omitempty"`
}

type RunRequest struct {
	// 请求标识，1～64 位字母、数字、横线或下划线，用于幂等去重。
	RequestID string `json:"request_id" validate:"required" example:"chat-20261103-001"`
	// 用户问题，最多 16000 字节。
	Message          string      `json:"message" validate:"required" example:"帮我检查这次工作流为什么失败"`
	Context          PageContext `json:"context"`
	SkillExecutionID string      `json:"skill_execution_id,omitempty"`
	RegenerateFrom   *Run        `json:"-"`
	RetryFrom        string      `json:"-"`
}

type Conversation struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	CreatedAt    time.Time `json:"created_at"`
	LastActiveAt time.Time `json:"last_active_at"`
}

type Run struct {
	ID                  string          `json:"id"`
	RequestID           string          `json:"request_id"`
	ConversationID      string          `json:"conversation_id"`
	Status              string          `json:"status"`
	Context             PageContext     `json:"context"`
	CreatedAt           time.Time       `json:"created_at"`
	FinishedAt          *time.Time      `json:"finished_at,omitempty"`
	Error               string          `json:"error,omitempty"`
	InputTokens         int             `json:"input_tokens"`
	OutputTokens        int             `json:"output_tokens"`
	SkillID             string          `json:"skill_id,omitempty"`
	SkillExecutionID    *string         `json:"skill_execution_id,omitempty"`
	SkillStatus         *string         `json:"skill_status,omitempty"`
	Plan                json.RawMessage `json:"plan,omitempty" swaggertype:"object"`
	PlanVersion         int             `json:"plan_version"`
	PlanStatus          *string         `json:"plan_status,omitempty"`
	PlanExpiresAt       *time.Time      `json:"plan_expires_at,omitempty"`
	AnswerVersion       int             `json:"answer_version"`
	QuestionSourceRunID *string         `json:"question_source_run_id,omitempty"`
	AnswerAvailable     bool            `json:"answer_available"`
}

// AGUIForwardedProps 描述协议透传字段中的 Zadig 页面候选及补问关联。
type AGUIForwardedProps struct {
	PageContext      PageContext `json:"page_context"`
	SkillExecutionID string      `json:"skill_execution_id,omitempty"`
}
