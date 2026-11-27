package types

import (
	"fmt"
	"slices"
)

type AIAssistantConfig struct {
	Enabled              bool   `json:"enabled" bson:"enabled"`
	LLMIntegrationID     string `json:"llm_integration_id" bson:"llm_integration_id"`
	HistoryRetentionDays int    `json:"history_retention_days" bson:"history_retention_days"`
	MaxLLMCalls          *int   `json:"max_llm_calls" bson:"max_llm_calls"`
	MaxToolIterations    *int   `json:"max_tool_iterations" bson:"max_tool_iterations"`
	RunTimeoutSeconds    *int   `json:"run_timeout_seconds" bson:"run_timeout_seconds"`
	HistoryContextTokens *int   `json:"history_context_tokens" bson:"history_context_tokens"`

	LLMIntegrationIDs       []string `json:"llm_integration_ids,omitempty" bson:"llm_integration_ids,omitempty"`
	AllowUserModelSelection bool     `json:"allow_user_model_selection" bson:"allow_user_model_selection"`
}

func (c AIAssistantConfig) WithRunDefaults() AIAssistantConfig {
	withDefault := func(field *int, value int) *int {
		if field == nil {
			return &value
		}
		return field
	}
	c.MaxLLMCalls = withDefault(c.MaxLLMCalls, 8)
	c.MaxToolIterations = withDefault(c.MaxToolIterations, 6)
	c.RunTimeoutSeconds = withDefault(c.RunTimeoutSeconds, 300)
	if c.LLMIntegrationID == "" && len(c.LLMIntegrationIDs) > 0 {
		c.LLMIntegrationID = c.LLMIntegrationIDs[0]
	}
	return c
}

func (c AIAssistantConfig) AllowsModel(id string) bool {
	return id != "" && (id == c.LLMIntegrationID || slices.Contains(c.LLMIntegrationIDs, id))
}

func (c AIAssistantConfig) ModelIDs() []string {
	if c.LLMIntegrationID != "" && !slices.Contains(c.LLMIntegrationIDs, c.LLMIntegrationID) {
		return append([]string{c.LLMIntegrationID}, c.LLMIntegrationIDs...)
	}
	return c.LLMIntegrationIDs
}

func (c AIAssistantConfig) Validate() error {
	if c.HistoryRetentionDays < 30 || c.HistoryRetentionDays > 365 {
		return fmt.Errorf("聊天保留天数必须在 30 到 365 之间")
	}

	c = c.WithRunDefaults()
	if *c.MaxLLMCalls < 1 || *c.MaxLLMCalls > 100 {
		return fmt.Errorf("max_llm_calls 必须在 1 到 100 之间")
	}
	if *c.MaxToolIterations < 1 || *c.MaxToolIterations > 100 {
		return fmt.Errorf("max_tool_iterations 必须在 1 到 100 之间")
	}
	if seconds := *c.RunTimeoutSeconds; seconds < 60 || seconds > 3600 || seconds%60 != 0 {
		return fmt.Errorf("run_timeout_seconds 必须在 60 到 3600 秒之间，且为 60 的倍数")
	}
	if tokens := c.HistoryContextTokens; tokens != nil && (*tokens < 0 || *tokens > 1048576) {
		return fmt.Errorf("history_context_tokens 必须在 0 到 1048576 之间")
	}
	seen := make(map[string]bool, len(c.LLMIntegrationIDs))
	for _, id := range c.LLMIntegrationIDs {
		if id == "" || seen[id] {
			return fmt.Errorf("llm_integration_ids 不能包含空值或重复模型")
		}
		seen[id] = true
	}

	return nil
}

type AIAssistantModelOption struct {
	ID                     string   `json:"llm_integration_id"`
	Name                   string   `json:"name"`
	DefaultReasoningEffort string   `json:"default_reasoning_effort"`
	ReasoningEfforts       []string `json:"reasoning_efforts"`
}

type AIAssistantModelOptions struct {
	AllowUserModelSelection bool                     `json:"allow_user_model_selection"`
	DefaultModelID          string                   `json:"default_model_id"`
	Models                  []AIAssistantModelOption `json:"models"`
}

// AIAssistantRuntime 由配置数据库加载，仅在进程内使用，不得持久化或返回给聊天用户。
type AIAssistantRuntime struct {
	AIAssistantConfig
	UsingModelPreference bool   `json:"-"`
	Language             string `json:"-"`
	Protocol             string `json:"protocol"`
	Provider             string `json:"provider"`
	Model                string `json:"model"`
	BaseURL              string `json:"base_url"`
	Token                string `json:"token"`
	Proxy                string `json:"proxy"`
	ModelError           string `json:"model_error,omitempty"`

	ContextWindow   int    `json:"context_window,omitempty"`
	MaxOutputTokens int    `json:"max_output_tokens,omitempty"`
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
}
