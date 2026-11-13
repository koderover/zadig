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

func TestHTTPDoesNotAcceptBodyUserIDAsIdentity(t *testing.T) {
	s, mock := mockStore(t)
	platform := &service.PlatformClient{

		CheckEnterpriseLicense: func() error { return nil },
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

func TestAllAIRoutesRequireEnterpriseLicenseAfterUserAuthentication(t *testing.T) {
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
				CheckEnterpriseLicense: func() error {
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
				t.Fatal("健康检查不应受用户登录或企业版许可证影响")
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
		CheckEnterpriseLicense: func() error { return nil },
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
	for _, busy := range []bool{false, true} {
		mock.ExpectBegin()
		mock.ExpectQuery("SELECT id FROM ai_conversation WHERE id=.*AND user_id=.*FOR UPDATE").WithArgs("conversation", "user").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("conversation"))
		count := 0
		if busy {
			count = 1
		}
		mock.ExpectQuery("SELECT COUNT.*FROM ai_run.*WHERE conversation_id=").WithArgs("conversation").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(count))
		want := http.StatusNoContent
		if busy {
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
			t.Fatalf("无 body 删除指定会话应限定当前用户且保护活动运行: busy=%v status=%d body=%s", busy, w.Code, w.Body)
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
