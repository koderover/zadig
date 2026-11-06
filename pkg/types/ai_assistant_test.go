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
	for _, enabled := range []bool{false, true} {
		original := AIAssistantConfig{Enabled: enabled, HistoryRetentionDays: 90}
		raw, err := bson.Marshal(original)
		if err != nil {
			t.Fatal(err)
		}
		var restored AIAssistantConfig
		if err := bson.Unmarshal(raw, &restored); err != nil || restored.Enabled != enabled {
			t.Fatalf("MongoDB 配置必须保留 AI 开关: %+v %v", restored, err)
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
