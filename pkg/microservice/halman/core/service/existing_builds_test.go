package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

func TestExistingServiceBuildsDoNotRequireServiceConfiguration(t *testing.T) {
	for _, kind := range []string{"k8s", "helm"} {
		for _, references := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/references=%t", kind, references), func(t *testing.T) {
				store, mock := mockStore(t)
				reads, writes := 0, 0
				platform := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/api/project/products/p":
						w.Write([]byte(`{"product_name":"p","product_feature":{"deploy_type":"` + kind + `"}}`))
					case "/api/project/products/p/services":
						w.Write([]byte(`{"services":[["service1","service2","service3"]]}`))
					case "/api/v1/authorization/auth-info":
						w.Write([]byte(`{"project_auth_info":{"p":{"build":{"create":true}}}}`))
					case "/api/template/build/000000000000000000000003":
						w.Write([]byte(`{"name":"general"}`))
					case "/api/code/codehost":
						w.Write([]byte(`[{"id":1,"type":"github"}]`))
					case "/api/build/build":
						if r.Method == http.MethodGet {
							w.Write([]byte(`[]`))
							return
						}
						var body map[string]any
						if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
							t.Fatal(err)
						}
						if body["template_id"] != "000000000000000000000003" || body["product_name"] != "p" {
							t.Fatalf("构建参数错误: %v", body)
						}
						writes++
						w.Write([]byte(`{}`))
					default:
						if strings.HasPrefix(r.URL.Path, "/api/service/services/") && kind == "k8s" {
							reads++
							w.Write([]byte(`{"service_module":[{"name":"app"}]}`))
						} else if strings.HasPrefix(r.URL.Path, "/api/service/helm/p/") && kind == "helm" {
							reads++
							w.Write([]byte(`{"service_modules":[{"name":"app"}]}`))
						} else {
							t.Fatalf("不应创建服务或查询其配置来源: %s %s", r.Method, r.URL)
						}
					}
				})
				s := &AgentService{Store: store, Platform: platform}
				e := reportExecution("service_onboarding")
				e.RootID = "run"
				e.Cutoff = time.Now().Add(-time.Hour)
				args := models.OnboardingInput{ProjectKey: "p", ExpectedServiceCount: 3}
				for i := 1; i <= 3; i++ {
					name := fmt.Sprintf("service%d", i)
					if references {
						args.Services = append(args.Services, models.ServiceInput{Name: name})
					}
					args.Builds = append(args.Builds, models.BuildInput{Name: name + "-build", TemplateID: "000000000000000000000003", ServiceName: name, Repositories: []models.GitConfig{{CodehostID: 1, Owner: "team", Repo: "repo", Branch: "main"}}})
				}
				mock.ExpectBegin()
				mock.ExpectQuery("SELECT resource_scopes FROM ai_run").WithArgs("run", "user").WillReturnRows(sqlmock.NewRows([]string{"scopes"}).AddRow("[]"))
				mock.ExpectExec("UPDATE ai_run SET context=.*resource_scopes=").WithArgs(sqlmock.AnyArg(), `[{"page_type":"project","project_key":"p"}]`, "run", "user").WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
				mock.ExpectBegin()
				mock.ExpectQuery("SELECT id FROM ai_conversation").WithArgs("conversation", "user").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("conversation"))
				mock.ExpectQuery("SELECT active_plan_run_id,skill_status FROM ai_run").WithArgs("run", sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"active", "status"}).AddRow(nil, "running"))
				mock.ExpectQuery("SELECT plan_version,status FROM ai_run").WithArgs("run").WillReturnRows(sqlmock.NewRows([]string{"version", "status"}).AddRow(0, "running"))
				mock.ExpectExec("UPDATE ai_run SET plan=.*plan_version=.*plan_status='pending_confirmation'").WithArgs(sqlmock.AnyArg(), 1, sqlmock.AnyArg(), "run").WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec("UPDATE ai_run SET active_plan_run_id=.*skill_status='pending_confirmation'").WithArgs("run", "run").WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
				value, err := s.prepareServices(context.WithValue(context.Background(), executionKey{}, e), args)
				if err != nil {
					t.Fatal(err)
				}
				plan := value.(map[string]any)["plan"].(models.OperationPlan)
				if len(plan.Items) != 3 || reads != 3 || writes != 0 || plan.Draft.ProjectType != kind {
					t.Fatalf("清单错误: %+v, reads=%d,writes=%d", plan, reads, writes)
				}
				for _, item := range plan.Items {
					var build models.BuildInput
					json.Unmarshal(item.Params, &build)
					if item.Action != "create_build" || len(item.DependsOn) != 0 || build.ServiceModule != "app" {
						t.Fatalf("只允许绑定已有组件创建构建: %+v", item)
					}
					expectClaim(mock, e, plan, "executing", true)
					if _, err := s.submitOperation(context.Background(), e, &item); err != nil {
						t.Fatal(err)
					}
				}
				if writes != 3 || reads != 6 {
					t.Fatalf("提交应重新验证组件并创建三个构建: writes=%d reads=%d", writes, reads)
				}
			})
		}
	}
}

func TestExistingBuildTargetValidation(t *testing.T) {
	for _, tc := range []struct {
		name, modules, target, module, auth, want string
		fail                                      bool
	}{
		{"多组件补问", `[{"name":"api"},{"name":"worker"}]`, "service1", "", "true", "请选择已有服务组件", false},
		{"无模板", `[]`, "service1", "", "true", "未生成可执行操作", false},
		{"无组件", `[]`, "service1", "", "true", "没有可用构建组件", false},
		{"错误组件", `[{"name":"api"}]`, "service1", "other", "true", "不属于服务", true},
		{"服务不存在", `[]`, "missing", "", "true", "不存在", false},
		{"无构建权限", `[]`, "service1", "", "false", "权限", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, mock := mockStore(t)
			platform := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/project/products/p":
					w.Write([]byte(`{"product_name":"p","product_feature":{"deploy_type":"k8s"}}`))
				case "/api/project/products/p/services":
					w.Write([]byte(`{"services":[["service1"]]}`))
				case "/api/v1/authorization/auth-info":
					w.Write([]byte(`{"project_auth_info":{"p":{"build":{"create":` + tc.auth + `}}}}`))
				case "/api/service/services/service1":
					w.Write([]byte(`{"service_module":` + tc.modules + `}`))
				default:
					t.Fatalf("补问期间不能创建或读取模板: %s", r.URL)
				}
			})
			e := reportExecution("service_onboarding")
			e.RootID = "run"
			e.Cutoff = time.Now().Add(-time.Hour)
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT resource_scopes FROM ai_run").WithArgs("run", "user").WillReturnRows(sqlmock.NewRows([]string{"scopes"}).AddRow("[]"))
			mock.ExpectExec("UPDATE ai_run SET context=.*resource_scopes=").WithArgs(sqlmock.AnyArg(), `[{"page_type":"project","project_key":"p"}]`, "run", "user").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()
			if !tc.fail {
				expectOnboardingDraftSave(mock)
			}
			templateID := "000000000000000000000003"
			if tc.name == "无模板" {
				templateID = ""
			}
			value, err := (&AgentService{Store: store, Platform: platform}).prepareServices(context.WithValue(context.Background(), executionKey{}, e), models.OnboardingInput{ProjectKey: "p", Builds: []models.BuildInput{{Name: "build", ServiceName: tc.target, ServiceModule: tc.module, TemplateID: templateID}}})
			if tc.fail {
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("应拒绝非法目标: %v", err)
				}
			} else if err != nil || !strings.Contains(string(common.JSON(value)), tc.want) {
				t.Fatalf("应补问并保留草稿: %v %v", value, err)
			}
		})
	}
}

func TestProjectOnboardingSubmitsOneTemplateBuildForAllTargets(t *testing.T) {
	store, mock := mockStore(t)
	var posted map[string]any
	posts := 0
	workerReads := 0
	platform := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/authorization/auth-info":
			w.Write([]byte(`{"is_system_admin":true}`))
		case "/api/template/yaml/000000000000000000000004", "/api/template/build/000000000000000000000003":
			w.Write([]byte(`{}`))
		case "/api/code/codehost":
			w.Write([]byte(`[{"id":1,"type":"github"}]`))
		case "/api/service/services/api":
			w.Write([]byte(`{"service_module":[{"name":"app"},{"name":"sidecar"}]}`))
		case "/api/service/services/worker":
			workerReads++
			w.Write([]byte(`{"service_module":[{"name":"app"}]}`))
		case "/api/build/build":
			if r.Method == http.MethodGet {
				w.Write([]byte(`[]`))
				return
			}
			if err := json.NewDecoder(r.Body).Decode(&posted); err != nil {
				t.Error(err)
			}
			posts++
			w.Write([]byte(`{}`))
		default:
			t.Errorf("意外请求: %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusNotFound)
		}
	})
	s := &AgentService{Store: store, Platform: platform}
	e := reportExecution("project_onboarding")
	e.RootID = e.Run.ID
	ctx := context.WithValue(context.Background(), executionKey{}, e)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM ai_conversation").WithArgs("conversation", "user").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("conversation"))
	mock.ExpectQuery("SELECT active_plan_run_id,skill_status FROM ai_run").WithArgs("run", sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"active", "status"}).AddRow(nil, "running"))
	mock.ExpectQuery("SELECT plan_version,status FROM ai_run").WithArgs("run").WillReturnRows(sqlmock.NewRows([]string{"version", "status"}).AddRow(0, "running"))
	mock.ExpectExec("UPDATE ai_run SET plan=.*plan_version=.*plan_status='pending_confirmation'").WithArgs(sqlmock.AnyArg(), 1, sqlmock.AnyArg(), "run").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE ai_run SET active_plan_run_id=.*skill_status='pending_confirmation'").WithArgs("run", "run").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	draft := onboardingInput(t, `{"project_key":"demo","project_name":"示例","project_type":"k8s","skip_environment":true,"create_workflow":false,"services":[{"name":"api","source":"yaml_template","template_id":"000000000000000000000004"},{"name":"worker","source":"yaml_template","template_id":"000000000000000000000004"}],"builds":[{"name":"backend-build","template_id":"000000000000000000000003"}]}`)
	e.OnboardingDraft = &draft
	badService := models.BuildInput{Targets: []models.BuildTarget{{ServiceName: "api-typo"}}}
	if _, missing, err := s.resolveBuildTargets(ctx, e, draft, &badService, nil, "backend-build", true); err != nil || len(missing) != 1 || !strings.Contains(missing[0], "api-typo") {
		t.Fatalf("准备阶段应识别未列入清单的服务名: %v, %v", missing, err)
	}
	prepare := nativeSkillTool(t, s, "project_onboarding")
	if _, err := prepare.Call(ctx, []byte(`{"builds":[{"name":"api-build"},{"name":"worker-build"}]}`)); err == nil || !strings.Contains(err.Error(), "api-build") {
		t.Fatalf("续接时不能用新构建名隐式拆分共享构建: %v", err)
	}
	value, err := prepare.Call(ctx, []byte(`{"builds":[{"name":"backend-build","repositories":[{"codehost_id":1,"owner":"team","repo":"backend","branch":"main"}]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	plan := value.(map[string]any)["plan"].(models.OperationPlan)
	if posts != 0 || len(plan.Items) != 4 {
		t.Fatalf("确认前只应生成项目、两个服务和一个构建: %+v", plan.Items)
	}
	item := plan.Items[3]
	if item.Action != "create_build" || !slices.Equal(item.DependsOn, []string{"service-api", "service-worker"}) {
		t.Fatalf("共享构建必须依赖所有待创建服务: %+v", item)
	}
	var build models.BuildInput
	if err := json.Unmarshal(item.Params, &build); err != nil {
		t.Fatal(err)
	}
	if build.Name != "backend-build" || build.TemplateID != "000000000000000000000003" || len(build.Repositories) != 1 || build.Repositories[0].Branch != "main" {
		t.Fatalf("补充源码应沿用原构建名称和模板: %+v", build)
	}
	if !slices.Equal(build.Targets, []models.BuildTarget{{ServiceName: "api"}, {ServiceName: "worker"}}) || len(plan.Draft.Builds[0].Targets) != 0 {
		t.Fatalf("默认范围应进入确认清单，草稿仍保留自动覆盖: %+v", build)
	}
	for _, invalid := range []models.BuildInput{
		{Name: build.Name, TemplateID: build.TemplateID, Targets: []models.BuildTarget{{ServiceName: "api", ServiceModule: "typo"}}},
		{Name: build.Name, TemplateID: build.TemplateID, Targets: build.Targets, ExcludeTargets: []models.BuildTarget{{ServiceName: "api", ServiceModule: "typo"}}},
	} {
		bad := item
		bad.Params = common.JSON(invalid)
		if _, err := s.submitOperation(ctx, e, &bad); err == nil || !strings.Contains(planError(err, e.Language), "typo") || posts != 0 {
			t.Fatalf("组件名错误时应指出错误且不提交构建: %v", err)
		}
	}
	build.ExcludeTargets = []models.BuildTarget{{ServiceName: "api", ServiceModule: "sidecar"}, {ServiceName: "worker"}}
	deps, missing, err := s.resolveBuildTargets(ctx, e, *plan.Draft, &build, nil, build.Name, true)
	if err != nil || len(missing) != 0 || !slices.Equal(deps, []string{"service-api"}) {
		t.Fatalf("排除的服务不能阻塞构建: %v, %v, %v", deps, missing, err)
	}
	before := workerReads
	excluded, err := s.buildOperationTargets(ctx, e, &item, build)
	if err != nil || workerReads != before || !slices.Equal(excluded, []models.BuildTarget{{ServiceName: "api", ServiceModule: "app"}}) {
		t.Fatalf("应尊重组件排除，并跳过读取整个排除服务: %+v, %v", excluded, err)
	}
	var claimed capturedDraft
	expectClaim(mock, e, plan, "executing", true).WithArgs(&claimed, e.Run.ID)
	if _, err := s.submitOperation(ctx, e, &item); err != nil {
		t.Fatal(err)
	}
	targets, _ := posted["target_repos"].([]any)
	if posts != 1 || len(targets) != 3 {
		t.Fatalf("应以一次请求创建绑定全部实际组件的构建: %+v", posted)
	}
	if saved := claimed.plan.Items[3]; saved.Status != "submitting" || string(saved.Params) != string(item.Params) {
		t.Fatalf("提交前必须保存实际目标用于结果核实: %+v", saved)
	}
	if matched, _, err := matchesOperation(item, common.JSON(posted), e.Run.CreatedAt); err != nil || !matched {
		t.Fatalf("应能核实包含完整目标的构建: %t, %v", matched, err)
	}
	posted["target_repos"] = targets[:1]
	if matched, _, err := matchesOperation(item, common.JSON(posted), e.Run.CreatedAt); err != nil || matched {
		t.Fatalf("缺少部分组件时不能将构建核实为成功: %t, %v", matched, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestBuildDraftSwitchesFromSingleComponentToTargets(t *testing.T) {
	base := onboardingInput(t, `{"builds":[{"name":"backend","service_name":"api","service_module":"app","template_id":"base"}]}`)
	patch := onboardingInput(t, `{"builds":[{"name":"backend","targets":[{"service_name":"api","service_module":"app"},{"service_name":"worker","service_module":"app"}]}]}`)
	draft, err := mergeOnboardingDraft(&base, patch)
	if err != nil {
		t.Fatal(err)
	}
	if build := draft.Builds[0]; build.ServiceName != "" || build.ServiceModule != "" || len(build.Targets) != 2 || build.TemplateID != "base" {
		t.Fatalf("扩展绑定时应保留模板并替换旧单组件字段: %+v", build)
	}
	patch = onboardingInput(t, `{"builds":[{"name":"backend","variables":{"MODE":"test"}}]}`)
	draft, err = mergeOnboardingDraft(&draft, patch)
	if err != nil || len(draft.Builds[0].Targets) != 2 || draft.Builds[0].Variables["MODE"] != "test" {
		t.Fatalf("只改构建变量时应保留组件绑定: %+v, %v", draft.Builds, err)
	}
}
