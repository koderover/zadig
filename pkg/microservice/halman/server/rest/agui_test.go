package rest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	aguijson "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/encoding/json"
	authv3 "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	"github.com/gin-gonic/gin"
	rpcstatus "google.golang.org/genproto/googleapis/rpc/status"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/service"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/types"
)

func aguiTestEngine(t *testing.T) (*gin.Engine, sqlmock.Sqlmock) {
	t.Helper()
	store, mock := mockStore(t)
	platform := &service.PlatformClient{
		Auth: fakeAuth{CheckFunc: func(*authv3.CheckRequest) (*authv3.CheckResponse, error) {
			return &authv3.CheckResponse{Status: &rpcstatus.Status{}, HttpResponse: &authv3.CheckResponse_OkResponse{OkResponse: &authv3.OkHttpResponse{}}}, nil
		}},
		CheckEnterpriseLicense: func() error { return nil },
		LoadRuntime: func(context.Context) (*types.AIAssistantRuntime, error) {
			return &types.AIAssistantRuntime{AIAssistantConfig: types.AIAssistantConfig{Enabled: true, HistoryRetentionDays: 90}, ModelError: "模型已删除"}, nil
		},
	}
	return NewEngine(&service.AgentService{Store: store, Platform: platform}), mock
}

func TestAGUIHTTPRejectsInvalidInputBeforeDatabase(t *testing.T) {
	r, _ := aguiTestEngine(t)
	valid := `{"threadId":"conversation","runId":"request","messages":[{"id":"message","role":"user","content":"你好"}]}`
	for _, body := range []string{"", "{}", valid + "{}", strings.Replace(valid, `"threadId":"conversation"`, `"threadId":"another"`, 1), strings.Replace(valid, "user", "tool", 1), strings.Repeat(" ", 1<<20) + valid} {
		req := httptest.NewRequest(http.MethodPost, "/api/halman/conversations/conversation/runs", strings.NewReader(body))
		req.Header.Set("Authorization", token("user"))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest || strings.Contains(w.Header().Get("Content-Type"), "event-stream") {
			t.Fatalf("格式错误必须在启动运行和发送流前拒绝: %d %s", w.Code, w.Body)
		}
		assertAssistantErrorCode(t, w, e.ErrInvalidParam.Code())
	}
}

func TestAGUIHTTPDuplicateAndResubscriptionOnlyReadExistingRun(t *testing.T) {
	for _, subscribe := range []bool{false, true} {
		r, mock := aguiTestEngine(t)
		now := time.Now().UTC()
		request := models.RunRequest{RequestID: "request", Message: "你好"}
		if subscribe {
			mock.ExpectQuery("SELECT r.id,r.conversation_id,r.created_at").WithArgs("run", "user", "user", sqlmock.AnyArg()).
				WillReturnRows(sqlmock.NewRows([]string{"id", "conversation_id", "created_at", "skill_id", "skill_execution_id", "skill_status"}).
					AddRow("run", "conversation", now, "", nil, nil))
		} else {
			mock.ExpectQuery("SELECT r.id,r.request_hash,r.created_at").WithArgs("user", "request", "user").
				WillReturnRows(sqlmock.NewRows([]string{"id", "request_hash", "created_at"}).AddRow("run", common.RequestFingerprint("conversation", request), now))
		}
		count := 1
		if !subscribe {
			count = 2
		}
		for i := 0; i < count; i++ {
			rows := sqlmock.NewRows(strings.Split("id,request_id,conversation_id,status,context,created_at,finished_at,error,input_tokens,output_tokens,skill_id,skill_execution_id,skill_status,plan,plan_version,plan_status,plan_expires_at,answer_version,question_source_run_id,answer_available", ",")).
				AddRow("run", "request", "conversation", "succeeded", "{}", now, now, "", 1, 1, "", nil, nil, nil, 0, nil, nil, 1, nil, true)
			mock.ExpectQuery("SELECT r.id,.*JOIN ai_conversation").WithArgs("run", "user", "user", sqlmock.AnyArg()).WillReturnRows(rows)
		}
		path, method, body := "/api/halman/conversations/conversation/runs", http.MethodPost, `{"threadId":"conversation","runId":"request","messages":[{"id":"message","role":"user","content":"你好"}]}`
		if subscribe {
			path, method, body = "/api/halman/runs/run/events", http.MethodGet, ""
		} else {
			// 标准客户端可能回传较大的状态，换路由后仍需沿用 1 MiB 上限。
			body = strings.TrimSuffix(body, "}") + `,"state":{"padding":"` + strings.Repeat("x", 64<<10) + `"}}`
		}
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Authorization", token("user"))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "text/event-stream" || w.Header().Get("X-Accel-Buffering") != "no" {
			t.Fatalf("重复提交和恢复订阅都应输出 AG-UI，不依赖当前模型: %d %s", w.Code, w.Body)
		}
		frames := strings.Split(strings.TrimSpace(w.Body.String()), "\n\n")
		if len(frames) != 3 {
			t.Fatalf("已有终态运行只发送开始、快照和结束: %s", w.Body)
		}
		for i, kind := range []string{"RUN_STARTED", "STATE_SNAPSHOT", "RUN_FINISHED"} {
			data := strings.SplitN(frames[i], "data: ", 2)
			if len(data) != 2 {
				t.Fatalf("缺少 SSE data: %s", frames[i])
			}
			event, err := aguijson.NewDecoder().Decode(context.Background(), []byte(data[1]))
			if err != nil || string(event.Type()) != kind {
				t.Fatalf("事件不是 AG-UI 格式: %s", w.Body)
			}
			if i != 1 && (event.RunID() != "request" || event.ThreadID() != "conversation") {
				t.Fatal("协议必须保留客户端 runId 和会话 threadId")
			}
		}
	}
}

func TestAGUIHTTPDoesNotReadAnotherUsersRun(t *testing.T) {
	r, mock := aguiTestEngine(t)
	mock.ExpectQuery("SELECT r.id,r.conversation_id,r.created_at").WithArgs("other-run", "user", "user", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/halman/runs/other-run/events", nil)
	req.Header.Set("Authorization", token("user"))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("恢复订阅前必须校验本人归属: %d %s", w.Code, w.Body)
	}
}
