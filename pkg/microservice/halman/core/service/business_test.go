package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	authv3 "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	"github.com/go-resty/resty/v2"
	configbase "github.com/koderover/zadig/v2/pkg/config"
	userclient "github.com/koderover/zadig/v2/pkg/shared/client/user"
	"github.com/koderover/zadig/v2/pkg/tool/httpclient"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	rpcstatus "google.golang.org/genproto/googleapis/rpc/status"
	"net/url"
	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/session"

	halmanconfig "github.com/koderover/zadig/v2/pkg/microservice/halman/config"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
	ormrepo "github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/orm"
	"github.com/koderover/zadig/v2/pkg/types"
)

func mockStore(t *testing.T) (*ormrepo.Store, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Error(err)
		}

		db.Close()
	})
	store, err := ormrepo.NewStore(db, halmanconfig.DefaultDBMaxConnections)
	if err != nil {
		t.Fatal(err)
	}

	return store, mock
}

func runRows(id string, now time.Time) *sqlmock.Rows {
	return sqlmock.NewRows(strings.Split(
		"id, request_id, conversation_id, status, context, created_at, finished_at, error, input_tokens, "+
			"output_tokens, skill_id, skill_execution_id, skill_status, plan, plan_version, plan_status, "+
			"plan_expires_at, answer_version, question_source_run_id, answer_available",
		", ",
	)).AddRow(id, "req-1", "conversation-1", "running", "{}", now, nil, "", 0, 0, "", nil, nil, nil, 0, nil, nil, 1, nil, false)
}

func TestStartDuplicateAtCapacityAndAfterModelDeletion(t *testing.T) {
	s, mock := mockStore(t)
	now := time.Now().UTC()
	req := models.RunRequest{RequestID: "req-1", Message: "你好"}
	mock.ExpectQuery("SELECT r.id,r.request_hash,r.created_at").WithArgs("user-1", "req-1", "user-1").WillReturnRows(sqlmock.NewRows([]string{"id", "request_hash", "created_at"}).
		AddRow("run-1", common.RequestFingerprint("conversation-1", req), now))
	mock.ExpectQuery("SELECT r.id,.* JOIN ai_conversation").WithArgs("run-1", "user-1", "user-1", sqlmock.AnyArg()).
		WillReturnRows(runRows("run-1", now))
	a := &AgentService{Store: s, maxConcurrentRuns: 1, active: map[string]*activeRun{"occupied": {}}}
	r, err := a.Start(
		context.Background(),
		"user-1",
		"conversation-1",
		"",
		req,
		&types.AIAssistantRuntime{
			AIAssistantConfig: types.AIAssistantConfig{HistoryRetentionDays: 90},
			ModelError:        "模型已被删除",
		},
	)
	if err != nil || r.ID != "run-1" {
		t.Fatalf("容量或模型变化不应破坏重复请求查询: %+v %v", r, err)
	}
}

func TestSessionContextExcludesExpiredAndUnauthorizedHistory(t *testing.T) {
	s, mock := mockStore(t)
	now := time.Now().UTC()
	cutoff := now.Add(-24 * time.Hour)
	exec := &execution{
		Run:           &models.Run{ID: "current", ConversationID: "conversation-1"},
		User:          "user-1",
		Authorization: token("user-1"),
		Cutoff:        cutoff,
	}
	state := session.NewSession(halmanconfig.AppName, "user-1", "conversation-1")
	for _, entry := range []struct {
		id string
		at time.Time
	}{
		{"plain", now},
		{"scoped", now},
		{"old", cutoff.Add(-time.Hour)},
	} {
		state.Events = append(state.Events, historyEvent(entry.id, "user", "历史问题", entry.at))
	}

	mock.ExpectQuery("SELECT id,context,resource_scopes FROM ai_run .* id IN").WithArgs("user-1", "conversation-1", cutoff, "plain", "scoped", "old").
		WillReturnRows(sqlmock.NewRows([]string{"id", "context", "resource_scopes"}).
			AddRow("plain", "{}", "[]").
			AddRow("scoped", `{"project_key":"secret-project"}`, `[{"project_key":"secret-project"}]`).
			AddRow("old", "{}", "[]"))
	mock.ExpectQuery("SELECT r.id,COALESCE").WithArgs(cutoff, cutoff, "user-1", "conversation-1", cutoff, "plain", "scoped", "old").
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"origin",
			"version",
			"regenerated",
			"question_available",
			"latest",
		}).
			AddRow("plain", "plain", 1, false, true, "plain").
			AddRow("scoped", "scoped", 1, false, true, "scoped").
			AddRow("old", "old", 1, false, false, "old"))
	platform := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/authorization/auth-info" {
			t.Fatalf("历史资源应通过当前权限过滤: %s", r.URL)
		}
		w.Write([]byte(`{}`))
	})
	a := &AgentService{Store: s, Platform: platform}
	filtered, err := a.sessionContext(&session.GetSessionContext{Context: context.WithValue(context.Background(), executionKey{}, exec)}, func() (*session.Session, error) { return state, nil })
	if err != nil || len(filtered.Events) != 1 || filtered.Events[0].RequestID != "plain" {
		t.Fatalf("历史上下文过滤失败: %+v %v", filtered, err)
	}
}

func TestAppendFailureIsVisibleToExecution(t *testing.T) {
	exec := &execution{Run: &models.Run{ID: "run-1"}}
	h := &session.AppendEventContext{
		Context: context.WithValue(context.Background(), executionKey{}, exec),
		Event:   event.NewResponseEvent("inv", "assistant", &model.Response{Choices: []model.Choice{{Message: model.NewAssistantMessage("回答")}}}),
	}
	want := errors.New("存储失败")
	err := (&AgentService{}).beforePersist(h, func() error { return want })
	if !errors.Is(err, want) || !exec.PersistenceFailed.Load() || h.Event.RequestID != "run-1" {
		t.Fatalf("持久化失败不能被显示为成功: %v", err)
	}
}

func testPlatform(t *testing.T, handler http.HandlerFunc) *PlatformClient {
	t.Helper()
	log.Init(&log.Config{Level: "error"})
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, r)
		return rec.Result(), nil
	})}
	users := &userclient.Client{Client: httpclient.New(httpclient.SetHostURL(configbase.UserServiceAddress() + "/api/v1"))}
	users.SetTransport(client.Transport)
	users.SetRedirectPolicy(resty.NoRedirectPolicy())
	return &PlatformClient{HTTP: client, Users: users, Auth: fakeAuth{CheckFunc: func(*authv3.CheckRequest) (*authv3.CheckResponse, error) {
		return &authv3.CheckResponse{Status: &rpcstatus.Status{}, HttpResponse: &authv3.CheckResponse_OkResponse{OkResponse: &authv3.OkHttpResponse{}}}, nil
	}}}
}

func TestBusinessUsesUserIdentityAndNeverRetriesWrite(t *testing.T) {
	calls := 0
	p := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "Bearer current-user" || r.Header.Get("X-Internal-Service-Token") != "" {
			t.Error("业务请求不得使用服务凭据")
		}

		w.WriteHeader(http.StatusGatewayTimeout)
	})
	_, err := p.business(context.Background(), "Bearer current-user", http.MethodPost, "/api/workflow/v4/auto", nil, nil)
	var platform *PlatformError
	if !errors.As(err, &platform) || !platform.Uncertain || calls != 1 {
		t.Fatalf("写超时不能重试或报告成功: %v, calls=%d", err, calls)
	}
}

func TestReadOnlyUserCannotConfirmResourceWrites(t *testing.T) {
	for _, action := range []string{"create_project", "create_service", "create_build", "create_environment", "create_workflow", "restart_service", "rollback_environment"} {
		t.Run(action, func(t *testing.T) {
			p := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.Header.Get("Authorization") != "Bearer user" {
					t.Fatal("拒绝无权操作前不能提交业务写请求")
				}
				switch r.URL.Path {
				case "/api/v1/authorization/auth-info":
					if r.URL.Query().Get("uid") != "user" {
						t.Fatal("必须检查当前用户的权限")
					}
					w.Write([]byte(`{"project_auth_info":{"demo":{"env":{"View":true},"workflow":{"View":true}}}}`))
				case "/api/v1/authorization/collaboration-action", "/api/v1/authorization/collaboration-permission":
					w.Write([]byte(`{"has_permission":false}`))
				default:
					t.Fatalf("没有权限时不能请求业务资源: %s", r.URL)
				}
			})
			err := (&AgentService{Platform: p}).preflightPermissions(context.Background(),
				&execution{User: "user", Authorization: "Bearer user"},
				models.OperationPlan{Items: []models.OperationItem{{Action: action, ProjectKey: "demo", Params: common.JSON(models.RepairInput{
					Target: models.ServiceTarget{ProjectKey: "demo", EnvName: "dev", ServiceName: "api"},
				})}}})
			var denied *PlatformError
			if !errors.As(err, &denied) || denied.Status != http.StatusForbidden {
				t.Fatalf("只读权限不能执行 %s，且须返回 403: %v", action, err)
			}
		})
	}
}

func TestWorkflowAndEnvironmentRejectMismatchedTargets(t *testing.T) {
	p := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "workflowtask") {
			w.Write([]byte(`{"project_key":"other","workflow_key":"build","task_id":8}`))
		} else {
			w.Write([]byte(`{"product_name":"demo","env_name":"dev","production":true}`))
		}
	})
	if _, err := p.workflowTask(context.Background(), "Bearer user", models.WorkflowTarget{ProjectKey: "demo", WorkflowName: "build", TaskID: 8}); err == nil {
		t.Fatal("不能信任与项目不匹配的任务")
	}

	if _, err := p.environment(context.Background(), "Bearer user", models.ServiceTarget{ProjectKey: "demo", EnvName: "dev", ServiceName: "api"}); err == nil {
		t.Fatal("不能以非生产属性读取生产目标")
	}
}

func TestCatalogAndDiagnosticExcludeCredentialsAndRawConfiguration(t *testing.T) {
	raw := json.RawMessage(`{"content":"data: hidden-secret","rendered_yaml":"password: hidden-password","scripts":"echo hidden-script","variables":[{"key":"ACCESS_KEY","value":"hidden-key"}],"url":"https://u:hidden-url@example.com","name":"可见名称"}`)
	for _, sanitize := range []func(json.RawMessage) (any, error){safeCatalogJSON, safeDiagnosticJSON} {
		value, err := sanitize(raw)
		if err != nil {
			t.Fatal(err)
		}

		encoded := string(common.JSON(value))
		if strings.Contains(encoded, "hidden-") {
			t.Fatalf("敏感数据不能进入模型: %s", encoded)
		}

		if !strings.Contains(encoded, "可见名称") {
			t.Fatal("应保留必要元数据")
		}
	}
}

func TestGitConfigurationUsesFilesAPIAndChecksChartName(t *testing.T) {
	fileReads := 0
	p := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/code/codehost" {
			w.Write([]byte(`[{"id":1,"type":"gitlab"}]`))
			return
		}

		if r.URL.Path != "/api/code/workspace/getcontents/1" || r.URL.Query().Get("path") != "charts/api/Chart.yaml" ||
			r.URL.Query().
				Get("repoOwner") != "group/sub" {
			t.Errorf("应按 API 读取明确文件: %s", r.URL)
		}

		fileReads++
		json.NewEncoder(w).Encode("apiVersion: v2\nname: wrong-name\nversion: 1.0.0\n")
	})
	item := models.ServiceInput{
		Name:   "api",
		Source: "helm_git",
		Git: &models.GitConfig{
			CodehostID: 1,
			Owner:      "group",
			Namespace:  "group/sub",
			Repo:       "deploy",
			Branch:     "main",
			Path:       "charts/api",
		},
	}
	if err := p.checkConfiguration(context.Background(), "Bearer user", item); err == nil || fileReads != 1 {
		t.Fatalf("Chart 名称不符应在写之前失败: %v", err)
	}

	for _, path := range []string{
		"../deploy",
		"/etc/passwd",
		"a/../b",
		"a\\b",
	} {
		if checkConfigPath(path) == nil {
			t.Fatalf("非法配置路径被接受: %s", path)
		}
	}
}

func TestNonAPIGitSourcesAreRejected(t *testing.T) {
	p := testPlatform(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`[{"id":1,"type":"other"}]`)) })
	if err := p.requireGitSource(context.Background(), "Bearer user", 1); err == nil {
		t.Fatal("不能经现有导入接口绕过禁止 clone 的限制")
	}
}

func expectClaim(mock sqlmock.Sqlmock, e *execution, plan models.OperationPlan, status string, commit bool) {
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM ai_conversation .* FOR UPDATE").WithArgs(e.Run.ConversationID, e.User).WillReturnRows(sqlmock.NewRows([]string{"id"}).
		AddRow(e.Run.ConversationID))
	mock.ExpectQuery("SELECT active_plan_run_id .* FOR UPDATE").WithArgs(e.RootID).WillReturnRows(sqlmock.NewRows([]string{"active"}).
		AddRow(e.Run.ID))
	mock.ExpectQuery("SELECT plan,status .* FOR UPDATE").WithArgs(e.Run.ID).WillReturnRows(sqlmock.NewRows([]string{"plan", "status"}).
		AddRow([]byte(common.JSON(plan)), status))
	if commit {
		mock.ExpectExec("UPDATE ai_run SET plan").WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()
	} else {
		mock.ExpectRollback()
	}
}

func TestCancelledPlanCannotSubmitAndClaimedWriteCannotReplay(t *testing.T) {
	s, mock := mockStore(t)
	posts := 0
	p := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		posts++
		if r.Method != http.MethodPost || r.URL.Path != "/api/workflow/v4/auto" || r.URL.Query().Get("projectName") != "demo" {
			t.Errorf("工作流必须直接复用 AutoCreateWorkflow: %s %s", r.Method, r.URL)
		}

		w.Write([]byte(`{}`))
	})
	a := &AgentService{Store: s, Platform: p}
	e := &execution{
		Run:           &models.Run{ID: "run", ConversationID: "conversation"},
		RootID:        "root",
		User:          "user",
		Authorization: "Bearer user",
	}
	item := models.OperationItem{
		ID:         "workflow",
		Action:     "create_workflow",
		ProjectKey: "demo",
		Status:     "pending",
	}
	plan := models.OperationPlan{Items: []models.OperationItem{item}}
	expectClaim(mock, e, plan, "cancelling", false)
	if _, err := a.submitOperation(context.Background(), e, item); !errors.Is(err, context.Canceled) || posts != 0 {
		t.Fatalf("取消后不能提交: %v, posts=%d", err, posts)
	}

	expectClaim(mock, e, plan, "executing", true)
	if _, err := a.submitOperation(context.Background(), e, item); err != nil || posts != 1 {
		t.Fatalf("首次提交失败: %v", err)
	}

	plan.Items[0].Status = "submitting"
	expectClaim(mock, e, plan, "executing", false)
	if _, err := a.submitOperation(context.Background(), e, item); !errors.Is(err, common.ErrRequestConflict) || posts != 1 {
		t.Fatalf("提交中的动作不能重复写: %v, posts=%d", err, posts)
	}
}

func TestPlanPreservesTemplateVersionsAndSeparatesProjectParameters(t *testing.T) {
	args := models.OnboardingInput{
		ProjectKey:  "demo",
		ProjectName: "示例",
		ProjectType: "k8s",
		Services:    []models.ServiceInput{{Name: "api", TemplateID: "template", TemplateFingerprint: "fixed-version"}},
		Builds:      []models.BuildInput{{TemplateFingerprint: "build-version"}},
	}
	raw := common.JSON(args)
	var restored models.OnboardingInput
	if json.Unmarshal(raw, &restored) != nil || restored.Services[0].TemplateFingerprint != "fixed-version" ||
		restored.Builds[0].TemplateFingerprint != "build-version" {
		t.Fatal("确认之后仍需要校验模板版本")
	}

	project := string(common.JSON(projectInput(args)))
	if strings.Contains(project, "services") || strings.Contains(project, "builds") || !strings.Contains(project, `"deploy_type":"k8s"`) {
		t.Fatalf("项目动作只保存项目参数: %s", project)
	}

	item := models.OperationItem{DependsOn: []string{"api"}}
	for _, status := range []string{
		"failed",
		"unknown",
		"cancelled",
		"blocked",
	} {
		if dependenciesReady(item, map[string]string{"api": status}) {
			t.Fatalf("前置 %s 不能继续写", status)
		}
	}

	if !dependenciesReady(item, map[string]string{"api": "succeeded"}) {
		t.Fatal("独立成功的前置应允许继续")
	}
}

func TestTemplateVariablesKeepTypesAndRejectUnknownOverrides(t *testing.T) {
	raw := []byte(`{"service_variable_kvs":[{"key":"enabled","type":"bool","value":false},{"key":"tier","type":"enum","options":["small","large"]}]}`)
	_, values, err := templateVariables(raw, map[string]string{"enabled": "true", "tier": "small"})
	if err != nil || values["enabled"] != true {
		t.Fatalf("布尔变量不能被转成字符串: %v %v", values, err)
	}

	for _, input := range []map[string]string{{"enabled": "yes"}, {"tier": "invalid"}, {"template-name": "cannot-inject"}} {
		if checkTemplateVariables(raw, input) == nil {
			t.Fatalf("不应接受不符合模板定义的覆盖: %v", input)
		}
	}
}

func TestBusinessCallsInternalServicesWithSeparatedCredentials(t *testing.T) {
	calls := 0
	p := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "Bearer actual-user" || r.Header.Get("X-Request-ID") != "run-id" {
			t.Fatal("必须透传实际用户身份和运行 ID")
		}
		if strings.HasPrefix(r.URL.Path, "/api/v1/authorization/") {
			if r.URL.Host != "user:80" || r.Header.Get("X-Internal-Service-Token") != "permission-query-only" || r.URL.Query().Get("uid") != "user" {
				t.Fatalf("权限查询必须直连 user 客户端: %s", r.URL)
			}
			deadline, ok := r.Context().Deadline()
			if !ok || time.Until(deadline) > 10*time.Second {
				t.Fatal("权限查询必须保留超时")
			}
		} else {
			if r.URL.Host != "aslan:25000" || r.Header.Get("X-Internal-Service-Token") != "" || r.URL.Path != "/api/workflow/v4/auto" {
				t.Fatalf("资源操作只能携带用户身份直连 aslan 原生路径: %s", r.URL)
			}
		}
		w.Write([]byte(`{}`))
	})
	p.Users.SetHeader("X-Internal-Service-Token", "permission-query-only")
	ctx := context.WithValue(context.Background(), executionKey{}, &execution{Run: &models.Run{ID: "run-id"}})
	for _, path := range []string{"/api/v1/authorization/auth-info", "/api/v1/authorization/collaboration-action", "/api/v1/authorization/collaboration-permission"} {
		if _, err := p.business(ctx, "Bearer actual-user", http.MethodGet, path, url.Values{"uid": {"user"}}, nil); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := p.business(ctx, "Bearer actual-user", http.MethodPost, "/api/workflow/v4/auto", nil, map[string]string{"project": "demo"}); err != nil {
		t.Fatal(err)
	}
	before := calls
	for _, path := range []string{"/api/v1/authorization/auth-info", "/api/v1/authorization/unknown", "https://other.example/api"} {
		if _, err := p.business(ctx, "Bearer actual-user", http.MethodPost, path, nil, nil); !errors.Is(err, common.ErrInvalidRequest) {
			t.Fatalf("不得放开 user 写入或任意地址: %s %v", path, err)
		}
	}
	if calls != before {
		t.Fatal("无效适配器路径不得发送请求")
	}
}

func TestBusinessRevalidatesTokenBeforeEachOperation(t *testing.T) {
	calls, checks := 0, 0
	p := testPlatform(t, func(w http.ResponseWriter, r *http.Request) { calls++; w.Write([]byte(`{}`)) })
	p.Auth = fakeAuth{CheckFunc: func(r *authv3.CheckRequest) (*authv3.CheckResponse, error) {
		checks++
		if r.GetAttributes().GetRequest().GetHttp().GetHeaders()["authorization"] != "Bearer actual-user" {
			t.Fatal("必须向 user 校验原始用户 Token")
		}
		if checks > 1 {
			return &authv3.CheckResponse{Status: &rpcstatus.Status{Code: 7}}, nil
		}
		return &authv3.CheckResponse{Status: &rpcstatus.Status{}, HttpResponse: &authv3.CheckResponse_OkResponse{OkResponse: &authv3.OkHttpResponse{}}}, nil
	}}
	if _, err := p.business(context.Background(), "Bearer actual-user", http.MethodGet, "/api/workflow/v4", nil, nil); err != nil {
		t.Fatal(err)
	}
	_, err := p.business(context.Background(), "Bearer actual-user", http.MethodPost, "/api/workflow/v4/auto", nil, nil)
	var failure *PlatformError
	if !errors.As(err, &failure) || failure.Status != 401 || failure.Uncertain || calls != 1 || checks != 2 {
		t.Fatalf("Token 撤销或 MFA 拒绝后不能发送写请求: %v calls=%d checks=%d", err, calls, checks)
	}
	p.Auth = nil
	if _, err := p.business(context.Background(), "Bearer actual-user", http.MethodGet, "/api/v1/authorization/auth-info", nil, nil); err == nil || calls != 1 {
		t.Fatal("未配置用户鉴权不能查询内部权限")
	}
}

func TestUserPermissionReadsDoNotRedirectOrExceedBodyLimit(t *testing.T) {
	for _, redirect := range []bool{true, false} {
		calls := 0
		p := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
			calls++
			if r.URL.Host != "user:80" {
				t.Fatal("不得向其他服务发送内部权限凭据")
			}
			if redirect {
				w.Header().Set("Location", "http://other.example/permissions")
				w.WriteHeader(http.StatusFound)
			} else {
				w.Write([]byte(strings.Repeat(" ", (2<<20)+1)))
			}
		})
		if _, err := p.business(context.Background(), "Bearer actual-user", http.MethodGet, "/api/v1/authorization/auth-info", nil, nil); err == nil || calls != 1 {
			t.Fatalf("权限查询不得跟随跳转、重试或读取过大响应: redirect=%v err=%v calls=%d", redirect, err, calls)
		}
	}
}
