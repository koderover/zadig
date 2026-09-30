package models

import "time"

// AIReviewFeedbackComment is an embedded published-comment target and its API snapshot.
type AIReviewFeedbackComment struct {
	Kind        string    `bson:"kind"` // issue comment, GitHub review batch, or GitLab note
	CommentID   int64     `bson:"comment_id"`
	ReviewID    int64     `bson:"review_id,omitempty"`
	InlineTotal int       `bson:"inline_total,omitempty"`
	Up          int       `bson:"up"`
	Down        int       `bson:"down"`
	SyncedAt    time.Time `bson:"synced_at,omitempty"`
	DirtyAt     time.Time `bson:"dirty_at,omitempty"`
}

// AIReviewInlineThread tracks only the original AI comment, not replies.
type AIReviewInlineThread struct {
	Fingerprint   string    `bson:"fingerprint,omitempty"`
	CommentID     int64     `bson:"comment_id"`
	CommentNodeID string    `bson:"comment_node_id,omitempty"`
	ThreadID      string    `bson:"thread_id,omitempty"`
	ReviewID      int64     `bson:"review_id,omitempty"`
	Resolved      bool      `bson:"resolved"`
	UpdatedAt     time.Time `bson:"updated_at"`
	Deleted       bool      `bson:"deleted"`
}

// AIReviewFeedback is the materialized PR/MR-level result for the future insights view.
type AIReviewFeedback struct {
	PRTitle            string                    `bson:"pr_title"`
	InlineThreads      []AIReviewInlineThread    `bson:"inline_threads"`
	InlineTotal        int                       `bson:"inline_total"`
	InlineResolved     int                       `bson:"inline_resolved"`
	ResolutionSyncedAt time.Time                 `bson:"resolution_synced_at,omitempty"`
	ProjectName        string                    `bson:"project_name"`
	CodehostID         int                       `bson:"codehost_id"`
	RepoOwner          string                    `bson:"repo_owner"`
	RepoName           string                    `bson:"repo_name"`
	PR                 int                       `bson:"pr"`
	Provider           string                    `bson:"provider"`
	ProjectID          int                       `bson:"project_id"`
	SourceHost         string                    `bson:"source_host"`
	Up                 int                       `bson:"up"`
	Down               int                       `bson:"down"`
	NextSyncAt         time.Time                 `bson:"next_sync_at"`
	FullSyncAt         time.Time                 `bson:"full_sync_at"`
	CycleStartedAt     time.Time                 `bson:"cycle_started_at,omitempty"`
	SyncedAt           time.Time                 `bson:"synced_at,omitempty"`
	LeaseUntil         time.Time                 `bson:"lease_until,omitempty"`
	LeaseToken         string                    `bson:"lease_token,omitempty"`
	Revision           int64                     `bson:"revision"`
	LastError          string                    `bson:"last_error,omitempty"`
	Closed             bool                      `bson:"closed"`
	FinalSync          bool                      `bson:"final_sync,omitempty"`
	UpdatedAt          time.Time                 `bson:"updated_at"`
	Comments           []AIReviewFeedbackComment `bson:"comments"`
	// ponytail: embedded targets and per-MR reaction states; move to a separate collection if a single MR approaches MongoDB's document size limit.
	GitLabReactions map[string]string `bson:"gitlab_reactions,omitempty"`
}

func (AIReviewFeedback) TableName() string { return "ai_review_feedback" }
