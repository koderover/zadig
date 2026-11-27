package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	agentevent "trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/session/inmemory"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
	"github.com/koderover/zadig/v2/pkg/types"
)

func TestStartRejectsStalePreferenceAndReusesExistingRun(t *testing.T) {
	store, mock := mockStore(t)
	s := &AgentService{Store: store, Platform: &PlatformClient{}}
	cfg := &types.AIAssistantRuntime{AIAssistantConfig: types.AIAssistantConfig{
		Enabled: true, HistoryRetentionDays: 90, AllowUserModelSelection: true, LLMIntegrationID: "default",
	}}
	req := models.RunRequest{RequestID: "request", Message: "你好"}
	mock.ExpectQuery("SELECT r.id,r.request_hash,r.created_at").WillReturnRows(sqlmock.NewRows([]string{"id", "request_hash", "created_at"}))
	mock.ExpectQuery("SELECT model_id,reasoning_effort").WithArgs("user").
		WillReturnRows(sqlmock.NewRows([]string{"model_id", "reasoning_effort"}).AddRow("removed", "high"))
	if _, err := s.Start(context.Background(), "user", "conversation", "", req, cfg); !errors.Is(err, common.ErrInvalidModelPreference) {
		t.Fatalf("偏好失效时应在启动模型前报错: %v", err)
	}

	now := time.Now().UTC()
	mock.ExpectQuery("SELECT r.id,r.request_hash,r.created_at").
		WillReturnRows(sqlmock.NewRows([]string{"id", "request_hash", "created_at"}).AddRow("existing", common.RequestFingerprint("conversation", req), now))
	mock.ExpectQuery("SELECT r.id, r.request_id,").WillReturnRows(sqlmock.NewRows(strings.Fields(
		"id request_id conversation_id status context created_at finished_at error input_tokens output_tokens skill_id skill_execution_id skill_status plan plan_version plan_status plan_expires_at answer_version question_source_run_id answer_available",
	)).AddRow("existing", "request", "conversation", "succeeded", "{}", now, now, "", 0, 0, "", nil, nil, nil, 0, nil, nil, 1, nil, true))
	run, err := s.Start(context.Background(), "user", "conversation", "", req, cfg)
	if err != nil || run.ID != "existing" {
		t.Fatalf("幂等重试应复用原运行，无需重读偏好: %v %v", run, err)
	}
}

func TestStartStreamSubscribesBeforeModelAndSurvivesRequestDisconnect(t *testing.T) {
	store, mock := mockStore(t)
	mock.ExpectQuery("SELECT r.id,r.request_hash,r.created_at").WillReturnRows(sqlmock.NewRows([]string{"id", "request_hash", "created_at"}))
	mock.ExpectQuery("SELECT model_id,reasoning_effort FROM ai_user_preference").WithArgs("user").
		WillReturnRows(sqlmock.NewRows([]string{"model_id", "reasoning_effort"}).AddRow("selected", "low"))
	expectSkillSnapshot(mock)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT user_id,title,title_source_at").WillReturnRows(sqlmock.NewRows([]string{"user_id", "title", "title_source_at"}).AddRow("user", "已有标题", nil))
	mock.ExpectQuery("SELECT id,request_hash FROM ai_run").WillReturnRows(sqlmock.NewRows([]string{"id", "request_hash"}))
	mock.ExpectQuery("SELECT COUNT.*FROM ai_run").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectExec("INSERT INTO ai_run").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("UPDATE ai_conversation SET title").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT id,context,resource_scopes").WillReturnRows(sqlmock.NewRows([]string{"id", "context", "scopes"}).AddRow("run", "{}", "[]"))
	mock.ExpectQuery("SELECT r.id FROM ai_run r JOIN ai_conversation.*r.plan IS NOT NULL").WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery("SELECT count.* FROM `ai_conversation`").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM ai_conversation .*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("conversation"))
	mock.ExpectQuery("SELECT status,model_id,model_name FROM ai_run .*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"status", "model_id", "model_name"}).AddRow("running", "stored-model-id", "stored-model-name"))
	mock.ExpectExec("UPDATE ai_run SET status").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO ai_usage_daily").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE ai_conversation SET last_active_at").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	cfg := &types.AIAssistantRuntime{
		AIAssistantConfig: types.AIAssistantConfig{Enabled: true, AllowUserModelSelection: true, HistoryRetentionDays: 90, LLMIntegrationID: "default", LLMIntegrationIDs: []string{"selected"}, RunTimeoutSeconds: model.IntPtr(60)},
		Protocol:          "openai", BaseURL: "http://model.test/v1", Model: "test", Token: "model-secret", Language: "en-us",
		MaxOutputTokens: 512, ReasoningEffort: "medium",
	}
	platform := testPlatform(t, func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("{}")) })
	current := *cfg
	current.Language = "zh-cn"
	platform.LoadRuntime = func(context.Context) (*types.AIAssistantRuntime, error) { return &current, nil }
	platform.LoadModelRuntime = func(_ context.Context, id string) (*types.AIAssistantRuntime, error) {
		selected := *cfg
		selected.LLMIntegrationID, selected.Model = id, "selected-model"
		return &selected, nil
	}
	platform.ListModels = func(context.Context, []string) ([]types.AIAssistantModelOption, error) {
		return []types.AIAssistantModelOption{{ID: "selected"}}, nil
	}
	s := &AgentService{Store: store, Platform: platform, Sessions: inmemory.NewSessionService(), Context: context.Background(), maxConcurrentRuns: 1, active: make(map[string]*activeRun)}
	t.Cleanup(func() { s.Close() })

	previous := model.DefaultNewHTTPClient
	t.Cleanup(func() { model.DefaultNewHTTPClient = previous })
	modelEntered, proceed := make(chan struct{}), make(chan struct{})
	defer close(proceed)
	model.DefaultNewHTTPClient = func(...model.HTTPClientOption) model.HTTPClient {
		return &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			deadline, ok := r.Context().Deadline()
			if remaining := time.Until(deadline); !ok || remaining <= 0 || remaining > time.Minute {
				t.Errorf("模型请求应受 60 秒运行上限约束，实际剩余 %v", remaining)
			}
			payload, err := io.ReadAll(r.Body)
			if err != nil || !strings.Contains(string(payload), "system language for this generation is English") || !strings.Contains(string(payload), "reasoning_content), if provided, must also be in English") {
				t.Error("回答和可见推理均须使用本次生成的英文语言快照")
			}
			if raw := string(payload); !strings.Contains(raw, `"model":"selected-model"`) || !strings.Contains(raw, `"max_completion_tokens":512`) || !strings.Contains(raw, `"reasoning_effort":"low"`) {
				t.Error("模型请求应带上选中的输出上限和推理等级")
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
	subscription, err := s.StartStream(requestContext, "user", "conversation", token("user"), models.RunRequest{
		RequestID: "request", Message: "你好",
	}, cfg)
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
	if text != "首段回答" || cfg.ReasoningEffort != "medium" {
		t.Fatalf("启动和断线之间不能丢失首段输出: %q", text)
	}
}
