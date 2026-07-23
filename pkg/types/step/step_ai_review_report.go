/*
Copyright 2026 The KodeRover Authors.

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

package step

type StepAIReviewReportSpec struct {
	ReportPath      string          `bson:"report_path" json:"report_path" yaml:"report_path"`
	CodehostID      int             `bson:"codehost_id" json:"codehost_id" yaml:"codehost_id"`
	RepoOwner       string          `bson:"repo_owner" json:"repo_owner" yaml:"repo_owner"`
	RepoName        string          `bson:"repo_name" json:"repo_name" yaml:"repo_name"`
	PR              int             `bson:"pr" json:"pr" yaml:"pr"`
	Report          *AIReviewReport `bson:"report,omitempty" json:"report,omitempty" yaml:"report,omitempty"`
	CollectionError string          `bson:"collection_error,omitempty" json:"collection_error,omitempty" yaml:"collection_error,omitempty"`
}

// AIReviewReport mirrors the JSON report emitted by zadig-review-agent v0.1.1.
type AIReviewReport struct {
	Metadata      AIReviewMetadata       `bson:"metadata" json:"metadata" yaml:"metadata"`
	Stats         AIReviewStats          `bson:"stats" json:"stats" yaml:"stats"`
	Usage         AIReviewTokenUsage     `bson:"usage" json:"usage" yaml:"usage"`
	DurationMS    int64                  `bson:"duration_ms" json:"duration_ms" yaml:"duration_ms"`
	Process       AIReviewProcess        `bson:"process" json:"process" yaml:"process"`
	ResolvedRules []AIReviewResolvedRule `bson:"resolved_rules,omitempty" json:"resolved_rules,omitempty" yaml:"resolved_rules,omitempty"`
	ExcludedFiles []AIReviewExcludedFile `bson:"excluded_files,omitempty" json:"excluded_files,omitempty" yaml:"excluded_files,omitempty"`
	Warnings      []string               `bson:"warnings,omitempty" json:"warnings,omitempty" yaml:"warnings,omitempty"`
	Findings      []AIReviewFinding      `bson:"findings" json:"findings" yaml:"findings"`
	Incomplete    bool                   `bson:"incomplete" json:"incomplete" yaml:"incomplete"`
	Errors        []string               `bson:"errors,omitempty" json:"errors,omitempty" yaml:"errors,omitempty"`
	ExitCode      int                    `bson:"exit_code" json:"exit_code" yaml:"exit_code"`
}

type AIReviewFinding struct {
	Severity     string  `bson:"severity" json:"severity" yaml:"severity"`
	Category     string  `bson:"category" json:"category" yaml:"category"`
	RuleID       string  `bson:"rule_id,omitempty" json:"rule_id,omitempty" yaml:"rule_id,omitempty"`
	File         string  `bson:"file" json:"file" yaml:"file"`
	StartLine    int     `bson:"start_line" json:"start_line" yaml:"start_line"`
	EndLine      int     `bson:"end_line" json:"end_line" yaml:"end_line"`
	Title        string  `bson:"title" json:"title" yaml:"title"`
	Problem      string  `bson:"problem" json:"problem" yaml:"problem"`
	Evidence     string  `bson:"evidence" json:"evidence" yaml:"evidence"`
	Suggestion   string  `bson:"suggestion" json:"suggestion" yaml:"suggestion"`
	ExistingCode string  `bson:"existing_code,omitempty" json:"existing_code,omitempty" yaml:"existing_code,omitempty"`
	Confidence   float64 `bson:"confidence" json:"confidence" yaml:"confidence"`
	Fingerprint  string  `bson:"fingerprint" json:"fingerprint" yaml:"fingerprint"`
}

type AIReviewProcess struct {
	ToolCalls      []AIReviewToolCall      `bson:"tool_calls" json:"tool_calls" yaml:"tool_calls"`
	Compressions   []AIReviewCompression   `bson:"compressions" json:"compressions" yaml:"compressions"`
	ModelResponses []AIReviewModelResponse `bson:"model_responses" json:"model_responses" yaml:"model_responses"`
}

type AIReviewModelResponse struct {
	ID              string             `bson:"id" json:"id" yaml:"id"`
	Stage           string             `bson:"stage" json:"stage" yaml:"stage"`
	File            string             `bson:"file,omitempty" json:"file,omitempty" yaml:"file,omitempty"`
	Attempt         int                `bson:"attempt" json:"attempt" yaml:"attempt"`
	Status          string             `bson:"status" json:"status" yaml:"status"`
	StartedOffsetMS int64              `bson:"started_offset_ms" json:"started_offset_ms" yaml:"started_offset_ms"`
	DurationMS      int64              `bson:"duration_ms" json:"duration_ms" yaml:"duration_ms"`
	FinishReason    string             `bson:"finish_reason,omitempty" json:"finish_reason,omitempty" yaml:"finish_reason,omitempty"`
	Text            string             `bson:"text,omitempty" json:"text,omitempty" yaml:"text,omitempty"`
	Usage           AIReviewTokenUsage `bson:"usage" json:"usage" yaml:"usage"`
	Error           string             `bson:"error,omitempty" json:"error,omitempty" yaml:"error,omitempty"`
}

type AIReviewCompression struct {
	ID                 string             `bson:"id" json:"id" yaml:"id"`
	File               string             `bson:"file" json:"file" yaml:"file"`
	Round              int                `bson:"round" json:"round" yaml:"round"`
	Status             string             `bson:"status" json:"status" yaml:"status"`
	StartedOffsetMS    int64              `bson:"started_offset_ms" json:"started_offset_ms" yaml:"started_offset_ms"`
	DurationMS         int64              `bson:"duration_ms" json:"duration_ms" yaml:"duration_ms"`
	BeforeTokens       int                `bson:"before_tokens" json:"before_tokens" yaml:"before_tokens"`
	AfterTokens        int                `bson:"after_tokens" json:"after_tokens" yaml:"after_tokens"`
	CompressedMessages int                `bson:"compressed_messages" json:"compressed_messages" yaml:"compressed_messages"`
	PreservedMessages  int                `bson:"preserved_messages" json:"preserved_messages" yaml:"preserved_messages"`
	Usage              AIReviewTokenUsage `bson:"usage" json:"usage" yaml:"usage"`
	Error              string             `bson:"error,omitempty" json:"error,omitempty" yaml:"error,omitempty"`
}

type AIReviewToolCall struct {
	ID              string                `bson:"id" json:"id" yaml:"id"`
	File            string                `bson:"file" json:"file" yaml:"file"`
	Round           int                   `bson:"round" json:"round" yaml:"round"`
	Tool            string                `bson:"tool" json:"tool" yaml:"tool"`
	Arguments       AIReviewToolArguments `bson:"arguments" json:"arguments" yaml:"arguments"`
	Status          string                `bson:"status" json:"status" yaml:"status"`
	Cached          bool                  `bson:"cached,omitempty" json:"cached,omitempty" yaml:"cached,omitempty"`
	StartedOffsetMS int64                 `bson:"started_offset_ms" json:"started_offset_ms" yaml:"started_offset_ms"`
	DurationMS      int64                 `bson:"duration_ms" json:"duration_ms" yaml:"duration_ms"`
	OutputBytes     int                   `bson:"output_bytes" json:"output_bytes" yaml:"output_bytes"`
	OutputTruncated bool                  `bson:"output_truncated" json:"output_truncated" yaml:"output_truncated"`
	Summary         string                `bson:"summary" json:"summary" yaml:"summary"`
	Output          string                `bson:"output" json:"output" yaml:"output"`
}

type AIReviewToolArguments struct {
	FilePath      string           `bson:"file_path,omitempty" json:"file_path,omitempty" yaml:"file_path,omitempty"`
	QueryName     string           `bson:"query_name,omitempty" json:"query_name,omitempty" yaml:"query_name,omitempty"`
	SearchText    string           `bson:"search_text,omitempty" json:"search_text,omitempty" yaml:"search_text,omitempty"`
	FilePatterns  []string         `bson:"file_patterns,omitempty" json:"file_patterns,omitempty" yaml:"file_patterns,omitempty"`
	CaseSensitive bool             `bson:"case_sensitive,omitempty" json:"case_sensitive,omitempty" yaml:"case_sensitive,omitempty"`
	UsePerlRegexp bool             `bson:"use_perl_regexp,omitempty" json:"use_perl_regexp,omitempty" yaml:"use_perl_regexp,omitempty"`
	StartLine     int              `bson:"start_line,omitempty" json:"start_line,omitempty" yaml:"start_line,omitempty"`
	EndLine       int              `bson:"end_line,omitempty" json:"end_line,omitempty" yaml:"end_line,omitempty"`
	Finding       *AIReviewFinding `bson:"finding,omitempty" json:"finding,omitempty" yaml:"finding,omitempty"`
}

type AIReviewTokenUsage struct {
	PromptTokens     int64 `bson:"prompt_tokens" json:"prompt_tokens" yaml:"prompt_tokens"`
	CompletionTokens int64 `bson:"completion_tokens" json:"completion_tokens" yaml:"completion_tokens"`
	TotalTokens      int64 `bson:"total_tokens" json:"total_tokens" yaml:"total_tokens"`
	LLMRequests      int64 `bson:"llm_requests" json:"llm_requests" yaml:"llm_requests"`
	CacheReadTokens  int64 `bson:"cache_read_tokens" json:"cache_read_tokens" yaml:"cache_read_tokens"`
	CacheWriteTokens int64 `bson:"cache_write_tokens" json:"cache_write_tokens" yaml:"cache_write_tokens"`
}

type AIReviewMetadata struct {
	DiffMode   string `bson:"diff_mode" json:"diff_mode" yaml:"diff_mode"`
	From       string `bson:"from,omitempty" json:"from,omitempty" yaml:"from,omitempty"`
	To         string `bson:"to,omitempty" json:"to,omitempty" yaml:"to,omitempty"`
	Commit     string `bson:"commit,omitempty" json:"commit,omitempty" yaml:"commit,omitempty"`
	Head       string `bson:"head,omitempty" json:"head,omitempty" yaml:"head,omitempty"`
	Protocol   string `bson:"protocol" json:"protocol" yaml:"protocol"`
	Model      string `bson:"model" json:"model" yaml:"model"`
	Zadig      bool   `bson:"zadig" json:"zadig" yaml:"zadig"`
	Repository string `bson:"repository,omitempty" json:"repository,omitempty" yaml:"repository,omitempty"`
	Language   string `bson:"language,omitempty" json:"language,omitempty" yaml:"language,omitempty"`
	ReportDir  string `bson:"report_dir,omitempty" json:"report_dir,omitempty" yaml:"report_dir,omitempty"`
	JSONReport string `bson:"json_report,omitempty" json:"json_report,omitempty" yaml:"json_report,omitempty"`
	MDReport   string `bson:"markdown_report,omitempty" json:"markdown_report,omitempty" yaml:"markdown_report,omitempty"`
}

type AIReviewStats struct {
	ChangedFiles int            `bson:"changed_files" json:"changed_files" yaml:"changed_files"`
	Chunks       int            `bson:"chunks" json:"chunks" yaml:"chunks"`
	BySeverity   map[string]int `bson:"by_severity" json:"by_severity" yaml:"by_severity"`
}

type AIReviewResolvedRule struct {
	File       string `bson:"file" json:"file" yaml:"file"`
	Source     string `bson:"source" json:"source" yaml:"source"`
	SourcePath string `bson:"source_path,omitempty" json:"source_path,omitempty" yaml:"source_path,omitempty"`
	Pattern    string `bson:"pattern" json:"pattern" yaml:"pattern"`
	Digest     string `bson:"digest" json:"digest" yaml:"digest"`
}

type AIReviewExcludedFile struct {
	Path           string `bson:"path" json:"path" yaml:"path"`
	Reason         string `bson:"reason" json:"reason" yaml:"reason"`
	MatchedPattern string `bson:"matched_pattern,omitempty" json:"matched_pattern,omitempty" yaml:"matched_pattern,omitempty"`
}
