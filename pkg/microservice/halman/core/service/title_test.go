package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/DATA-DOG/go-sqlmock"
	authv3 "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	rpcstatus "google.golang.org/genproto/googleapis/rpc/status"
	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/model"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
	"github.com/koderover/zadig/v2/pkg/types"
)

type titleModel struct {
	request *model.Request
	text    string
	err     error
}

func (m *titleModel) Info() model.Info { return model.Info{Name: "test"} }
func (m *titleModel) GenerateContent(_ context.Context, req *model.Request) (<-chan *model.Response, error) {
	m.request = req
	if m.err != nil {
		return nil, m.err
	}
	ch := make(chan *model.Response, 1)
	ch <- &model.Response{Choices: []model.Choice{{Message: model.NewAssistantMessage(m.text)}}, Usage: &model.Usage{PromptTokens: 3, CompletionTokens: 4}}
	close(ch)
	return ch, nil
}

func TestGenerateTitleBoundsAndFallback(t *testing.T) {
	for _, input := range []struct {
		name, text, want string
		err              error
	}{
		{name: "简洁标题", text: "“分析支付服务启动失败”", want: "分析支付服务启动失败"},
		{name: "中文长度", text: strings.Repeat("中", 50), want: strings.Repeat("中", 40)},
		{name: "多行响应", text: "标题\n解释"},
		{name: "空响应"},
		{name: "引号内空白", text: "“  ”"},
		{name: "控制字符", text: "标题\x00"},
		{name: "请求失败", err: errors.New("model unavailable")},
	} {
		t.Run(input.name, func(t *testing.T) {
			m := &titleModel{text: input.text, err: input.err}
			title, in, out := generateTitle(context.Background(), m, strings.Repeat("中", 2500)+" password=private-value")
			if title != input.want || !utf8.ValidString(title) {
				t.Fatalf("标题不符: %q", title)
			}
			if len(m.request.Tools) != 0 || m.request.Stream || *m.request.MaxTokens != 128 || utf8.RuneCountInString(m.request.Messages[1].Content) > 2000 {
				t.Fatal("标题必须独立限制输入和输出，且不注册工具")
			}
			if input.err == nil && (in != 3 || out != 4) {
				t.Fatal("即使标题无效也不能遗漏模型用量")
			}
		})
	}
	m := &titleModel{text: "数据库连接 password=private-value"}
	title, _, _ := generateTitle(context.Background(), m, "password=private-value")
	if strings.Contains(title, "private-value") || strings.Contains(m.request.Messages[1].Content, "private-value") {
		t.Fatal("标题输入输出必须沿用凭据脱敏")
	}
}

type answerRunner struct{}

func (answerRunner) Close() error { return nil }
func (answerRunner) Run(context.Context, string, string, model.Message, ...agent.RunOption) (<-chan *event.Event, error) {
	ch := make(chan *event.Event, 1)
	ch <- &event.Event{Response: &model.Response{Choices: []model.Choice{{Message: model.NewAssistantMessage("已分析服务错误")}}, Usage: &model.Usage{PromptTokens: 10, CompletionTokens: 2}}}
	close(ch)
	return ch, nil
}

func TestExecuteGeneratesTitleAndIncludesUsageWithoutExtraRun(t *testing.T) {
	for _, outcome := range []string{"生成成功", "标题失败保留正文", "标题超时保留正文"} {
		t.Run(outcome, func(t *testing.T) {
			fail := outcome != "生成成功"
			timeout := outcome == "标题超时保留正文"
			store, mock := mockStore(t)
			previous := model.DefaultNewHTTPClient
			t.Cleanup(func() { model.DefaultNewHTTPClient = previous })
			calls := 0
			model.DefaultNewHTTPClient = func(...model.HTTPClientOption) model.HTTPClient {
				return &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
					calls++
					body, _ := io.ReadAll(r.Body)
					if strings.Contains(string(body), "已分析服务错误") || strings.Contains(string(body), "model-secret") || r.Header.Get("Authorization") != "Bearer model-secret" {
						t.Fatal("标题只使用首条问题，模型凭据只能出现在认证头")
					}
					if timeout {
						<-r.Context().Done()
						return nil, r.Context().Err()
					}
					if fail {
						return nil, errors.New("unavailable")
					}
					return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"role":"assistant","content":"支付服务启动失败分析"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":4,"total_tokens":7}}`))}, nil
				})}
			}
			cfg := &types.AIAssistantRuntime{AIAssistantConfig: types.AIAssistantConfig{Enabled: true, LLMIntegrationID: "selected", HistoryRetentionDays: 90}, Model: "test", Protocol: "openai", BaseURL: "http://model.test/v1", Token: "model-secret"}
			platform := &PlatformClient{Auth: fakeAuth{CheckFunc: func(*authv3.CheckRequest) (*authv3.CheckResponse, error) {
				return &authv3.CheckResponse{Status: &rpcstatus.Status{}, HttpResponse: &authv3.CheckResponse_OkResponse{OkResponse: &authv3.OkHttpResponse{}}}, nil
			}}, LoadRuntime: func(context.Context) (*types.AIAssistantRuntime, error) { return cfg, nil }}
			r := &models.Run{ID: "run", ConversationID: "conversation", CreatedAt: time.Now().UTC().Truncate(time.Microsecond)}
			mock.ExpectQuery("SELECT count.* FROM `ai_conversation`").WithArgs("conversation", "user", r.CreatedAt).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
			in, out := 10, 2
			if !fail {
				in, out = 13, 6
				mock.ExpectExec("UPDATE ai_conversation SET title=.*title_source_at").WithArgs("支付服务启动失败分析", "conversation", "user", r.CreatedAt).WillReturnResult(sqlmock.NewResult(0, 1))
			}
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT id FROM ai_conversation .*FOR UPDATE").WithArgs("conversation", "user").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("conversation"))
			mock.ExpectQuery("SELECT status FROM ai_run .*FOR UPDATE").WithArgs("run").WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("running"))
			mock.ExpectExec("UPDATE ai_run SET status").WithArgs("succeeded", "", sqlmock.AnyArg(), in, out, sqlmock.AnyArg(), true, "run").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectExec("INSERT INTO ai_usage_daily").WithArgs(sqlmock.AnyArg(), "user", "", in, out, sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectExec("UPDATE ai_conversation SET last_active_at").WithArgs(sqlmock.AnyArg(), "conversation").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()
			updates := make(chan StreamEvent, 4)
			s := &AgentService{Store: store, Platform: platform, active: map[string]*activeRun{"run": {subscribers: map[chan StreamEvent]struct{}{updates: {}}}}}
			ctx, cancel := context.WithCancel(context.Background())
			if timeout {
				cancel()
				ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
			}
			ctx = context.WithValue(ctx, executionKey{}, &execution{Authorization: token("user")})
			s.wg.Add(1)
			s.execute(ctx, cancel, answerRunner{}, "user", r, "帮我分析支付服务启动失败", cfg)
			if !r.AnswerAvailable || calls != 1 {
				t.Fatalf("标题失败不能影响正文，调用不得重试: available=%v calls=%d", r.AnswerAvailable, calls)
			}
			if !fail {
				<-updates // 先接收正文模型事件，再更新会话标题。
				update := <-updates
				if update.Type != "conversation_title" {
					t.Fatal("标题成功后应通知前端更新会话列表")
				}
			}
		})
	}
}

func TestLaterRunSkipsTitleModel(t *testing.T) {
	store, mock := mockStore(t)
	r := &models.Run{ID: "later", ConversationID: "conversation", CreatedAt: time.Now().UTC()}
	mock.ExpectQuery("SELECT count.* FROM `ai_conversation`").WithArgs("conversation", "user", r.CreatedAt).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	// 不提供模型或平台客户端，非首轮不应访问它们。
	s := &AgentService{Store: store}
	if in, out := s.generateConversationTitle(context.Background(), "user", r, "继续", nil); in != 0 || out != 0 {
		t.Fatal("后续轮次不产生标题用量")
	}
}

func TestTitleGenerationRechecksLoginAndSwitch(t *testing.T) {
	for _, revoked := range []bool{false, true} {
		t.Run(map[bool]string{false: "助手已关闭", true: "登录态已撤销"}[revoked], func(t *testing.T) {
			store, mock := mockStore(t)
			previous := model.DefaultNewHTTPClient
			t.Cleanup(func() { model.DefaultNewHTTPClient = previous })
			model.DefaultNewHTTPClient = func(...model.HTTPClientOption) model.HTTPClient {
				t.Fatal("停用或失效登录不能创建标题模型")
				return nil
			}
			cfg := &types.AIAssistantRuntime{AIAssistantConfig: types.AIAssistantConfig{LLMIntegrationID: "selected", HistoryRetentionDays: 90}, Model: "test", Token: "secret", BaseURL: "http://model.test/v1"}
			p := &PlatformClient{Auth: fakeAuth{CheckFunc: func(*authv3.CheckRequest) (*authv3.CheckResponse, error) {
				if revoked {
					return &authv3.CheckResponse{Status: &rpcstatus.Status{Code: 7}}, nil
				}
				return &authv3.CheckResponse{Status: &rpcstatus.Status{}, HttpResponse: &authv3.CheckResponse_OkResponse{OkResponse: &authv3.OkHttpResponse{}}}, nil
			}}, LoadRuntime: func(context.Context) (*types.AIAssistantRuntime, error) {
				if revoked {
					t.Fatal("登录失败后不应继续读取配置")
				}
				return cfg, nil
			}}
			r := &models.Run{ConversationID: "conversation", CreatedAt: time.Now().UTC()}
			mock.ExpectQuery("SELECT count.* FROM `ai_conversation`").WithArgs("conversation", "user", r.CreatedAt).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
			ctx := context.WithValue(context.Background(), executionKey{}, &execution{Authorization: token("user")})
			if in, out := (&AgentService{Store: store, Platform: p}).generateConversationTitle(ctx, "user", r, "问题", cfg); in != 0 || out != 0 {
				t.Fatal("跳过标题不得产生模型用量")
			}
		})
	}
}
