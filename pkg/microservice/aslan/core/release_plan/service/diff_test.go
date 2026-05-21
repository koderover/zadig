package service

import "testing"

func TestGetReleasePlanArrayItemKey(t *testing.T) {
	t.Run("job key", func(t *testing.T) {
		key, ok := getReleasePlanArrayItemKey(map[string]interface{}{
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
		key, ok := getReleasePlanArrayItemKey(map[string]interface{}{
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

	t.Run("name and id key", func(t *testing.T) {
		key, ok := getReleasePlanArrayItemKey(map[string]interface{}{
			"id":   "job-id",
			"name": "build",
		})
		if !ok {
			t.Fatalf("expected key")
		}
		if key != "build|job-id" {
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

func TestBuildReleasePlanDiffLabelForOrderChange(t *testing.T) {
	label := buildReleasePlanDiffLabel("order")
	if label != "顺序" {
		t.Fatalf("unexpected order label: %s", label)
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

func TestToGenericValueSupportsRootArrays(t *testing.T) {
	value := []map[string]interface{}{
		{"id": "job-1", "name": "job-a"},
		{"id": "job-2", "name": "job-b"},
	}

	generic, err := toGenericValue(value)
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

func TestDiffReleasePlanValuesDetectsOrderedArrayChanges(t *testing.T) {
	left := []interface{}{
		map[string]interface{}{"id": "job-1", "name": "build"},
		map[string]interface{}{"id": "job-2", "name": "deploy"},
	}
	right := []interface{}{
		map[string]interface{}{"id": "job-2", "name": "deploy"},
		map[string]interface{}{"id": "job-1", "name": "build"},
	}

	entries := make([]*releasePlanRawDiffEntry, 0)
	diffReleasePlanValues(releasePlanDiffContext{GroupType: releasePlanVersionSectionJobsOrder}, "", left, right, &entries)

	if len(entries) != 1 {
		t.Fatalf("expected exactly one order change, got %d", len(entries))
	}
	entry := entries[0]
	if entry.ChangeType != releasePlanDiffChangeTypeOrder {
		t.Fatalf("unexpected change type: %s", entry.ChangeType)
	}
	if entry.Path != "order" {
		t.Fatalf("unexpected path: %s", entry.Path)
	}
	if len(entry.BeforeOrder) != 2 || len(entry.AfterOrder) != 2 {
		t.Fatalf("unexpected order item counts: before=%d after=%d", len(entry.BeforeOrder), len(entry.AfterOrder))
	}
	if entry.BeforeOrder[0].ID != "job-1" || entry.BeforeOrder[0].Name != "build" {
		t.Fatalf("unexpected first before order item: %#v", entry.BeforeOrder[0])
	}
	if entry.AfterOrder[0].ID != "job-2" || entry.AfterOrder[0].Name != "deploy" {
		t.Fatalf("unexpected first after order item: %#v", entry.AfterOrder[0])
	}
}

func TestDiffReleasePlanValuesDetectsOrderedArrayChangesWithDuplicateNames(t *testing.T) {
	left := []interface{}{
		map[string]interface{}{"id": "job-1", "name": "build"},
		map[string]interface{}{"id": "job-2", "name": "build"},
	}
	right := []interface{}{
		map[string]interface{}{"id": "job-2", "name": "build"},
		map[string]interface{}{"id": "job-1", "name": "build"},
	}

	entries := make([]*releasePlanRawDiffEntry, 0)
	diffReleasePlanValues(releasePlanDiffContext{GroupType: releasePlanVersionSectionJobsOrder}, "", left, right, &entries)

	if len(entries) != 1 {
		t.Fatalf("expected exactly one order change, got %d", len(entries))
	}
	entry := entries[0]
	if entry.ChangeType != releasePlanDiffChangeTypeOrder {
		t.Fatalf("unexpected change type: %s", entry.ChangeType)
	}
	if entry.BeforeOrder[0].ID != "job-1" || entry.AfterOrder[0].ID != "job-2" {
		t.Fatalf("unexpected duplicate-name order diff: before=%#v after=%#v", entry.BeforeOrder[0], entry.AfterOrder[0])
	}
}

func TestDiffReleasePlanValuesKeepsDefaultKeyedArrayBehavior(t *testing.T) {
	left := []interface{}{
		map[string]interface{}{"id": "stage-1", "name": "build"},
		map[string]interface{}{"id": "stage-2", "name": "deploy"},
	}
	right := []interface{}{
		map[string]interface{}{"id": "stage-2", "name": "deploy"},
		map[string]interface{}{"id": "stage-1", "name": "build"},
	}

	entries := make([]*releasePlanRawDiffEntry, 0)
	diffReleasePlanValues(releasePlanDiffContext{GroupType: "job"}, "spec.workflow.stages", left, right, &entries)

	if len(entries) != 0 {
		t.Fatalf("expected keyed unordered arrays to ignore pure reorder by default, got %d entries", len(entries))
	}
}
