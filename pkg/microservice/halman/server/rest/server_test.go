package rest

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	authv3 "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	rpcstatus "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc"

	halmanconfig "github.com/koderover/zadig/v2/pkg/microservice/halman/config"
	ormrepo "github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/orm"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/service"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
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

type fakeAuth struct {
	CheckFunc func(*authv3.CheckRequest) (*authv3.CheckResponse, error)
}

func (f fakeAuth) Check(_ context.Context, r *authv3.CheckRequest, _ ...grpc.CallOption) (*authv3.CheckResponse, error) {
	return f.CheckFunc(r)
}

func token(uid string) string {
	return "Bearer header." + base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"uid":%q}`, uid))) + ".signature"
}

func assertAssistantErrorCode(t *testing.T, w *httptest.ResponseRecorder, code int) {
	t.Helper()
	var body e.ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != code || body.Type != "error" || body.Message == "" || body.Extra == nil {
		t.Fatalf("错误码或结构不符合已有约定: %s", w.Body)
	}
}

func TestHTTPModelPreferences(t *testing.T) {
	store, mock := mockStore(t)
	cfg := types.AIAssistantConfig{Enabled: true, HistoryRetentionDays: 90, LLMIntegrationID: "default", LLMIntegrationIDs: []string{"alternate"}}
	platform := &service.PlatformClient{
		CheckAILicense: func() error { return nil },
		Auth: fakeAuth{CheckFunc: func(*authv3.CheckRequest) (*authv3.CheckResponse, error) {
			return &authv3.CheckResponse{Status: &rpcstatus.Status{}, HttpResponse: &authv3.CheckResponse_OkResponse{OkResponse: &authv3.OkHttpResponse{}}}, nil
		}},
		LoadRuntime: func(context.Context) (*types.AIAssistantRuntime, error) {
			return &types.AIAssistantRuntime{AIAssistantConfig: cfg}, nil
		},
		ListModels: func(_ context.Context, ids []string) ([]types.AIAssistantModelOption, error) {
			var result []types.AIAssistantModelOption
			for _, id := range ids {
				result = append(result, types.AIAssistantModelOption{ID: id, ReasoningEfforts: []string{"high"}})
			}
			return result, nil
		},
	}
	engine := NewEngine(&service.AgentService{Store: store, Platform: platform})
	request := func(method, path, user, body string, status int) string {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Authorization", token(user))
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		if w.Code != status {
			t.Fatalf("%s %s 返回 %d，预期 %d: %s", method, path, w.Code, status, w.Body)
		}
		return w.Body.String()
	}
	const path = "/api/halman/preferences"
	const saved = `{"llm_integration_id":"alternate","reasoning_effort":"high"}`
	request(http.MethodPut, path, "alice", saved, http.StatusForbidden)
	request(http.MethodPut, path, "alice", `{}`, http.StatusForbidden)
	if body := request(http.MethodGet, "/api/halman/models", "alice", "", http.StatusOK); strings.Contains(body, "alternate") || !strings.Contains(body, `"reasoning_efforts":[]`) {
		t.Fatalf("关闭时应只返回默认模型和空强度选项: %s", body)
	}

	cfg.AllowUserModelSelection = true
	for _, invalid := range []string{`null`, `{"user_id":"bob"}`, `{"reasoning_effort":"high"}`} {
		request(http.MethodPut, path, "alice", invalid, http.StatusBadRequest)
	}
	mock.ExpectExec("INSERT INTO ai_user_preference").WithArgs("alice", "alternate", "high", "alternate", "high").WillReturnResult(sqlmock.NewResult(0, 1))
	request(http.MethodPut, path, "alice", saved, http.StatusOK)
	mock.ExpectQuery("SELECT model_id,reasoning_effort").WithArgs("bob").WillReturnRows(sqlmock.NewRows([]string{"model_id", "reasoning_effort"}))
	if body := request(http.MethodGet, path, "bob", "", http.StatusOK); body != "{}" {
		t.Fatalf("不能读取其他用户的偏好: %s", body)
	}
	cfg.AllowUserModelSelection = false
	mock.ExpectQuery("SELECT model_id,reasoning_effort").WithArgs("alice").WillReturnRows(sqlmock.NewRows([]string{"model_id", "reasoning_effort"}).AddRow("alternate", "high"))
	if body := request(http.MethodGet, path, "alice", "", http.StatusOK); body != saved {
		t.Fatalf("关闭后应保留本人偏好: %s", body)
	}
	cfg.AllowUserModelSelection = true
	mock.ExpectExec("INSERT INTO ai_user_preference").WithArgs("alice", "default", "", "default", "").WillReturnResult(sqlmock.NewResult(0, 1))
	request(http.MethodPut, path, "alice", `{"llm_integration_id":"default"}`, http.StatusOK)
	mock.ExpectExec("DELETE FROM ai_user_preference").WithArgs("alice").WillReturnResult(sqlmock.NewResult(0, 1))
	request(http.MethodPut, path, "alice", `{}`, http.StatusOK)
	for _, body := range []string{`{"request_id":"new","llm_integration_id":null}`, `{"request_id":"new","reasoning_effort":""}`} {
		if response := request(http.MethodPost, "/api/halman/runs/run/regenerate", "alice", body, http.StatusBadRequest); !strings.Contains(response, path) {
			t.Fatalf("重答应提示通过偏好接口修改: %s", response)
		}
	}
}

func TestHTTPDoesNotAcceptBodyUserIDAsIdentity(t *testing.T) {
	s, mock := mockStore(t)
	platform := &service.PlatformClient{

		CheckAILicense: func() error { return nil },
		Auth: fakeAuth{CheckFunc: func(*authv3.CheckRequest) (*authv3.CheckResponse, error) {
			return &authv3.CheckResponse{
				Status:       &rpcstatus.Status{},
				HttpResponse: &authv3.CheckResponse_OkResponse{OkResponse: &authv3.OkHttpResponse{}},
			}, nil
		}},
		LoadRuntime: func(context.Context) (*types.AIAssistantRuntime, error) {
			return &types.AIAssistantRuntime{AIAssistantConfig: types.AIAssistantConfig{Enabled: true, HistoryRetentionDays: 90}, Token: "model-secret"}, nil
		},
	}
	mock.ExpectExec("INSERT INTO `ai_conversation`").WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), "新会话", "actual-user").
		WillReturnResult(sqlmock.NewResult(0, 1))
	r := NewEngine(&service.AgentService{Store: s, Platform: platform})
	req := httptest.NewRequest(http.MethodPost, "/api/halman/conversations", strings.NewReader(`{"user_id":"other-user"}`))
	req.Header.Set("Authorization", token("actual-user"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated || strings.Contains(w.Body.String(), "model-secret") {
		t.Fatalf("会话创建或凭据隔离失败: %d %s", w.Code, w.Body.String())
	}
}

func TestHTTPRejectsAnonymousBeforeDatabaseAccess(t *testing.T) {
	r := NewEngine(&service.AgentService{Platform: &service.PlatformClient{}})
	for _, route := range r.Routes() {
		if route.Path == "/healthz" || route.Path == "/readyz" {
			continue
		}
		t.Run(route.Method+" "+route.Path, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(route.Method, route.Path, nil))
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("所有 AI 业务路由均须在访问数据库或模型前拒绝匿名请求，实际 %d", w.Code)
			}
			assertAssistantErrorCode(t, w, e.ErrUnauthorized.Code())
		})
	}
}

func TestAllAIRoutesRequireAILicenseAfterUserAuthentication(t *testing.T) {
	for _, input := range []struct {
		name string
		err  error
		want int
	}{
		{"现有函数返回许可证无效", e.NewHTTPError(e.ErrLicenseInvalid.Code(), "许可证无效"), http.StatusBadRequest},
		{"现有函数返回查询失败", errors.New("查询许可证失败"), http.StatusInternalServerError},
	} {
		t.Run(input.name, func(t *testing.T) {
			authCalls, licenseCalls := 0, 0
			p := &service.PlatformClient{

				Auth: fakeAuth{CheckFunc: func(r *authv3.CheckRequest) (*authv3.CheckResponse, error) {
					authCalls++
					if r.GetAttributes().GetRequest().GetHttp().GetHeaders()["authorization"] != token("user") {
						t.Fatal("每个 AI 入口都应请求 user 服务校验实际登录态")
					}
					return &authv3.CheckResponse{Status: &rpcstatus.Status{}, HttpResponse: &authv3.CheckResponse_OkResponse{OkResponse: &authv3.OkHttpResponse{}}}, nil
				}},
				CheckAILicense: func() error {
					licenseCalls++
					if authCalls != licenseCalls {
						t.Fatal("必须先请求 user 鉴权，许可证未通过时不能读取配置或处理业务")
					}
					return input.err
				},
			}
			r := NewEngine(&service.AgentService{Platform: p})
			count := 0
			for _, route := range r.Routes() {
				if route.Path == "/healthz" || route.Path == "/readyz" {
					continue
				}
				count++
				req := httptest.NewRequest(route.Method, route.Path, nil)
				req.Header.Set("Authorization", token("user"))
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
				if w.Code != input.want {
					t.Fatalf("%s %s 未统一拦截: %d", route.Method, route.Path, w.Code)
				}
				code := e.ErrInternalError.Code()
				if input.want == http.StatusBadRequest {
					code = e.ErrLicenseInvalid.Code()
				}
				assertAssistantErrorCode(t, w, code)
			}
			if count == 0 || authCalls != count || licenseCalls != count {
				t.Fatalf("存在漏检入口: routes=%d auth=%d license=%d", count, authCalls, licenseCalls)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))
			if w.Code != http.StatusOK || authCalls != count || licenseCalls != count {
				t.Fatal("健康检查不应受用户登录或AI 许可证影响")
			}
		})
	}
}

func TestUserRejectionStopsBeforeLicenseAndBusiness(t *testing.T) {
	for _, input := range []struct {
		name string
		resp *authv3.CheckResponse
		err  error
	}{
		{name: "user 拒绝", resp: &authv3.CheckResponse{Status: &rpcstatus.Status{Code: 7}}},
		{name: "user 不可用", err: errors.New("unavailable")},
		{name: "user 响应缺少状态", resp: &authv3.CheckResponse{HttpResponse: &authv3.CheckResponse_OkResponse{OkResponse: &authv3.OkHttpResponse{}}}},
	} {
		t.Run(input.name, func(t *testing.T) {
			calls := 0
			p := &service.PlatformClient{Auth: fakeAuth{CheckFunc: func(*authv3.CheckRequest) (*authv3.CheckResponse, error) {
				calls++
				return input.resp, input.err
			}}}
			r := NewEngine(&service.AgentService{Platform: p})
			req := httptest.NewRequest(http.MethodPost, "/api/halman/conversations", nil)
			req.Header.Set("Authorization", token("user"))
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusUnauthorized || calls != 1 {
				t.Fatalf("未得到 user 允许时不能查询许可证或处理业务: %d calls=%d", w.Code, calls)
			}
		})
	}
}

func TestMissingLicenseCheckerFailsClosed(t *testing.T) {
	p := &service.PlatformClient{Auth: fakeAuth{CheckFunc: func(*authv3.CheckRequest) (*authv3.CheckResponse, error) {
		return &authv3.CheckResponse{Status: &rpcstatus.Status{}, HttpResponse: &authv3.CheckResponse_OkResponse{OkResponse: &authv3.OkHttpResponse{}}}, nil
	}}}
	r := NewEngine(&service.AgentService{Platform: p})
	req := httptest.NewRequest(http.MethodPost, "/api/halman/conversations", nil)
	req.Header.Set("Authorization", token("user"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("未配置许可证检查时不能放行到模型配置或数据库，实际 %d", w.Code)
	}
}

func TestDisabledAssistantBlocksNewWorkButKeepsHistoryAndCancellation(t *testing.T) {
	store, mock := mockStore(t)
	p := &service.PlatformClient{
		CheckAILicense: func() error { return nil },
		Auth: fakeAuth{CheckFunc: func(*authv3.CheckRequest) (*authv3.CheckResponse, error) {
			return &authv3.CheckResponse{Status: &rpcstatus.Status{}, HttpResponse: &authv3.CheckResponse_OkResponse{OkResponse: &authv3.OkHttpResponse{}}}, nil
		}},
		LoadRuntime: func(context.Context) (*types.AIAssistantRuntime, error) {
			return &types.AIAssistantRuntime{AIAssistantConfig: types.AIAssistantConfig{HistoryRetentionDays: 90}}, nil
		},
	}
	r := NewEngine(&service.AgentService{Store: store, Platform: p})
	for _, input := range []struct{ method, path, body string }{
		{http.MethodPost, "/api/halman/conversations", ""},
		{http.MethodPost, "/api/halman/conversations/c/runs", `{"threadId":"c","runId":"new","messages":[{"id":"m","role":"user","content":"你好"}]}`},
		{http.MethodPost, "/api/halman/runs/r/regenerate", `{"request_id":"new"}`},
		{http.MethodPost, "/api/halman/runs/r/retry", `{"request_id":"new"}`},
		{http.MethodPost, "/api/halman/runs/r/confirmation", `{"version":1}`},
		{http.MethodGet, "/api/halman/recommendations", ""},
	} {
		req := httptest.NewRequest(input.method, input.path, strings.NewReader(input.body))
		req.Header.Set("Authorization", token("user"))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("关闭后不能开始 %s: %d %s", input.path, w.Code, w.Body)
		}
		assertAssistantErrorCode(t, w, e.ErrAIAssistantDisabled.Code())
	}
	for _, input := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/halman/runs/r", ""},
		{http.MethodPost, "/api/halman/runs/r/cancel", ""},
		{http.MethodPost, "/api/halman/runs/r/confirmation", `{"version":1,"decision":"reject"}`},
	} {
		mock.ExpectQuery("SELECT r.id,.* JOIN ai_conversation").WithArgs("r", "user", "user", sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"id"}))
		req := httptest.NewRequest(input.method, input.path, strings.NewReader(input.body))
		req.Header.Set("Authorization", token("user"))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("关闭后仍应检查本人运行，不能被功能开关拦截: %s %d", input.path, w.Code)
		}
	}
	for _, active := range []int{0, 1} {
		mock.ExpectBegin()
		mock.ExpectQuery("SELECT id FROM ai_conversation WHERE id=.*AND user_id=.*FOR UPDATE").WithArgs("conversation", "user").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("conversation"))
		mock.ExpectQuery("SELECT COUNT.*FROM ai_run.*WHERE conversation_id=").WithArgs("conversation").
			WillReturnRows(sqlmock.NewRows([]string{"active"}).AddRow(active))
		want := http.StatusNoContent
		if active > 0 {
			mock.ExpectRollback()
			want = http.StatusBadRequest
		} else {
			mock.ExpectExec("INSERT INTO ai_usage_daily").WithArgs(sqlmock.AnyArg(), "user", "conversation").WillReturnResult(sqlmock.NewResult(0, 0))
			mock.ExpectExec("UPDATE ai_run r").WithArgs(sqlmock.AnyArg(), "user", "conversation").WillReturnResult(sqlmock.NewResult(0, 0))
			mock.ExpectExec("UPDATE ai_run SET plan_status='cancelled'").WithArgs("conversation").WillReturnResult(sqlmock.NewResult(0, 0))
			mock.ExpectExec("UPDATE ai_conversation SET deleted_at=").WithArgs(sqlmock.AnyArg(), "conversation").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()
		}
		req := httptest.NewRequest(http.MethodDelete, "/api/halman/conversations/conversation", nil)
		req.Header.Set("Authorization", token("user"))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != want {
			t.Fatalf("无 body 删除指定会话返回意外状态: active=%d status=%d body=%s", active, w.Code, w.Body)
		}
		if active > 0 {
			assertAssistantErrorCode(t, w, e.ErrAIAssistantBusy.Code())
		}
	}
	for _, path := range []string{"/api/halman/agui", "/api/ai/conversations", "/internal/stat/ai_assistant/overview", "/api/halman/runs/r/confirm", "/api/halman/conversations/clear"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, path, nil))
		if w.Code != http.StatusNotFound && w.Code != http.StatusMethodNotAllowed {
			t.Fatalf("旧路径应删除: %s %d", path, w.Code)
		}
	}
	for _, path := range []string{"/api/halman/reports/overview", "/internal/stat/ai_assistant/overview"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusNotFound {
			t.Fatal("统计查询应由 aslan stat 直接实现")
		}
	}
	found := false
	for _, route := range r.Routes() {
		if route.Method == http.MethodDelete && route.Path == "/api/halman/conversations" {
			t.Fatal("不应保留清空全部会话的接口")
		}
		if route.Method == http.MethodDelete && route.Path == "/api/halman/conversations/:id" {
			found = true
		}
	}
	if !found {
		t.Fatal("必须保留删除指定会话的接口")
	}
}
