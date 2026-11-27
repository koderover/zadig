package types

import (
	"encoding/json"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
)

func TestAssistantEnabledDefaultsOffAndPersists(t *testing.T) {
	var defaults AIAssistantConfig
	if err := json.Unmarshal([]byte(`{"history_retention_days":90}`), &defaults); err != nil || defaults.Enabled {
		t.Fatal("没有 enabled 配置时必须默认关闭")
	}
	zero := 0
	for _, enabled := range []bool{false, true} {
		original := AIAssistantConfig{Enabled: enabled, HistoryRetentionDays: 90, HistoryContextTokens: &zero}
		raw, err := bson.Marshal(original)
		if err != nil {
			t.Fatal(err)
		}
		var restored AIAssistantConfig
		if err := bson.Unmarshal(raw, &restored); err != nil || restored.Enabled != enabled ||
			restored.HistoryContextTokens == nil || *restored.HistoryContextTokens != 0 {
			t.Fatalf("MongoDB 配置必须保留 AI 开关和显式的零历史预算: %+v %v", restored, err)
		}
	}
}

func TestAIAssistantRetentionBounds(t *testing.T) {
	for _, days := range []int{0, 29, 366} {
		if (AIAssistantConfig{HistoryRetentionDays: days}).Validate() == nil {
			t.Fatalf("应拒绝保留天数 %d", days)
		}
	}

	for _, days := range []int{30, 90, 365} {
		if err := (AIAssistantConfig{HistoryRetentionDays: days}).Validate(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAssistantRunConfiguration(t *testing.T) {
	defaults := (AIAssistantConfig{}).WithRunDefaults()
	if *defaults.MaxLLMCalls != 8 || *defaults.MaxToolIterations != 6 ||
		*defaults.RunTimeoutSeconds != 300 || defaults.HistoryContextTokens != nil {
		t.Fatalf("unexpected run defaults: %+v", defaults)
	}
	for _, tc := range []struct {
		settings string
		valid    bool
	}{
		{`{"history_retention_days":90,"max_llm_calls":1,"max_tool_iterations":100,"run_timeout_seconds":3600,"history_context_tokens":0}`, true},
		{`{"history_retention_days":90,"max_llm_calls":0}`, false},
		{`{"history_retention_days":90,"max_tool_iterations":101}`, false},
		{`{"history_retention_days":90,"run_timeout_seconds":61}`, false},
		{`{"history_retention_days":90,"history_context_tokens":1048577}`, false},
	} {
		var config AIAssistantConfig
		if err := json.Unmarshal([]byte(tc.settings), &config); err != nil {
			t.Fatal(err)
		}
		if err := config.Validate(); (err == nil) != tc.valid {
			t.Fatalf("Validate(%s) = %v", tc.settings, err)
		}
	}
}
