package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type TerminalAuditAIStatus string

const (
	TerminalAuditAIStatusRunning   TerminalAuditAIStatus = "running"
	TerminalAuditAIStatusSucceeded TerminalAuditAIStatus = "succeeded"
	TerminalAuditAIStatusFailed    TerminalAuditAIStatus = "failed"
)

type TerminalAuditAIFinding struct {
	Seq        int64  `bson:"seq"        json:"seq"`
	Command    string `bson:"command"    json:"command"`
	Risk       string `bson:"risk"       json:"risk"`
	Reason     string `bson:"reason"     json:"reason"`
	Suggestion string `bson:"suggestion" json:"suggestion"`
}

type TerminalAuditAIResult struct {
	ID                   primitive.ObjectID       `bson:"_id,omitempty"    json:"id,omitempty"`
	SessionID            string                   `bson:"session_id"       json:"session_id"`
	Status               TerminalAuditAIStatus    `bson:"status"           json:"status"`
	RiskLevel            string                   `bson:"risk_level"       json:"risk_level"`
	Summary              string                   `bson:"summary"          json:"summary"`
	Findings             []TerminalAuditAIFinding `bson:"findings"         json:"findings"`
	Coverage             string                   `bson:"coverage"         json:"coverage"`
	Model                string                   `bson:"model"            json:"model"`
	TokenNum             int                      `bson:"token_num"        json:"token_num"`
	AnalyzedCommandCount int64                    `bson:"analyzed_command_count" json:"analyzed_command_count"`
	TotalCommandCount    int64                    `bson:"total_command_count" json:"total_command_count"`
	ErrorMessage         string                   `bson:"error_message"    json:"error_message,omitempty"`
	RunID                string                   `bson:"run_id"           json:"-"`
	LeaseExpiresAt       int64                    `bson:"lease_expires_at" json:"-"`
	StartedAt            int64                    `bson:"started_at"        json:"started_at"`
	FinishedAt           int64                    `bson:"finished_at"       json:"finished_at"`
	CreatedAt            int64                    `bson:"created_at"       json:"created_at"`
	UpdatedAt            int64                    `bson:"updated_at"       json:"updated_at"`
}

func (TerminalAuditAIResult) TableName() string {
	return "terminal_audit_ai_result"
}
