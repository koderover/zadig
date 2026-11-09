package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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
					case "/api/template/build/general-id":
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
						if body["template_id"] != "general-id" || body["product_name"] != "p" {
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
					args.Builds = append(args.Builds, models.BuildInput{Name: name + "-build", TemplateID: "general-id", ServiceName: name, Repositories: []models.GitConfig{{CodehostID: 1, Owner: "team", Repo: "repo", Branch: "main"}}})
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
					if _, err := s.submitOperation(context.Background(), e, item); err != nil {
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
			templateID := "general-id"
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
