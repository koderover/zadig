package service

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

type capturedDraft struct{ plan models.OperationPlan }

func (c *capturedDraft) Match(value driver.Value) bool {
	text, ok := value.(string)
	return ok && json.Unmarshal([]byte(text), &c.plan) == nil
}

func TestOnboardingSupplementReusesPersistedChecksAndInvalidatesDependencies(t *testing.T) {
	store, mock := mockStore(t)
	reads := map[string]int{}
	digest := "original"
	platform := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("准备及模板冲突不能写入: %s", r.URL)
		}
		reads[r.URL.Path]++
		switch {
		case r.URL.Path == "/api/v1/authorization/auth-info":
			w.Write([]byte(`{"is_system_admin":true}`))
		case r.URL.Path == "/api/code/codehost":
			w.Write([]byte(`[{"id":1,"type":"gitlab"}]`))
		case r.URL.Path == "/api/project/products/demo/services":
			w.Write([]byte(`{"services":[]}`))
		case strings.HasPrefix(r.URL.Path, "/api/template/"):
			w.Write(common.JSON(map[string]any{"version": digest, "variables": []map[string]any{{"key": "PORT", "type": "string", "value": "80"}}}))
		default:
			t.Fatalf("意外查询: %s", r.URL)
		}
	})
	s := &AgentService{Store: store, Platform: platform}
	patches := []string{
		`{"project_name":"demo","project_key":"demo","project_type":"k8s","services":[{"name":"service1","source":"yaml_template","template_id":"general"},{"name":"service2","source":"yaml_template","template_id":"general"}],"builds":[{"name":"build1","template_id":"general","service_name":"service1","service_module":"service1","repositories":[{"codehost_id":1,"owner":"team","repo":"repo","branch":"main"}]}],"environment":{"name":"dev","cluster_id":"local"}}`,
		`{"environment":{"namespace":"demo-dev"}}`,
		`{"services":[{"name":"service1","variables":{"PORT":"8080"}}]}`,
		`{"builds":[{"name":"build1","service_module":"worker"}]}`,
		`{"builds":[{"name":"build1","template_id":"other"}]}`,
		`{"services":[{"name":"service1","template_id":"other","template_fingerprint":"model-provided"}]}`,
	}
	wantReads := []map[string]int{
		{"/api/template/yaml/general": 1, "/api/template/build/general": 1, "/api/code/codehost": 1},
		{"/api/template/yaml/general": 1, "/api/template/build/general": 1, "/api/code/codehost": 1},
		{"/api/template/yaml/general": 2, "/api/template/build/general": 1, "/api/code/codehost": 1},
		{"/api/template/yaml/general": 2, "/api/template/build/general": 1, "/api/code/codehost": 1},
		{"/api/template/yaml/general": 2, "/api/template/build/general": 1, "/api/template/build/other": 1, "/api/code/codehost": 1},
		{"/api/template/yaml/general": 2, "/api/template/yaml/other": 1, "/api/template/build/general": 1, "/api/template/build/other": 1, "/api/code/codehost": 1},
	}
	var saved capturedDraft
	for i, patch := range patches {
		e := reportExecution("project_onboarding")
		e.Run.ID, e.RootID = fmt.Sprintf("round-%d", i), "root"
		e.Cutoff = time.Now().Add(-time.Hour)
		if i == 0 {
			e.RootID, e.Run.ID = "root", "root"
		} else {
			source := &models.Run{ID: fmt.Sprintf("round-%d", i-1), Plan: common.JSON(saved.plan)}
			expectSavedQueryInput(mock, e, source, "[]")
		}
		expectSkillInputSave(mock, e, "awaiting_input", &saved)
		value, err := nativeSkillTool(t, s, "project_onboarding").Call(context.WithValue(context.Background(), executionKey{}, e), []byte(patch))
		if err != nil {
			t.Fatalf("第 %d 轮: %v", i, err)
		}
		if len(saved.plan.DraftChecks) == 0 {
			t.Fatal("校验结果必须随草稿持久化")
		}
		if strings.Contains(string(common.JSON(value)), "draft_checks") {
			t.Fatal("内部校验记录不能作为补参返回值")
		}
		for path, count := range wantReads[i] {
			if reads[path] != count {
				t.Fatalf("第 %d 轮不应重读无关资源: %s=%d，期望 %d", i, path, reads[path], count)
			}
		}
		if i == 0 && !reflect.DeepEqual(saved.plan.MissingFields, []string{"环境的 Kubernetes namespace", "环境的镜像仓库"}) {
			t.Fatalf("缺项应一次汇总: %v", saved.plan.MissingFields)
		}
	}
	service := saved.plan.Draft.Services[0]
	if len(service.Variables) != 0 || service.TemplateFingerprint == "model-provided" {
		t.Fatal("换模板后须清除旧变量并使用后端指纹")
	}
	e := reportExecution("project_onboarding")
	e.OnboardingChecks = saved.plan.DraftChecks
	item := models.OperationItem{Action: "create_service", ProjectKey: "demo", Params: common.JSON(service)}
	digest = "changed"
	if err := s.verifyPlan(context.Background(), e, models.OperationPlan{Items: []models.OperationItem{item}}); err == nil || !strings.Contains(err.Error(), "模板已变化") {
		t.Fatalf("确认前必须绕过草稿缓存、拒绝模板变化: %v", err)
	}
	if _, err := s.submitOperation(context.Background(), e, item); err == nil || !strings.Contains(err.Error(), "模板已变化") {
		t.Fatalf("提交前必须绕过草稿缓存、拒绝模板变化: %v", err)
	}
}

func TestOnboardingTemplateCheckDoesNotCacheFailuresOrTrustClientFingerprint(t *testing.T) {
	reads := 0
	platform := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		reads++
		if reads == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.Write([]byte(`{"variables":[{"key":"cred","is_credential":true,"value":"configured"}]}`))
	})
	s := &AgentService{Platform: platform}
	e := reportExecution("project_onboarding")
	for i := 0; i < 2; i++ {
		_, err := s.checkOnboardingTemplate(context.Background(), e, "template", "demo", "yaml_template", "general", nil)
		if (err != nil) != (i == 0) {
			t.Fatalf("失败结果不得缓存: %v", err)
		}
	}
	check, err := s.checkOnboardingTemplate(context.Background(), e, "template", "demo", "yaml_template", "general", map[string]string{"cred": "override"})
	if err != nil || !reflect.DeepEqual(check.Credentials, []string{"cred"}) {
		t.Fatalf("修改凭据变量应重新校验并保留凭据标记: %+v %v", check, err)
	}
	if reads != 3 {
		t.Fatalf("预期重读失败及修改的检查: %d", reads)
	}
	if strings.Contains(string(common.JSON(e.OnboardingChecks)), "configured") {
		t.Fatal("缓存不得持久化模板凭据")
	}
}
