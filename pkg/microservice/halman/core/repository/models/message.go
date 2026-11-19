package models

import (
	"encoding/json"
	"time"

	"trpc.group/trpc-go/trpc-agent-go/model"
)

type HistoryEvent struct {
	ID            int64           `json:"id"`
	RunID         string          `json:"run_id"`
	QuestionRunID string          `json:"question_run_id"`
	AnswerVersion int             `json:"answer_version"`
	Latest        bool            `json:"latest"`
	CreatedAt     time.Time       `json:"created_at"`
	Messages      []model.Message `json:"messages"`
}

// HistoryRun is the authoritative plan state for a run visible on a history page.
type HistoryRun struct {
	ID            string          `json:"id"`
	Status        string          `json:"status"`
	Plan          json.RawMessage `json:"plan"`
	PlanVersion   int             `json:"plan_version"`
	PlanStatus    *string         `json:"plan_status,omitempty"`
	PlanExpiresAt *time.Time      `json:"plan_expires_at,omitempty"`
	Error         string          `json:"error,omitempty"`
}

type MessageVersion struct {
	OriginalID        string
	LatestID          string
	Version           int
	Regenerated       bool
	QuestionAvailable bool
}
