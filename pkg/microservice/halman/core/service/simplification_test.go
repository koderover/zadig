package service

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
)

func TestEvidenceFiltersKeepSeparatePolicies(t *testing.T) {
	items := make([]any, 60)
	for i := range items {
		items[i] = map[string]any{"name": "api", "script": "hidden-script", "spec": map[string]any{"image": "api:v1"}, "access_token": "hidden-token"}
	}
	for _, input := range []struct {
		name   string
		filter func(json.RawMessage) (any, error)
		count  int
	}{
		{"目录", safeCatalogJSON, 60}, {"诊断", safeDiagnosticJSON, 50},
	} {
		t.Run(input.name, func(t *testing.T) {
			value, err := input.filter(common.JSON(map[string]any{"items": items}))
			if err != nil {
				t.Fatal(err)
			}
			rows := value.(map[string]any)["items"].([]any)
			text := string(common.JSON(value))
			if len(rows) != input.count || strings.Contains(text, "hidden-") {
				t.Fatalf("过滤规则或数组上限改变: %s", text)
			}
			_, hasSpec := rows[0].(map[string]any)["spec"]
			_, hasTokenKey := rows[0].(map[string]any)["access_token"]
			if hasSpec != (input.name == "目录") || hasTokenKey != (input.name == "目录") {
				t.Fatal("目录保留脱敏字段，诊断删除配置和凭据字段")
			}
			if _, err := input.filter(json.RawMessage(`{broken`)); err == nil {
				t.Fatal("无效 JSON 必须报错")
			}
			if _, err := input.filter(common.JSON(map[string]string{"message": strings.Repeat("x", 65<<10)})); err == nil {
				t.Fatal("输出超过读取上限必须报错")
			}
		})
	}
	// 保留原有处理顺序：诊断先脱敏叶节点，目录在字段筛选后整体脱敏。
	raw := json.RawMessage(`{"name":"https://user:token@example.com","value":"visible"}`)
	catalog, _ := safeCatalogJSON(raw)
	diagnostic, _ := safeDiagnosticJSON(raw)
	if catalog.(map[string]any)["value"] != "[已隐藏]" || diagnostic.(map[string]any)["value"] != "visible" {
		t.Fatal("合并遍历不能改变字段筛选和脱敏的先后顺序")
	}
}

func TestSharedTemplateReadRequiresExpectedFingerprint(t *testing.T) {
	for _, source := range []string{"yaml_template", "helm_template", "build_template"} {
		t.Run(source, func(t *testing.T) {
			raw := common.JSON(map[string]any{"name": "base", "sha1": "digest", "variables": []any{}})
			platform := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Fatal("模板读取不得提交写操作")
				}
				w.Write(raw)
			})
			s := &AgentService{Platform: platform}
			e := reportExecution("service_onboarding")
			matching := fingerprint(raw)
			for _, expected := range []*string{nil, &matching} {
				if _, err := s.readTemplate(context.Background(), e, "p", source, "base", expected); err != nil {
					t.Fatal(err)
				}
			}
			for _, expected := range []string{"", "changed"} {
				if _, err := s.readTemplate(context.Background(), e, "p", source, "base", &expected); err == nil {
					t.Fatal("确认和提交阶段不能把空指纹当作不校验")
				}
			}
		})
	}
}
