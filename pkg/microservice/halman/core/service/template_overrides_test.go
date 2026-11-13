package service

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

func TestTemplateOverridesPreserveTypesCredentialsAndInheritance(t *testing.T) {
	raw := []byte(`{"pre_build":{"envs":[{"key":"k1","type":"string","is_credential":true,"value":"old-secret"},{"key":"PASSWORD","type":"string","required":true,"is_credential":true},{"key":"enabled","type":"bool","value":true},{"key":"tier","type":"choice","choice_option":["small","large"],"value":"large"},{"key":"inherited","type":"string","value":"default"}]}}`)
	input := map[string]string{"k1": "new-secret", "PASSWORD": "new-password", "enabled": "false", "tier": "small"}
	if err := validateVariables(input); err != nil {
		t.Fatal(err)
	}
	list, values, err := templateVariables(raw, input)
	if err != nil || len(list) != 4 || values["enabled"] != false || values["tier"] != "small" || values["PASSWORD"] != "new-password" {
		t.Fatalf("覆盖应保留类型与用户新值: %v %v %v", list, values, err)
	}
	if list[0]["is_credential"] != true || list[1]["is_credential"] != true || list[2]["is_credential"] != false || !reflect.DeepEqual(list[3]["choice_option"], []string{"small", "large"}) {
		t.Fatalf("凭据和选项元数据丢失: %v", list)
	}
	if _, exists := values["inherited"]; exists {
		t.Fatal("未修改变量不能重复提交模板默认值")
	}
	for _, placeholder := range []string{"[已隐藏]", "********"} {
		if err := validateVariables(map[string]string{"k1": placeholder}); err == nil {
			t.Fatal("脱敏占位符不能覆盖真实值")
		}
		if _, _, err := templateVariables(raw, map[string]string{"k1": placeholder}); err == nil {
			t.Fatal("实际提交时也须拒绝脱敏占位符")
		}
	}
	if err := validateDraftSecrets(models.OnboardingInput{Environment: &models.EnvironmentInput{Variables: map[string]string{"PASSWORD": "value"}}}); err == nil {
		t.Fatal("模板覆盖不能放开环境非模板凭据配置")
	}
	_, _, err = templateVariables([]byte(`{"variables":[{"key":"k1","is_credential":true,"required":true}]}`), nil)
	if _, ok := err.(*missingTemplateVariables); !ok {
		t.Fatalf("凭据缺项也应允许补充: %v", err)
	}
}

func TestCatalogRedactionPreservesCredentialFlagsAndHidesValues(t *testing.T) {
	value, err := safeCatalogJSON([]byte(`{"variables":[{"key":"k1","is_credential":true,"value":"hidden-secret","default":"hidden-default"},{"key":"normal","is_credential":false,"value":"visible"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	items := value.(map[string]any)["variables"].([]any)
	if items[0].(map[string]any)["is_credential"] != true || items[1].(map[string]any)["is_credential"] != false || items[1].(map[string]any)["value"] != "visible" || strings.Contains(string(common.JSON(value)), "hidden-") {
		t.Fatalf("元数据应保留为布尔值，敏感值须隐藏: %v", value)
	}
}

func TestCredentialOverrideDraftRestoresRawValueButPublicViewsMaskIt(t *testing.T) {
	store, mock := mockStore(t)
	e := reportExecution("project_onboarding")
	e.RootID = e.Run.ID
	platform := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/template/build/general" {
			t.Fatalf("意外查询: %s", r.URL)
		}
		w.Write([]byte(`{"pre_build":{"envs":[{"key":"k1","is_credential":true,"type":"string","value":"template-secret"},{"key":"PORT","type":"string","value":"80"}]}}`))
	})
	s := &AgentService{Store: store, Platform: platform}
	var captured capturedDraft
	expectSkillInputSave(mock, e, "awaiting_input", &captured)
	value, err := nativeSkillTool(t, s, "project_onboarding").Call(context.WithValue(context.Background(), executionKey{}, e), []byte(`{"builds":[{"name":"build","template_id":"general","credential_variables":[],"variables":{"k1":"user-secret","PORT":"8080"}}]}`))
	if err != nil {
		t.Fatal(err)
	}
	build := captured.plan.Draft.Builds[0]
	if build.Variables["k1"] != "user-secret" || !reflect.DeepEqual(build.CredentialVariables, []string{"k1"}) {
		t.Fatal("持久化草稿必须保留原覆盖值和后端凭据标记，不能信任输入的标记")
	}
	public := string(common.JSON(value))
	if strings.Contains(public, "user-secret") || !strings.Contains(public, "8080") {
		t.Fatalf("草稿输出应只隐藏凭据值: %s", public)
	}
	item := models.OperationItem{Action: "create_build", Params: common.JSON(build)}
	plan := models.OperationPlan{Draft: captured.plan.Draft, Items: []models.OperationItem{item}}
	views := []any{plan.Redacted(), item.Redacted(), &models.Run{Plan: common.JSON(plan)}}
	for _, view := range views {
		if strings.Contains(string(common.JSON(view)), "user-secret") {
			t.Fatal("确认清单、增量事件和运行快照都不能回显凭据")
		}
	}
	if build.Variables["k1"] != "user-secret" || !strings.Contains(string(plan.Items[0].Params), "user-secret") {
		t.Fatal("脱敏不得修改执行参数")
	}
}

func TestBuildSubmissionUsesTemplateMetadataForOverrides(t *testing.T) {
	store, mock := mockStore(t)
	template := []byte(`{"pre_build":{"envs":[{"key":"k1","is_credential":true,"type":"string","value":"old"},{"key":"enabled","type":"bool","value":true}]}}`)
	posts := 0
	platform := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/build/build":
			if r.Method == http.MethodGet {
				w.Write([]byte(`[]`))
				return
			}
			var body commonmodels.Build
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			envs := body.TargetRepos[0].Envs
			if len(envs) != 2 || envs[0].Value != "new-secret" || !envs[0].IsCredential || envs[1].Value != "false" || envs[1].Type != "bool" {
				t.Fatalf("提交参数丢失类型或凭据标记: %v", envs)
			}
			posts++
			w.Write([]byte(`{}`))
		case "/api/service/services/api":
			w.Write([]byte(`{"service_module":[{"name":"api"}]}`))
		case "/api/template/build/general":
			w.Write(template)
		case "/api/code/codehost":
			w.Write([]byte(`[{"id":1,"type":"gitlab"}]`))
		default:
			t.Fatalf("意外请求: %s", r.URL)
		}
	})
	s := &AgentService{Store: store, Platform: platform}
	e := reportExecution("project_onboarding")
	e.RootID = e.Run.ID
	args := models.BuildInput{Name: "build", TemplateID: "general", TemplateFingerprint: fingerprint(template), ServiceName: "api", ServiceModule: "api", Repositories: []models.GitConfig{{CodehostID: 1, Owner: "team", Repo: "repo", Branch: "main"}}, Variables: map[string]string{"k1": "new-secret", "enabled": "false"}}
	item := models.OperationItem{ID: "build", Action: "create_build", ProjectKey: "demo", Params: common.JSON(args), Status: "pending"}
	expectClaim(mock, e, models.OperationPlan{Items: []models.OperationItem{item}}, "executing", true)
	if _, err := s.submitOperation(context.Background(), e, item); err != nil || posts != 1 {
		t.Fatalf("构建应正常提交一次: %d %v", posts, err)
	}
}
