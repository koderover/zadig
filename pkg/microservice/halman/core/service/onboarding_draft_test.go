package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
	"trpc.group/trpc-go/trpc-agent-go/tool"
)

func onboardingInput(t *testing.T, text string) models.OnboardingInput {
	t.Helper()
	var input models.OnboardingInput
	if err := json.Unmarshal([]byte(text), &input); err != nil {
		t.Fatal(err)
	}
	return input
}

func TestOnboardingPartialChangesKeepThreeServicesAndExplicitZeroValues(t *testing.T) {
	base := onboardingInput(t, `{"project_name":"示例","project_key":"demo","project_type":"k8s","expected_service_count":3,"public":true,"create_workflow":true,"services":[{"name":"api","source":"yaml_git","git":{"codehost_id":1,"owner":"team","repo":"repo","branch":"main","path":"api","is_dir":true}},{"name":"web","source":"yaml_git","git":{"codehost_id":1,"owner":"team","repo":"repo","branch":"main","path":"web","is_dir":true}},{"name":"worker","source":"yaml_template","template_id":"old","variables":{"PORT":"80"}}],"environment":{"name":"dev","cluster_id":"cluster"}}`)
	patch := onboardingInput(t, `{"services":[{"name":"web","git":{"path":"deploy/web"}}]}`)
	result, err := mergeOnboardingDraft(&base, patch)
	if err != nil || len(result.Services) != 3 || result.ExpectedServiceCount != 3 || result.Services[1].Git.Path != "deploy/web" || result.Services[1].Git.Branch != "main" || !result.Services[1].Git.IsDir || !result.Public || !result.CreateWorkflow || !reflect.DeepEqual(base.Services[0], result.Services[0]) || !reflect.DeepEqual(base.Services[2], result.Services[2]) {
		t.Fatalf("局部路径修改不得丢服务或共享配置: %+v %v", result, err)
	}
	if base.Services[1].Git.Path != "web" || patch.ProvidedFields()["services"] == nil {
		t.Fatal("合并不能修改原草稿或吞掉输入字段")
	}
	result, err = mergeOnboardingDraft(&result, onboardingInput(t, `{"public":false,"create_workflow":false,"environment":null,"services":[{"name":"web","git":{"is_dir":false}}]}`))
	if err != nil || result.Public || result.CreateWorkflow || result.Environment != nil || result.Services[1].Git.IsDir || result.Services[1].Git.Branch != "main" {
		t.Fatalf("显式 false 和清空不能当成字段省略: %+v %v", result, err)
	}
	result, err = mergeOnboardingDraft(&base, onboardingInput(t, `{"remove_services":["worker"],"expected_service_count":2}`))
	if err != nil || len(result.Services) != 2 || result.ExpectedServiceCount != 2 || len(result.RemoveServices) != 0 {
		t.Fatalf("只有明确移除才可减少服务: %+v %v", result, err)
	}
	result, err = mergeOnboardingDraft(&base, onboardingInput(t, `{"services":[]}`))
	if err != nil || len(result.Services) != 3 {
		t.Fatal("空数组不能隐式删除已有服务")
	}
	result, err = mergeOnboardingDraft(&base, onboardingInput(t, `{"services":[{"name":"worker","template_id":"new"}]}`))
	if err != nil || len(result.Services[2].Variables) != 0 {
		t.Fatal("更换模板后必须重新核对变量，不能保留旧模板覆盖值")
	}
	for _, text := range []string{`{"expected_service_count":0}`, `{"expected_service_count":21}`, `{"remove_services":["missing"]}`, `{"services":[{"name":"api"},{"name":"api"}]}`, `{"services":[null]}`} {
		if _, err := mergeOnboardingDraft(&base, onboardingInput(t, text)); err == nil {
			t.Fatalf("错误修改不能悄悄接受: %s", text)
		}
	}
	result, err = mergeOnboardingDraft(&base, onboardingInput(t, `{"services":[{"name":"extra"}]}`))
	if err != nil || result.ExpectedServiceCount != 3 || len(result.Services) != 4 {
		t.Fatal("模型追加条目不能隐式修改用户要求的服务总数")
	}
	partial := onboardingInput(t, `{"expected_service_count":3,"services":[{"name":"api"}]}`)
	result, err = mergeOnboardingDraft(&partial, onboardingInput(t, `{"services":[{"name":"web"},{"name":"worker"}]}`))
	if err != nil || result.ExpectedServiceCount != 3 || len(result.Services) != 3 {
		t.Fatal("补齐三个服务时不能误增总数")
	}
}

func expectOnboardingDraftSave(mock sqlmock.Sqlmock) {
	expectSkillInputSave(mock, reportExecution(""), "awaiting_input")
}

func TestOnboardingWorkflowScope(t *testing.T) {
	for _, tc := range []struct {
		name, skill, input string
		draft              *models.OnboardingInput
		reject             bool
		workflow           bool
	}{
		{"服务初始化拒绝工作流", "service_onboarding", `{"create_workflow":true}`, nil, true, false},
		{"恢复草稿也拒绝工作流", "service_onboarding", `{}`, &models.OnboardingInput{CreateWorkflow: true}, true, false},
		{"可以关闭草稿中的工作流", "service_onboarding", `{"create_workflow":false}`, &models.OnboardingInput{CreateWorkflow: true}, false, false},
		{"项目初始化保留工作流", "project_onboarding", `{"create_workflow":true}`, nil, false, true},
		{"项目初始化默认工作流", "project_onboarding", `{}`, nil, false, true},
		{"明确拒绝工作流", "project_onboarding", `{"create_workflow":false}`, nil, false, false},
		{"续轮保留拒绝工作流", "project_onboarding", `{}`, &models.OnboardingInput{CreateWorkflow: false}, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, mock := mockStore(t)
			e := reportExecution(tc.skill)
			e.Run.Context = models.PageContext{}
			e.RootID = e.Run.ID
			e.OnboardingDraft = tc.draft
			s := &AgentService{Store: store}
			if !tc.reject {
				expectOnboardingDraftSave(mock)
			}
			_, err := s.prepareOnboarding(context.WithValue(context.Background(), executionKey{}, e), onboardingInput(t, tc.input), tc.skill == "project_onboarding")
			if tc.reject {
				if err == nil || !strings.Contains(err.Error(), "服务初始化不支持创建工作流") {
					t.Fatalf("服务初始化必须拒绝工作流: %v", err)
				}
			} else if err != nil || e.OnboardingDraft.CreateWorkflow != tc.workflow {
				t.Fatalf("工作流开关错误: %+v %v", e.OnboardingDraft, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestServiceOnboardingUsesPageProjectOnlyForNewDraft(t *testing.T) {
	for _, tc := range []struct {
		name, input, project, projectType string
		draft                             *models.OnboardingInput
		readProject                       bool
	}{
		{"page_default", `{}`, "p", "helm", nil, true},
		{"explicit_project", `{"project_key":"other"}`, "other", "helm", nil, true},
		{"explicit_empty", `{"project_key":""}`, "", "", nil, false},
		{"saved_target", `{}`, "original", "helm", &models.OnboardingInput{ProjectKey: "original", ProjectType: "helm"}, false},
		{"saved_without_target", `{}`, "", "", &models.OnboardingInput{ExpectedServiceCount: 3}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, mock := mockStore(t)
			e := reportExecution("service_onboarding")
			e.RootID = e.Run.ID
			e.OnboardingDraft = tc.draft
			reads := 0
			platform := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
				reads++
				if !tc.readProject || r.Method != http.MethodGet || r.URL.Path != "/api/project/products/"+tc.project || r.Header.Get("Authorization") != token("user") {
					t.Fatalf("只按实际目标只读查询项目，不能创建资源或读取页面中的环境: %s %s", r.Method, r.URL)
				}
				w.Write(common.JSON(map[string]any{"product_name": tc.project, "product_feature": map[string]string{"deploy_type": "helm"}}))
			})
			if tc.readProject {
				mock.ExpectBegin()
				mock.ExpectQuery("SELECT resource_scopes FROM ai_run").WithArgs("run", "user").WillReturnRows(sqlmock.NewRows([]string{"scopes"}).AddRow("[]"))
				mock.ExpectExec("UPDATE ai_run SET context=.*resource_scopes=").WithArgs(sqlmock.AnyArg(), string(common.JSON([]models.ResourceScope{{PageContext: models.PageContext{PageType: "project", ProjectKey: tc.project}}})), "run", "user").WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			}
			expectOnboardingDraftSave(mock)
			s := &AgentService{Store: store, Platform: platform}
			value, err := nativeSkillTool(t, s, "service_onboarding").Call(context.WithValue(context.Background(), executionKey{}, e), []byte(tc.input))
			if err != nil || e.OnboardingDraft == nil || e.OnboardingDraft.ProjectKey != tc.project || e.OnboardingDraft.ProjectType != tc.projectType || e.Outcome != "awaiting_input" {
				t.Fatalf("应自动补齐页面项目及类型，只追问缺失的服务信息: %+v %v", value, err)
			}
			if (reads == 1) != tc.readProject {
				t.Fatalf("项目查询次数错误: %d", reads)
			}
			if tc.name == "page_default" {
				expectOnboardingDraftSave(mock)
				value, err = nativeSkillTool(t, s, "service_onboarding").Call(context.WithValue(context.Background(), executionKey{}, e), []byte(`{"expected_service_count":3,"services":[{"name":"service1","source":"helm_repository","chart_repo":"test","chart_name":"service1"},{"name":"service2","source":"helm_repository","chart_repo":"test","chart_name":"service2"},{"name":"service3","source":"helm_repository","chart_repo":"test","chart_name":"service3"}]}`))
				text := string(common.JSON(value))
				if err != nil || e.OnboardingDraft.ProjectKey != "p" || e.OnboardingDraft.ProjectType != "helm" || len(e.OnboardingDraft.Services) != 3 || reads != 1 || strings.Contains(text, "项目标识") || strings.Contains(text, "项目类型") || !strings.Contains(text, "具体版本") {
					t.Fatalf("补齐三个 Chart 服务后应沿用项目，只缺具体版本: %+v %v", value, err)
				}
			}
		})
	}
}

func TestOnboardingCountMismatchAsksBeforeReadingConfiguration(t *testing.T) {
	store, mock := mockStore(t)
	e := reportExecution("project_onboarding")
	e.RootID = "run"
	e.Cutoff = time.Now().Add(-time.Hour)
	s := &AgentService{Store: store}
	expectOnboardingDraftSave(mock)
	value, err := s.prepareProject(context.WithValue(context.Background(), executionKey{}, e), onboardingInput(t, `{"project_name":"示例","project_key":"demo","project_type":"k8s","expected_service_count":3,"services":[{"name":"api","source":"yaml_template","template_id":"t"},{"name":"web","source":"yaml_template","template_id":"t"}]}`))
	if err != nil || e.Outcome != "awaiting_input" || !strings.Contains(string(common.JSON(value)), "当前清单为 2 个") || e.OnboardingDraft.ExpectedServiceCount != 3 {
		t.Fatalf("少服务时必须保存草稿补问，不查询或创建资源: %v %v", value, err)
	}
}

func TestOnboardingMissingFieldsAskForSpecificServiceInformation(t *testing.T) {
	for _, tc := range []struct {
		name, services, expected string
	}{
		{"未提供服务", `[]`, "本次要新建的 3 个服务的名称"},
		{"缺少配置来源", `[{"name":"api"},{"name":"web"},{"name":"worker"}]`, "服务 api 的服务配置来源"},
		{"缺少服务模板", `[{"name":"api","source":"yaml_template"},{"name":"web","source":"yaml_template"},{"name":"worker","source":"yaml_template"}]`, "服务 api 要使用的已有服务模板（不是构建模板）"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, mock := mockStore(t)
			e := reportExecution("project_onboarding")
			e.RootID = "run"
			e.Cutoff = time.Now().Add(-time.Hour)
			expectOnboardingDraftSave(mock)
			s := &AgentService{Store: store}
			args := onboardingInput(t, `{"project_name":"示例","project_key":"demo","project_type":"k8s","expected_service_count":3,"services":`+tc.services+`}`)
			value, err := s.prepareProject(context.WithValue(context.Background(), executionKey{}, e), args)
			text := string(common.JSON(value))
			if err != nil || !strings.Contains(text, tc.expected) || strings.Contains(text, "当前清单为 0 个") {
				t.Fatalf("应逐项补问，空清单不重复提示数量不足: %s %v", text, err)
			}
		})
	}
}

func TestThreeServiceTemplateMissingFieldsUseDraftAndMergeThroughNativeTool(t *testing.T) {
	for _, emptyEnvironment := range []bool{false, true} {
		t.Run(fmt.Sprintf("empty_environment=%t", emptyEnvironment), func(t *testing.T) {
			store, mock := mockStore(t)
			reads := 0
			platform := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v1/authorization/auth-info" {
					w.Write([]byte(`{"is_system_admin":true}`))
					return
				}
				if r.Method != http.MethodGet || !strings.HasPrefix(r.URL.Path, "/api/template/yaml/") {
					t.Fatalf("缺参期间只能读取模板: %s %s", r.Method, r.URL)
				}
				reads++
				w.Write([]byte(`{"service_variable_kvs":[{"key":"PORT","type":"string","value":""}]}`))
			})
			s := &AgentService{Store: store, Platform: platform}
			e := reportExecution("project_onboarding")
			e.RootID = "run"
			e.Cutoff = time.Now().Add(-time.Hour)
			ctx := context.WithValue(context.Background(), executionKey{}, e)
			for _, fn := range s.tools() {
				if fn.Declaration().Name != "project_onboarding" {
					continue
				}
				callable := fn.(tool.CallableTool)
				schema := fn.Declaration().InputSchema
				if schema.Properties["expected_service_count"] == nil || schema.Properties["remove_services"] == nil {
					t.Fatal("工具必须暴露总数与明确移除字段")
				}
				for _, required := range schema.Required {
					if required == "project_key" || required == "services" || required == "create_workflow" {
						t.Fatal("工具必须允许局部修改省略旧参数")
					}
				}
				expectOnboardingDraftSave(mock)
				value, err := callable.Call(ctx, []byte(`{"project_name":"示例","project_key":"demo","project_type":"k8s","expected_service_count":3,"services":[{"name":"api","source":"yaml_template","template_id":"t"},{"name":"web","source":"yaml_template","template_id":"t"},{"name":"worker","source":"yaml_template","template_id":"t"}]}`))
				if err != nil || e.Outcome != "awaiting_input" {
					t.Fatalf("模板缺参应进入草稿而非报错: %v %v", value, err)
				}
				text := string(common.JSON(value))
				for _, name := range []string{"api", "web", "worker"} {
					if !strings.Contains(text, "服务 "+name+" 的模板必填变量 PORT") {
						t.Fatalf("应按三个服务汇总补问: %s", text)
					}
				}
				expectOnboardingDraftSave(mock)
				value, err = callable.Call(ctx, []byte(`{"services":[{"name":"web","variables":{"PORT":"8080"}}]}`))
				if err != nil || len(e.OnboardingDraft.Services) != 3 || e.OnboardingDraft.ProjectKey != "demo" || e.OnboardingDraft.Services[1].Variables["PORT"] != "8080" {
					t.Fatalf("局部补参应保留项目和另外两个服务: %v %v", value, err)
				}
				missing := value.(map[string]any)["missing_fields"].([]string)
				if len(missing) != 2 || strings.Contains(strings.Join(missing, " "), "服务 web") || reads != 2 {
					t.Fatalf("只应询问剩余两服务的缺参: %v，读取 %d 次", missing, reads)
				}
				expectOnboardingDraftSave(mock)
				value, err = callable.Call(ctx, []byte(`{"services":[{"name":"api","variables":{"PORT":"8081"}},{"name":"worker","variables":{"PORT":"8082"}}]}`))
				if err != nil || e.Outcome != "awaiting_input" || !strings.Contains(string(common.JSON(value)), "只创建环境不部署服务") || !e.OnboardingDraft.CreateWorkflow {
					t.Fatalf("服务补齐后必须询问环境选择并保留默认工作流: %v %v", value, err)
				}
				expectOnboardingDraftSave(mock)
				value, err = callable.Call(ctx, []byte(`{"environment":{"name":"dev","cluster_id":"local","namespace":"demo-dev","services":["api","web","worker"]}}`))
				if err != nil {
					t.Fatal(err)
				}
				missing = value.(map[string]any)["missing_fields"].([]string)
				if !reflect.DeepEqual(missing, []string{"环境的镜像仓库"}) || e.OnboardingDraft.Environment.Namespace != "demo-dev" {
					t.Fatalf("仅补问缺失的镜像仓库，保留已填环境信息: %v", value)
				}
				mock.ExpectBegin()
				mock.ExpectQuery("SELECT id FROM ai_conversation").WithArgs("conversation", "user").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("conversation"))
				mock.ExpectQuery("SELECT active_plan_run_id,skill_status FROM ai_run").WithArgs("run", sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"active", "status"}).AddRow(nil, "running"))
				mock.ExpectQuery("SELECT plan_version,status FROM ai_run").WithArgs("run").WillReturnRows(sqlmock.NewRows([]string{"version", "status"}).AddRow(0, "running"))
				mock.ExpectExec("UPDATE ai_run SET plan=.*plan_version=.*plan_status='pending_confirmation'").WithArgs(sqlmock.AnyArg(), 1, sqlmock.AnyArg(), "run").WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec("UPDATE ai_run SET active_plan_run_id=.*skill_status='pending_confirmation'").WithArgs("run", "run").WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
				input := `{"skip_environment":true,"environment":null}`
				if emptyEnvironment {
					input = `{"environment":{"registry_id":"registry","services":[]}}`
				}
				value, err = callable.Call(ctx, []byte(input))
				if err != nil || e.Outcome != "pending_confirmation" {
					t.Fatalf("补齐后必须生成确认清单: %v %v", value, err)
				}
				plan := value.(map[string]any)["plan"].(models.OperationPlan)
				itemCount := 5
				if emptyEnvironment {
					itemCount = 6
					if plan.Items[4].Action != "create_environment" || plan.Draft.Environment == nil || len(plan.Draft.Environment.Services) != 0 || !strings.Contains(strings.Join(plan.Notes, " "), "仅创建环境，不部署服务") {
						t.Fatalf("空环境应纳入清单并明确不部署服务: %+v", plan)
					}
				}
				if len(plan.Items) != itemCount || plan.Items[itemCount-1].Action != "create_workflow" || len(plan.Items[itemCount-1].DependsOn) != itemCount-1 || plan.Draft == nil || plan.Draft.SkipEnvironment == emptyEnvironment || plan.Draft.ExpectedServiceCount != 3 || len(plan.Draft.Services) != 3 || plan.Draft.Services[1].Variables["PORT"] != "8080" || reads != 5 {
					t.Fatalf("完整清单必须保留三个服务及可恢复草稿: %+v", plan)
				}
				return
			}
			t.Fatal("项目初始化工具未注册")
		})
	}
}

func TestMissingTemplateVariablesKeepValidationAndCredentialBoundaries(t *testing.T) {
	raw := []byte(`{"service_variable_kvs":[{"key":"A","type":"string","value":""},{"key":"B","type":"bool","required":true},{"key":"OPTIONAL","type":"string","required":false}]}`)
	_, _, err := templateVariables(raw, nil)
	var missing *missingTemplateVariables
	if !errors.As(err, &missing) || !reflect.DeepEqual(missing.Keys, []string{"A", "B"}) {
		t.Fatalf("应汇总真实服务变量和显式必填项: %v", err)
	}
	if _, _, err := templateVariables(raw, map[string]string{"A": "", "B": "false"}); err != nil {
		t.Fatalf("明确允许的空服务字符串及布尔 false 应有效: %v", err)
	}
	for _, test := range []struct {
		definition string
		values     map[string]string
	}{
		{`{"variables":[{"key":"PASSWORD","required":true}]}`, nil},
		{`{"variables":[{"key":"cred","required":true,"is_credential":true}]}`, nil},
		{string(raw), map[string]string{"unknown": "x"}},
		{string(raw), map[string]string{"A": "x", "B": "invalid"}},
	} {
		_, _, err := templateVariables([]byte(test.definition), test.values)
		if err == nil || errors.As(err, &missing) {
			t.Fatalf("凭据和非法覆盖不能转成普通缺参补问: %v", err)
		}
	}
	if err := checkTemplateVariables([]byte(`{"pre_build":{"envs":[{"key":"TARGET","type":"choice","choice_option":["dev","prod"],"required":true}]}}`), map[string]string{"TARGET": "dev"}); err != nil {
		t.Fatalf("构建模板应识别已有 choice_option: %v", err)
	}
}
