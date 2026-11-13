package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	authv3 "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	rpcstatus "google.golang.org/genproto/googleapis/rpc/status"
	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/session"
	"trpc.group/trpc-go/trpc-agent-go/session/inmemory"

	halmanconfig "github.com/koderover/zadig/v2/pkg/microservice/halman/config"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
	"github.com/koderover/zadig/v2/pkg/types"
)

func TestConfiguredModelStreamsAndPersistsFinalAnswer(t *testing.T) {
	enabled, modelCalls := true, 0
	inspectionStarted := false
	previous := model.DefaultNewHTTPClient
	t.Cleanup(func() { model.DefaultNewHTTPClient = previous })
	model.DefaultNewHTTPClient = func(...model.HTTPClientOption) model.HTTPClient {
		return &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			modelCalls++
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}

			if strings.Contains(string(body), "model-secret") || strings.Contains(string(body), "signature") || strings.Contains(string(body), "hidden-") {
				t.Error("凭据不能进入模型上下文")
			}
			var payload struct{ Messages []model.Message }
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatal(err)
			}
			identityFound := false
			for _, message := range payload.Messages {
				if message.Role != model.RoleSystem || !strings.HasPrefix(message.Content, "当前登录用户（") {
					continue
				}
				var user authenticatedUser
				if err := json.Unmarshal([]byte(strings.SplitN(message.Content, "）：", 2)[1]), &user); err != nil || user.UID != "user-1" || user.Name != "当前用户" || user.Account != "current-account" {
					t.Fatalf("模型必须收到已验证的当前用户资料: %s %v", message.Content, err)
				}
				identityFound = true
			}
			if !identityFound {
				t.Error("模型请求缺少当前用户上下文")
			}
			if !strings.Contains(string(body), "本轮追加规则") || strings.Contains(string(body), `"name":"workflow_diagnosis"`) {
				t.Error("模型应收到数据库规则快照，且停用 Skill 的工具声明必须移除")
			}
			if strings.Contains(string(body), `"name":"environment_service_evidence"`) != inspectionStarted {
				t.Error("只有已经开始的环境巡检才能向模型提供补充服务证据工具")
			}

			if r.Header.Get("Authorization") != "Bearer model-secret" || r.URL.Host != "model.internal" {
				t.Error("没有使用本次选择的模型配置")
			}

			stream := `data: {"id":"chat-1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"role":"assistant","content":"你好"},"finish_reason":null}]}

data: {"id":"chat-1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":2,"total_tokens":12}}

data: [DONE]

`
			return &http.Response{
				StatusCode: 200,
				Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
				Body:       io.NopCloser(strings.NewReader(stream)),
				Request:    r,
			}, nil
		})}
	}
	platform := &PlatformClient{Auth: fakeAuth{CheckFunc: func(*authv3.CheckRequest) (*authv3.CheckResponse, error) {
		return &authv3.CheckResponse{
			Status:       &rpcstatus.Status{},
			HttpResponse: &authv3.CheckResponse_OkResponse{OkResponse: &authv3.OkHttpResponse{}},
		}, nil
	}}, LoadRuntime: func(context.Context) (*types.AIAssistantRuntime, error) {
		return &types.AIAssistantRuntime{AIAssistantConfig: types.AIAssistantConfig{Enabled: enabled, HistoryRetentionDays: 90}}, nil
	}}

	mem := inmemory.NewSessionService()
	defer mem.Close()
	r, err := makeRunner(
		&types.AIAssistantRuntime{
			AIAssistantConfig: types.AIAssistantConfig{LLMIntegrationID: "selected"},
			Protocol:          "openai",
			Provider:          "other",
			Model:             "selected-model",
			Token:             "model-secret",
			BaseURL:           "http://model.internal/v1",
		},
		mem,
		platform,
		"Bearer header."+base64.RawURLEncoding.EncodeToString([]byte(`{"uid":"user-1","name":"当前用户","preferred_username":"current-account","email":"hidden-email","phone":"hidden-phone","role":"hidden-role","token":"hidden-token"}`))+".signature",
		&AgentService{},
	)
	if err != nil {
		t.Fatal(err)
	}

	defer r.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	exec := &execution{Run: &models.Run{}, Skills: []models.Skill{
		{ID: "workflow_diagnosis", Source: "builtin", Enabled: false},
		{ID: "service_diagnosis", Source: "builtin", Enabled: true, Content: "诊断正文", CustomRules: "本轮追加规则"},
		{ID: "environment_inspection", Source: "builtin", Enabled: true},
	}}
	ctx = context.WithValue(ctx, executionKey{}, exec)
	events, err := r.Run(ctx, "user-1", "conversation-1", model.NewUserMessage("你好"), agent.WithRequestID("run-1"))
	if err != nil {
		t.Fatal(err)
	}

	delta, final := "", ""
	input, output := 0, 0
	for e := range events {
		if e.Response == nil {
			continue
		}

		if e.Error != nil {
			t.Errorf("模型返回错误: %+v", e.Error)
		}

		if !e.IsPartial && e.Usage != nil {
			input += e.Usage.PromptTokens
			output += e.Usage.CompletionTokens
		}

		for _, choice := range e.Choices {
			if e.IsPartial {
				delta += choice.Delta.Content
			} else if choice.Message.Role == model.RoleAssistant {
				final = choice.Message.Content
				if choice.FinishReason == nil || *choice.FinishReason != "stop" {
					t.Fatalf("完整响应必须保留结束原因: %v", choice.FinishReason)
				}
			}
		}
	}

	if delta != "你好" || final != "你好" {
		t.Fatalf("流式与最终回答不一致: %q %q", delta, final)
	}

	if input != 10 || output != 2 {
		t.Fatalf("流式用量不能重复计数或遗漏: %d %d", input, output)
	}

	saved, err := mem.GetSession(ctx, session.Key{AppName: halmanconfig.AppName, UserID: "user-1", SessionID: "conversation-1"})
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, e := range saved.Events {
		if e.Response != nil {
			for _, choice := range e.Choices {
				if choice.Message.Role == model.RoleAssistant && choice.Message.Content == "你好" {
					found = true
				}
			}
		}
	}

	if !found {
		t.Fatal("最终回答未保存为框架消息事件")
	}
	history, _ := json.Marshal(saved.Events)
	if strings.Contains(string(history), "current-account") || strings.Contains(string(history), "当前登录用户（") {
		t.Fatal("每轮注入的身份资料不能写入框架历史消息")
	}

	inspectionStarted, exec.SkillID = true, "environment_inspection"
	inspection, err := r.Run(ctx, "user-1", "inspection-conversation", model.NewUserMessage("补充服务证据"), agent.WithRequestID("run-inspection"))
	if err != nil {
		t.Fatal(err)
	}
	for e := range inspection {
		if e.Error != nil {
			t.Fatalf("已有巡检仍应可补充证据: %+v", e.Error)
		}
	}
	if modelCalls != 2 {
		t.Fatal("应分别验证新任务与已有巡检的模型工具列表")
	}

	enabled = false
	before := modelCalls
	blocked, err := r.Run(ctx, "user-1", "disabled-conversation", model.NewUserMessage("继续"), agent.WithRequestID("run-disabled"))
	if err != nil {
		t.Fatal(err)
	}
	for range blocked {
	}
	if modelCalls != before {
		t.Fatal("关闭 AI 后不能再发出模型请求")
	}
}

func TestModelDoesNotFallBackToEnvironment(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "environment-secret")
	if _, err := makeRunner(&types.AIAssistantRuntime{}, nil, nil, ""); err == nil {
		t.Fatal("未选择模型时不能使用环境默认模型")
	}

	if _, err := makeRunner(
		&types.AIAssistantRuntime{
			AIAssistantConfig: types.AIAssistantConfig{LLMIntegrationID: "selected"},
			Model:             "selected-model",
		},
		nil,
		nil,
		"",
	); err == nil {
		t.Fatal("未配置凭据时不能回退到环境凭据")
	}
}

func TestContextBudgetKeepsWholeRuns(t *testing.T) {
	now := time.Now()
	events := []event.Event{
		{RequestID: "old", Timestamp: now, Author: strings.Repeat("旧", 100)},
		{RequestID: "new", Timestamp: now, Author: "用户"},
		{RequestID: "new", Timestamp: now, Author: "助手"},
	}
	newBytes, _ := json.Marshal(events[1])
	answerBytes, _ := json.Marshal(events[2])
	selected := contextEvents(events, map[string]bool{"old": true, "new": true}, now.Add(-time.Hour), len(newBytes)+len(answerBytes), nil)
	if len(selected) != 2 || selected[0].RequestID != "new" || selected[1].RequestID != "new" {
		t.Fatalf("上下文不能仅截取一个运行的部分消息: %+v", selected)
	}
}
