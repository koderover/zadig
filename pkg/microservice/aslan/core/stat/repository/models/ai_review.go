package models

import "github.com/koderover/zadig/v2/pkg/types/step"

// AIReviewStat is the last valid report of a workflow task for one repository.
type AIReviewStat struct {
	ProjectName        string                  `bson:"project_name"`
	ProjectDisplayName string                  `bson:"project_display_name"`
	WorkflowName       string                  `bson:"workflow_name"`
	TaskID             int64                   `bson:"task_id"`
	CodehostID         int                     `bson:"codehost_id"`
	RepoOwner          string                  `bson:"repo_owner"`
	RepoName           string                  `bson:"repo_name"`
	PR                 int                     `bson:"pr"`
	PRTitle            string                  `bson:"pr_title"`
	PRAuthor           string                  `bson:"pr_author"`
	PRURL              string                  `bson:"pr_url"`
	ReviewedAt         int64                   `bson:"reviewed_at"`
	Model              string                  `bson:"model"`
	Usage              step.AIReviewTokenUsage `bson:"usage"`
	DurationMS         int64                   `bson:"duration_ms"`
	Incomplete         bool                    `bson:"incomplete"`
	ExitCode           int                     `bson:"exit_code"`
	Findings           []AIReviewStatFinding   `bson:"findings"`
}

type AIReviewStatFinding struct {
	Fingerprint  string                     `bson:"fingerprint"`
	Severity     string                     `bson:"severity"`
	Category     string                     `bson:"category"`
	CategoryName string                     `bson:"category_name"`
	RuleID       string                     `bson:"rule_id"`
	RuleName     string                     `bson:"rule_name"`
	MatchedRules []step.AIReviewMatchedRule `bson:"matched_rules"`
}

func (AIReviewStat) TableName() string { return "ai_review_stat" }
