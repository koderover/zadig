package service

import (
	"context"
	"net/http"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
	"github.com/koderover/zadig/v2/pkg/types"
)

func TestEnvironmentAndProjectAuthorizationUsesExistingUserEndpoints(t *testing.T) {
	for _, input := range []struct {
		name, permissions, collaboration string
		page                             models.PageContext
		allowed                          bool
	}{
		{name: "测试环境查看", permissions: `{"project_auth_info":{"p":{"env":{"view":true}}}}`, page: models.PageContext{ProjectKey: "p", EnvName: "dev"}, allowed: true},
		{name: "生产环境查看", permissions: `{"project_auth_info":{"p":{"production_env":{"view":true}}}}`, page: models.PageContext{ProjectKey: "p", EnvName: "prod", Production: true}, allowed: true},
		{name: "测试权限不能查看生产", permissions: `{"project_auth_info":{"p":{"env":{"view":true}}}}`, collaboration: `{"has_permission":false}`, page: models.PageContext{ProjectKey: "p", EnvName: "prod", Production: true}},
		{name: "生产协作查看", permissions: `{"project_auth_info":{"p":{}}}`, collaboration: `{"has_permission":true}`, page: models.PageContext{ProjectKey: "p", EnvName: "prod", Production: true}, allowed: true},
		{name: "仅编辑不能查看", permissions: `{"project_auth_info":{"p":{"env":{"edit_config":true}}}}`, collaboration: `{"has_permission":false}`, page: models.PageContext{ProjectKey: "p", EnvName: "dev"}},
		{name: "项目角色不再补查", permissions: `{"project_auth_info":{"p":{}}}`, page: models.PageContext{ProjectKey: "p"}, allowed: true},
		{name: "全局只读不再补查", permissions: `{"project_auth_info":{"p":{"workflow":{"view":true},"env":{"view":true}}}}`, page: models.PageContext{ProjectKey: "p"}, allowed: true},
		{name: "没有项目权限不再兜底", permissions: `{}`, collaboration: `{"has_permission":true}`, page: models.PageContext{ProjectKey: "p"}},
		{name: "其他项目权限不能放行", permissions: `{"project_auth_info":{"other":{}}}`, page: models.PageContext{ProjectKey: "p"}},
		{name: "空项目权限不能放行", permissions: `{"project_auth_info":{"p":null}}`, page: models.PageContext{ProjectKey: "p"}},
		{name: "系统管理员无需项目条目", permissions: `{"is_system_admin":true}`, page: models.PageContext{ProjectKey: "p"}, allowed: true},
		{name: "管理员", permissions: `{"is_system_admin":true}`, page: models.PageContext{ProjectKey: "p", EnvName: "prod", Production: true}, allowed: true},
	} {
		t.Run(input.name, func(t *testing.T) {
			platform := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Query().Get("uid") != "user" || r.Header.Get("Authorization") != token("user") {
					t.Fatal("必须使用实际用户身份查询权限")
				}
				switch r.URL.Path {
				case "/api/v1/authorization/auth-info":
					w.Write([]byte(input.permissions))
				case "/api/v1/authorization/collaboration-permission":
					action := types.EnvActionView
					if input.page.Production {
						action = types.ProductionEnvActionView
					}
					q := r.URL.Query()
					if q.Get("project_key") != "p" || q.Get("resource") != types.ResourceTypeEnvironment || q.Get("resource_name") != input.page.EnvName || q.Get("action") != action {
						t.Fatalf("协作权限必须区分生产属性及具体环境: %s", r.URL)
					}
					w.Write([]byte(input.collaboration))
				default:
					t.Fatalf("纯权限复查不能读取项目统计或环境配置: %s", r.URL)
				}
			})
			if err := platform.authorizeContext(context.Background(), token("user"), input.page); (err == nil) != input.allowed {
				t.Fatalf("查看权限判断错误: %v", err)
			}
		})
	}
}

func TestModelContextRechecksAllCurrentAndLoadedHistoryResources(t *testing.T) {
	store, mock := mockStore(t)
	e := reportExecution("workflow_diagnosis")
	e.HistoryRunIDs = []string{"past"}
	e.Run.Context = models.PageContext{ProjectKey: "p", WorkflowName: "b", TaskID: 2}
	current := common.JSON([]models.ResourceScope{
		{PageContext: models.PageContext{ProjectKey: "p", WorkflowName: "a", TaskID: 1}},
		{PageContext: e.Run.Context},
	})
	past := common.JSON([]models.ResourceScope{
		{PageContext: models.PageContext{ProjectKey: "p", WorkflowName: "a", TaskID: 9}},
		{PageContext: models.PageContext{ProjectKey: "p", WorkflowName: "c", TaskID: 3}},
	})
	revoked, infoCalls := "", 0
	checks := map[string]int{}
	platform := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/authorization/auth-info":
			infoCalls++
			w.Write([]byte(`{"project_auth_info":{"p":{}}}`))
		case "/api/v1/authorization/collaboration-permission":
			name := r.URL.Query().Get("resource_name")
			checks[name]++
			w.Write(common.JSON(map[string]bool{"has_permission": name != revoked}))
		default:
			t.Fatalf("复查不能读取业务详情: %s", r.URL)
		}
	})
	s := &AgentService{Store: store, Platform: platform}
	for _, name := range []string{"", "a", "c"} {
		revoked, infoCalls, checks = name, 0, map[string]int{}
		mock.ExpectQuery("SELECT id,context,resource_scopes").WithArgs("user", "conversation", e.Cutoff, "run", "past").
			WillReturnRows(sqlmock.NewRows([]string{"id", "context", "scopes"}).AddRow("run", "{}", string(current)).AddRow("past", "{}", string(past)))
		err := s.authorizeModelContext(context.Background(), e)
		if (err == nil) != (name == "") || infoCalls != 1 {
			t.Fatalf("必须逐次复查全部来源，并在同次复用权限快照: revoked=%s err=%v calls=%d", name, err, infoCalls)
		}
		if name == "" && (checks["a"] != 1 || checks["b"] != 1 || checks["c"] != 1) {
			t.Fatalf("相同工作流不同任务应只查一次协作权限: %+v", checks)
		}
	}
}

func TestModelContextRejectsMissingSourceScopes(t *testing.T) {
	store, mock := mockStore(t)
	e := reportExecution("workflow_diagnosis")
	e.HistoryRunIDs = []string{"past"}
	mock.ExpectQuery("SELECT id,context,resource_scopes").WillReturnRows(sqlmock.NewRows([]string{"id", "context", "scopes"}).AddRow("run", "{}", "[]"))
	if err := (&AgentService{Store: store}).authorizeModelContext(context.Background(), e); err == nil {
		t.Fatal("已载入来源的范围缺失后必须停止，不能继续使用旧正文")
	}
}

func TestEnvironmentIdentityRequiresExplicitProduction(t *testing.T) {
	for _, input := range []struct {
		body       string
		production bool
		allowed    bool
	}{
		{`{"product_name":"p","env_name":"dev","production":false,"is_prod":true}`, false, true},
		{`{"product_name":"p","env_name":"dev","production":true,"is_prod":false}`, true, true},
		{`{"product_name":"p","env_name":"dev","production":true}`, false, false},
		{`{"product_name":"p","env_name":"dev","production":false}`, true, false},
		{`{"product_name":"p","env_name":"dev","is_prod":true}`, true, false},
		{`{"product_name":"p","env_name":"dev"}`, false, false},
		{`{"product_name":"other","env_name":"dev","production":false}`, false, false},
	} {
		platform := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/environment/environments/dev" || (r.URL.Query().Get("production") == "true") != input.production {
				t.Fatalf("应通过现有 production 参数读取准确环境: %s", r.URL)
			}
			w.Write([]byte(input.body))
		})
		_, err := platform.environment(context.Background(), token("user"), models.ServiceTarget{ProjectKey: "p", EnvName: "dev", ServiceName: "api", Production: input.production})
		if (err == nil) != input.allowed {
			t.Fatalf("不能混淆环境属性与集群属性或接受缺失值: %s %v", input.body, err)
		}
	}
}
