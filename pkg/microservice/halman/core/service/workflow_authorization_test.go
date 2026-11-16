package service

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
	"github.com/koderover/zadig/v2/pkg/types"
)

func TestWorkflowAuthorizationUsesExistingPermissionsAndRechecksEveryTime(t *testing.T) {
	for _, taskID := range []int64{0, 128} {
		calls := 0
		platform := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
			calls++
			if r.Method != http.MethodGet || r.URL.Path != "/api/v1/authorization/auth-info" || r.URL.Query().Get("uid") != "user" || len(r.URL.Query()) != 1 || r.Header.Get("Authorization") != token("user") {
				t.Fatalf("权限复查只能以实际用户查询已有授权接口: %s", r.URL)
			}
			if calls > 1 {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.Write([]byte(`{"project_auth_info":{"p":{"workflow":{"view":true}}}}`))
		})
		page := models.PageContext{ProjectKey: "p", WorkflowName: "wf", TaskID: taskID}
		if err := platform.authorizeContext(context.Background(), token("user"), page); err != nil {
			t.Fatal(err)
		}
		if err := platform.authorizeContext(context.Background(), token("user"), page); err == nil || calls != 2 {
			t.Fatal("权限撤销后必须拒绝，不能缓存此前的授权")
		}
	}
}

func TestWorkflowLogDoesNotPrefetchTaskDetails(t *testing.T) {
	store, mock := mockStore(t)
	e := reportExecution("workflow_diagnosis")
	platform := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/logs/log/v4/workflow/wf/tasks/128/jobs/job-0-0-0-构建" || !strings.HasSuffix(r.URL.EscapedPath(), url.PathEscape("job-0-0-0-构建")) || r.URL.Query().Get("format") != "excerpt" || r.URL.Query().Get("projectName") != "p" {
			t.Fatalf("日志片段接口已校验权限和归属，不应预读任务详情: %s", r.URL)
		}
		w.Write([]byte(`{"text":"build failed"}`))
	})
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT resource_scopes FROM ai_run").WithArgs("run", "user").WillReturnRows(sqlmock.NewRows([]string{"scopes"}).AddRow("[]"))
	mock.ExpectExec("UPDATE ai_run SET context=.*resource_scopes=").WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), "run", "user").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	s := &AgentService{Store: store, Platform: platform}
	_, err := s.workflowLog(context.WithValue(context.Background(), executionKey{}, e), models.WorkflowLogWindow{
		Target: models.WorkflowTarget{ProjectKey: "p", WorkflowName: "wf", TaskID: 128, JobName: "job-0-0-0-构建"}, Limit: 200,
	})
	if err != nil || !e.EvidenceRead.Load() {
		t.Fatalf("应保留日志证据和权限范围: %v", err)
	}
}

func TestWorkflowJobNameAllowsChineseWithoutRelaxingResourceNames(t *testing.T) {
	for _, name := range []string{"build-api", "job-0-0-0-构建", "构建服务", "job-1-0-0-部署"} {
		if err := checkWorkflowJobName(name); err != nil {
			t.Fatalf("合法步骤名不能被拒绝: %s %v", name, err)
		}
	}
	for _, name := range []string{"", "..", "../logs", "build/api", `build\api`, "build?limit=0", "build#fragment", "build%2Fapi", "build\napi", strings.Repeat("a", 129)} {
		if err := checkWorkflowJobName(name); err == nil {
			t.Fatalf("不安全或缺失的步骤名应拒绝: %q", name)
		}
	}
	if checkNames("中文项目") == nil {
		t.Fatal("步骤名修复不能放宽其他资源标识校验")
	}
}

func TestWorkflowAuthorizationRequiresViewOrCollaborationPermission(t *testing.T) {
	for _, input := range []struct {
		name, permissions, collaboration string
		allowed                          bool
		collaborationStatus, calls       int
	}{
		{name: "系统管理员", permissions: `{"is_system_admin":true}`, allowed: true},
		{name: "项目管理员", permissions: `{"project_auth_info":{"p":{"is_system_admin":true}}}`, allowed: true},
		{name: "其他项目权限", permissions: `{"project_auth_info":{"other":{"workflow":{"view":true}}}}`},
		{name: "无权限", permissions: `{}`},
		{name: "授权响应无效", permissions: `invalid`},
		{name: "仅编辑权限", permissions: `{"project_auth_info":{"p":{"workflow":{"edit":true}}}}`, collaboration: `{"has_permission":false}`, calls: 1},
		{name: "协作查看权限", permissions: `{"project_auth_info":{"p":{}}}`, collaboration: `{"has_permission":true}`, allowed: true, calls: 1},
		{name: "协作响应无效", permissions: `{"project_auth_info":{"p":{}}}`, collaboration: `{}`, calls: 1},
		{name: "协作错误不能放行", permissions: `{"project_auth_info":{"p":{}}}`, collaboration: `{"has_permission":true,"error":"failed"}`, calls: 1},
		{name: "协作查询失败", permissions: `{"project_auth_info":{"p":{}}}`, collaborationStatus: http.StatusInternalServerError, calls: 1},
	} {
		t.Run(input.name, func(t *testing.T) {
			calls := 0
			platform := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != token("user") || r.URL.Query().Get("uid") != "user" {
					t.Fatal("必须查询实际用户的权限")
				}
				switch r.URL.Path {
				case "/api/v1/authorization/auth-info":
					w.Write([]byte(input.permissions))
				case "/api/v1/authorization/collaboration-permission":
					calls++
					q := r.URL.Query()
					if q.Get("project_key") != "p" || q.Get("resource") != types.ResourceTypeWorkflow || q.Get("resource_name") != "wf" || q.Get("action") != types.WorkflowActionView {
						t.Fatalf("必须检查指定工作流的协作查看授权: %s", r.URL)
					}
					if input.collaborationStatus != 0 {
						w.WriteHeader(input.collaborationStatus)
					}
					w.Write([]byte(input.collaboration))
				default:
					t.Fatalf("权限复查不应读取任务或工作流正文: %s", r.URL)
				}
			})
			err := platform.authorizeContext(context.Background(), token("user"), models.PageContext{ProjectKey: "p", WorkflowName: "wf", TaskID: 128})
			if (err == nil) != input.allowed || calls != input.calls {
				t.Fatalf("授权结果不符: err=%v, calls=%d", err, calls)
			}
		})
	}
}
