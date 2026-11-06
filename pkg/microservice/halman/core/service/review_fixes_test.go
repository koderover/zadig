package service

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

func TestHelmTemplateContentChangeInvalidatesConfirmationAndSubmission(t *testing.T) {
	digest := "original-content"
	platform := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatal("模板发生变化后不能提交写请求")
		}
		switch r.URL.Path {
		case "/api/template/charts/base":
			if r.URL.Query().Get("metadata_only") != "true" {
				t.Fatal("校验模板不能下载文件")
			}
			w.Write(common.JSON(map[string]any{"name": "base", "sha1": digest, "revision": 1, "variables": []any{map[string]string{"key": "PORT", "value": "8080"}}}))
		case "/api/project/products/p/services":
			w.Write([]byte(`{"services":[]}`))
		default:
			t.Fatalf("意外读取: %s", r.URL)
		}
	})
	s := &AgentService{Platform: platform}
	e := reportExecution("service_onboarding")
	raw, err := platform.catalog(context.Background(), e.Authorization, models.CatalogRequest{Kind: "helm_template", ID: "base"})
	if err != nil {
		t.Fatal(err)
	}
	args := models.ServiceInput{Name: "api", Source: "helm_template", TemplateID: "base", TemplateFingerprint: fingerprint(raw)}
	plan := models.OperationPlan{Items: []models.OperationItem{{Action: "create_service", ProjectKey: "p", Params: common.JSON(args)}}}
	if err := s.verifyPlan(context.Background(), e, plan); err != nil {
		t.Fatalf("未变化的模板应可确认: %v", err)
	}
	digest = "changed-content-with-identical-variables"
	if err := s.verifyPlan(context.Background(), e, plan); err == nil {
		t.Fatal("内容变化而变量不变，也必须阻止确认")
	}
	if _, err := s.submitOperation(context.Background(), e, plan.Items[0]); err == nil {
		t.Fatal("确认后的模板变化也必须在提交前拒绝")
	}
	digest = ""
	if _, err := platform.catalog(context.Background(), e.Authorization, models.CatalogRequest{Kind: "helm_template", ID: "base"}); err == nil {
		t.Fatal("没有内容摘要时不能降级成变量指纹")
	}
}

func TestServiceRepairGoalCannotCompleteFromDiagnosisOrBeDowngraded(t *testing.T) {
	for _, goal := range []string{"diagnose", "restart_service", "rollback_environment", ""} {
		e := reportExecution("service_diagnosis")
		e.SkillInput = common.JSON(models.ServiceDiagnosisInput{Goal: goal})
		e.EvidenceRead.Store(true)
		if diagnosisGoalComplete(e) != (goal == "diagnose") {
			t.Fatalf("只有只读诊断目标能由诊断证据完成: %s", goal)
		}
		if goal == "restart_service" || goal == "rollback_environment" {
			if _, err := resolveSkillInput(context.Background(), &AgentService{}, e, models.ServiceDiagnosisInput{Goal: "diagnose"}); err == nil {
				t.Fatal("同次执行不能把修复降级为诊断后计成功")
			}
		}
	}
}

func TestServiceRepairDiagnosisKeepsWaitingForConfirmationPlan(t *testing.T) {
	store, mock := mockStore(t)
	e := reportExecution("service_diagnosis")
	e.RootID = e.Run.ID
	platform := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/authorization/auth-info" {
			w.Write([]byte(`{"project_auth_info":{"p":{"env":{"view":true}}}}`))
			return
		}
		if r.Method != http.MethodGet {
			t.Fatal("诊断修复目标也不得直接写入")
		}
		if strings.HasSuffix(r.URL.Path, "/pods") {
			w.Write([]byte(`{"service_name":"api","pods":[]}`))
			return
		}
		w.Write([]byte(`{}`))
	})
	expectReportScope(mock, false)
	expectSkillInputSave(mock, e, "awaiting_input")
	s := &AgentService{Store: store, Platform: platform}
	value, err := nativeSkillTool(t, s, "service_diagnosis").Call(context.WithValue(context.Background(), executionKey{}, e), []byte(`{"goal":"restart_service","service_name":"api"}`))
	if err != nil || !e.EvidenceRead.Load() || e.Outcome != "awaiting_input" || diagnosisGoalComplete(e) || !strings.Contains(string(common.JSON(value)), "确认清单") {
		t.Fatalf("取得证据后仍应等待修复清单，不能计完成: %v %v", value, err)
	}
}

func TestDirectRollbackWithoutVersionSavesGoalAndAsksBeforeQuery(t *testing.T) {
	store, mock := mockStore(t)
	e := reportExecution("service_diagnosis")
	e.RootID = e.Run.ID
	expectSkillInputSave(mock, e, "awaiting_input")
	s := &AgentService{Store: store}
	value, err := s.prepareRepair(context.WithValue(context.Background(), executionKey{}, e), models.RepairInput{Action: "rollback_environment", Target: models.ServiceTarget{ProjectKey: "p", EnvName: "dev", ServiceName: "api"}})
	if err != nil || e.Outcome != "awaiting_input" || diagnosisGoalComplete(e) || !strings.Contains(string(common.JSON(value)), "历史版本") || !strings.Contains(string(e.SkillInput), "rollback_environment") {
		t.Fatalf("直接修复也必须先记录目标并补齐版本，不得查询或计成功: %v %v", value, err)
	}
}

func TestCatalogSearchForwardsSupportedFiltersAndRejectsOversizedKeywords(t *testing.T) {
	platform := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		switch r.URL.Path {
		case "/api/v1/codehosts":
			w.Write([]byte(`[{"id":1,"type":"gitlab"}]`))
			return
		case "/api/code/codehost/1/projects", "/api/code/codehost/1/branches":
			if query.Get("key") != "backend" || query.Get("per_page") != "50" {
				t.Fatal("仓库与分支应复用原 key 搜索")
			}
		case "/api/v1/picket/projects":
			if query.Get("filter") != "backend" || query.Get("ungrouped") != "false" {
				t.Fatal("项目应复用 filter 并传入有效分组参数")
			}
		default:
			t.Fatalf("意外请求: %s", r.URL)
		}
		w.Write([]byte(`[]`))
	})
	for _, kind := range []string{"projects", "repositories", "branches"} {
		if _, err := platform.catalog(context.Background(), token("user"), models.CatalogRequest{Kind: kind, Keyword: "backend", CodehostID: 1, RepoOwner: "team", RepoName: "repo"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := platform.catalog(context.Background(), token("user"), models.CatalogRequest{Kind: "projects", Keyword: strings.Repeat("a", 129)}); err == nil {
		t.Fatal("超长关键词必须在请求前拒绝")
	}
	filtered, err := filterCatalogNames(json.RawMessage(`{"chartTemplates":[{"name":"Backend"},{"name":"frontend"}],"systemVariables":[{"key":"service"}]}`), "backend")
	if err != nil || strings.Contains(string(filtered), "frontend") || !strings.Contains(string(filtered), "systemVariables") {
		t.Fatalf("完整目录仅筛选资源名称，不破坏其他元数据: %s %v", filtered, err)
	}
	filtered, err = filterCatalogNames(json.RawMessage(`[{"reg_addr":"harbor.local","namespace":""}]`), "harbor")
	if err != nil || !strings.Contains(string(filtered), "harbor.local") {
		t.Fatalf("没有 namespace 的镜像仓库也应可按地址搜索: %s %v", filtered, err)
	}
	entries := []map[string]string{}
	for i := 0; i < 51; i++ {
		entries = append(entries, map[string]string{"name": "backend"})
	}
	filtered, err = filterCatalogNames(common.JSON(entries), "backend")
	var matches []map[string]string
	if err != nil || json.Unmarshal(filtered, &matches) != nil || len(matches) != 50 {
		t.Fatalf("完整目录也应限制模型候选数量: %s %v", filtered, err)
	}
}
