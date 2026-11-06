package models

import (
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

type MessageVersion struct {
	OriginalID        string
	LatestID          string
	Version           int
	Regenerated       bool
	QuestionAvailable bool
}
