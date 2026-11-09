package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	agentevent "trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/session/inmemory"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
	"github.com/koderover/zadig/v2/pkg/types"
)

func TestStartStreamSubscribesBeforeModelAndSurvivesRequestDisconnect(t *testing.T) {
	store, mock := mockStore(t)
	mock.ExpectQuery("SELECT r.id,r.request_hash,r.created_at").WillReturnRows(sqlmock.NewRows([]string{"id", "request_hash", "created_at"}))
	expectSkillSnapshot(mock)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT user_id,title,title_source_at").WillReturnRows(sqlmock.NewRows([]string{"user_id", "title", "title_source_at"}).AddRow("user", "已有标题", nil))
	mock.ExpectQuery("SELECT id,request_hash FROM ai_run").WillReturnRows(sqlmock.NewRows([]string{"id", "request_hash"}))
	mock.ExpectQuery("SELECT COUNT.*FROM ai_run").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectExec("INSERT INTO ai_run").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("UPDATE ai_conversation SET title").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT id,context,resource_scopes").WillReturnRows(sqlmock.NewRows([]string{"id", "context", "scopes"}).AddRow("run", "{}", "[]"))
	mock.ExpectQuery("SELECT count.* FROM `ai_conversation`").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM ai_conversation .*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("conversation"))
	mock.ExpectQuery("SELECT status,model_id,model_name FROM ai_run .*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"status", "model_id", "model_name"}).AddRow("running", "stored-model-id", "stored-model-name"))
	mock.ExpectExec("UPDATE ai_run SET status").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO ai_usage_daily").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE ai_conversation SET last_active_at").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	cfg := &types.AIAssistantRuntime{AIAssistantConfig: types.AIAssistantConfig{Enabled: true, HistoryRetentionDays: 90, LLMIntegrationID: "selected"}, Protocol: "openai", BaseURL: "http://model.test/v1", Model: "test", Token: "model-secret", Language: "en-us"}
	platform := testPlatform(t, func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("{}")) })
	current := *cfg
	current.Language = "zh-cn"
	platform.LoadRuntime = func(context.Context) (*types.AIAssistantRuntime, error) { return &current, nil }
	s := &AgentService{Store: store, Platform: platform, Sessions: inmemory.NewSessionService(), Context: context.Background(), maxConcurrentRuns: 1, active: make(map[string]*activeRun)}
	t.Cleanup(func() { s.Close() })

	previous := model.DefaultNewHTTPClient
	t.Cleanup(func() { model.DefaultNewHTTPClient = previous })
	modelEntered, proceed := make(chan struct{}), make(chan struct{})
	defer close(proceed)
	model.DefaultNewHTTPClient = func(...model.HTTPClientOption) model.HTTPClient {
		return &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			payload, err := io.ReadAll(r.Body)
			if err != nil || !strings.Contains(string(payload), "system language for this generation is English") {
				t.Error("系统语言中途变化不能改变本次生成的英文语言快照")
			}
			s.mu.Lock()
			for _, active := range s.active {
				if len(active.subscribers) != 1 {
					t.Error("模型开始前必须已建立主订阅")
				}
			}
			s.mu.Unlock()
			close(modelEntered)
			<-proceed
			if r.Context().Err() != nil {
				t.Error("HTTP 断线不能取消后台运行")
			}
			body := "data: {\"id\":\"answer\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"首段回答\"},\"finish_reason\":null}]}\n\ndata: {\"id\":\"answer\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n"
			return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
		})}
	}
	requestContext, disconnect := context.WithCancel(context.Background())
	defer disconnect()
	subscription, err := s.StartStream(requestContext, "user", "conversation", token("user"), models.RunRequest{RequestID: "request", Message: "你好"}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer subscription.Unsubscribe()
	select {
	case <-modelEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("未开始模型运行")
	}
	disconnect()
	proceed <- struct{}{}
	var text string
	for event := range subscription.Events {
		if event.Type == "model" {
			response := event.Data.(*agentevent.Event).Response
			if response.IsPartial {
				for _, choice := range response.Choices {
					if choice.Delta.Role != model.RoleAssistant {
						t.Error("中间分片必须补齐助手角色以支持重连")
					}
					text += choice.Delta.Content
				}
			}
		}
	}
	if text != "首段回答" {
		t.Fatalf("启动和断线之间不能丢失首段输出: %q", text)
	}
}
