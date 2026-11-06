package types

import "fmt"

type AIAssistantConfig struct {
	Enabled              bool   `json:"enabled" bson:"enabled"`
	LLMIntegrationID     string `json:"llm_integration_id" bson:"llm_integration_id"`
	HistoryRetentionDays int    `json:"history_retention_days" bson:"history_retention_days"`
}

func (c AIAssistantConfig) Validate() error {
	if c.HistoryRetentionDays < 30 || c.HistoryRetentionDays > 365 {
		return fmt.Errorf("聊天保留天数必须在 30 到 365 之间")
	}

	return nil
}

// AIAssistantRuntime 由配置数据库加载，仅在进程内使用，不得持久化或返回给聊天用户。
type AIAssistantRuntime struct {
	AIAssistantConfig
	Protocol   string `json:"protocol"`
	Provider   string `json:"provider"`
	Model      string `json:"model"`
	BaseURL    string `json:"base_url"`
	Token      string `json:"token"`
	Proxy      string `json:"proxy"`
	ModelError string `json:"model_error,omitempty"`
}
