package service

import "testing"

func TestGetReleasePlanArrayItemKey(t *testing.T) {
	t.Run("job key", func(t *testing.T) {
		key, ok := buildReleasePlanArrayKeyByNameTypeID(map[string]interface{}{
			"name": "build",
			"type": "zadig-build",
			"id":   "job-id",
		})
		if !ok {
			t.Fatalf("expected key")
		}
		if key != "build|zadig-build|job-id" {
			t.Fatalf("unexpected key: %s", key)
		}
	})

	t.Run("service key", func(t *testing.T) {
		key, ok := buildReleasePlanArrayKeyByServiceModule(map[string]interface{}{
			"service_name":   "gateway",
			"service_module": "gateway",
		})
		if !ok {
			t.Fatalf("expected key")
		}
		if key != "gateway/gateway" {
			t.Fatalf("unexpected key: %s", key)
		}
	})
}

func TestBuildReleasePlanDiffLabel(t *testing.T) {
	label := buildReleasePlanDiffLabel("jobs[release-job|workflow|job-id].spec.workflow.stages[build].jobs[deploy|zadig-deploy].spec.namespace")
	expected := "任务 release-job / 阶段 build / 任务 deploy / 命名空间"
	if label != expected {
		t.Fatalf("unexpected label: %s", label)
	}
}

func TestReleasePlanDiffPathRules(t *testing.T) {
	if !shouldIgnoreReleasePlanDiffPath("update_time") {
		t.Fatalf("expected update_time to be ignored")
	}
	if !isLargeTextReleasePlanDiffPath("jobs[deploy].spec.script", "echo 1", "echo 2") {
		t.Fatalf("expected script to be marked as large text")
	}
}

func TestReleasePlanSubtreeHashPrune(t *testing.T) {
	left := map[string]interface{}{
		"a": 1.0,
		"b": "x",
		"c": []interface{}{map[string]interface{}{"id": "1"}, map[string]interface{}{"id": "2"}},
		"d": map[string]interface{}{"name": "demo"},
	}
	right := map[string]interface{}{
		"a": 1.0,
		"b": "x",
		"c": []interface{}{map[string]interface{}{"id": "1"}, map[string]interface{}{"id": "2"}},
		"d": map[string]interface{}{"name": "demo"},
	}

	equal, hashed := equalReleasePlanSubtreeByHash(left, right)
	if !hashed {
		t.Fatalf("expected hash pruning to be enabled for large maps")
	}
	if !equal {
		t.Fatalf("expected identical subtrees to be equal")
	}
}

func TestReleasePlanSubtreeHashPruneSkipSmallNodes(t *testing.T) {
	left := map[string]interface{}{"a": 1.0, "b": 2.0}
	right := map[string]interface{}{"a": 1.0, "b": 2.0}

	equal, hashed := equalReleasePlanSubtreeByHash(left, right)
	if hashed {
		t.Fatalf("expected hash pruning to skip small maps")
	}
	if equal {
		t.Fatalf("hash shortcut should not report equality for skipped small maps")
	}
}

func TestToReleasePlanGenericValueSupportsRootArrays(t *testing.T) {
	value := []map[string]interface{}{
		{"id": "job-1", "name": "job-a"},
		{"id": "job-2", "name": "job-b"},
	}

	generic, err := toReleasePlanGenericValue(value)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	items, ok := generic.([]interface{})
	if !ok {
		t.Fatalf("expected array root, got %T", generic)
	}
	if len(items) != 2 {
		t.Fatalf("unexpected item count: %d", len(items))
	}
}

func TestSanitizeReleasePlanValueForDisplay(t *testing.T) {
	value := map[string]interface{}{
		"vars": []interface{}{
			map[string]interface{}{
				"key":           "DB_PASSWORD",
				"value":         "secret-token",
				"is_credential": true,
			},
		},
	}

	sanitized := sanitizeReleasePlanValueForDisplay(value).(map[string]interface{})
	vars := sanitized["vars"].([]interface{})
	item := vars[0].(map[string]interface{})
	if item["value"] != releasePlanMaskedValueDisplay {
		t.Fatalf("expected credential value to be hidden")
	}
	if item["key"] != "DB_PASSWORD" {
		t.Fatalf("expected non-sensitive fields to stay visible")
	}
}
