package models

import "time"

type CleanupStats struct {
	Rows    int64 `json:"rows"`
	Batches int64 `json:"batches"`
}

type MaintenanceStatus struct {
	LastAttemptAt  *time.Time `json:"last_attempt_at,omitempty"`
	LastSuccessAt  *time.Time `json:"last_success_at,omitempty"`
	LastProgressAt *time.Time `json:"last_progress_at,omitempty"`
	LastDurationMS int64      `json:"last_duration_ms"`
	CleanupStats
	PauseReason     string     `json:"pause_reason,omitempty"`
	OldestExpiredAt *time.Time `json:"oldest_expired_at,omitempty"`
	OldestDeletedAt *time.Time `json:"oldest_deleted_at,omitempty"`
	BacklogOver24h  bool       `json:"backlog_over_24h"`
}
