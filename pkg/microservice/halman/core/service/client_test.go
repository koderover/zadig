package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	authv3 "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	"github.com/koderover/zadig/v2/pkg/types"
	rpcstatus "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc"
)

type fakeAuth struct {
	CheckFunc func(*authv3.CheckRequest) (*authv3.CheckResponse, error)
}

func (f fakeAuth) Check(_ context.Context, r *authv3.CheckRequest, _ ...grpc.CallOption) (*authv3.CheckResponse, error) {
	return f.CheckFunc(r)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func token(uid string) string {
	return "Bearer header." + base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"uid":%q}`, uid))) + ".signature"
}

func TestAuthenticateRequiresValidatedUserToken(t *testing.T) {
	p := &PlatformClient{Auth: fakeAuth{CheckFunc: func(r *authv3.CheckRequest) (*authv3.CheckResponse, error) {
		if r.Attributes.Request.Http.Headers["authorization"] != token("user-1") {
			t.Fatal("未透传实际用户登录态")
		}

		if r.Attributes.Request.Http.Path != "/api/halman/conversations" || r.Attributes.Request.Http.Body != "" {
			t.Fatal("鉴权不能走公共路径或携带聊天正文")
		}

		return &authv3.CheckResponse{
			Status:       &rpcstatus.Status{Code: 0},
			HttpResponse: &authv3.CheckResponse_OkResponse{OkResponse: &authv3.OkHttpResponse{}},
		}, nil
	}}}
	if uid, err := p.Authenticate(context.Background(), token("user-1")); err != nil || uid != "user-1" {
		t.Fatalf("登录态解析失败: %s %v", uid, err)
	}

	if _, err := p.Authenticate(context.Background(), ""); err == nil {
		t.Fatal("不能使用匿名或内部服务身份聊天")
	}

	p.Auth = fakeAuth{CheckFunc: func(*authv3.CheckRequest) (*authv3.CheckResponse, error) {
		return &authv3.CheckResponse{Status: &rpcstatus.Status{Code: 7}}, nil
	}}
	if _, err := p.Authenticate(context.Background(), token("forged")); err == nil {
		t.Fatal("不能信任被 user 拒绝的伪造 Token")
	}
}

func TestAuthenticateRequiresExplicitUserServiceApproval(t *testing.T) {
	for _, input := range []struct {
		name string
		resp *authv3.CheckResponse
		err  error
	}{
		{name: "鉴权服务错误", err: errors.New("unavailable")},
		{name: "空响应"},
		{name: "缺少状态", resp: &authv3.CheckResponse{HttpResponse: &authv3.CheckResponse_OkResponse{OkResponse: &authv3.OkHttpResponse{}}}},
		{name: "没有允许响应", resp: &authv3.CheckResponse{Status: &rpcstatus.Status{Code: 0}}},
	} {
		t.Run(input.name, func(t *testing.T) {
			p := &PlatformClient{Auth: fakeAuth{CheckFunc: func(*authv3.CheckRequest) (*authv3.CheckResponse, error) {
				return input.resp, input.err
			}}}
			if _, err := p.Authenticate(context.Background(), token("user")); err == nil {
				t.Fatal("不能只解码 JWT，必须得到 user 服务的明确允许")
			}
		})
	}
}

func TestAuthenticatedUserOnlyExposesVerifiedIdentityFields(t *testing.T) {
	checks := 0
	p := &PlatformClient{Auth: fakeAuth{CheckFunc: func(*authv3.CheckRequest) (*authv3.CheckResponse, error) {
		checks++
		return &authv3.CheckResponse{Status: &rpcstatus.Status{}, HttpResponse: &authv3.CheckResponse_OkResponse{OkResponse: &authv3.OkHttpResponse{}}}, nil
	}}}
	for _, claims := range []string{
		`{"uid":"user","name":"张三","preferred_username":"zhangsan","email":"hidden-email","phone":"hidden-phone","role":"hidden-role","token":"hidden-token"}`,
		`{"uid":"user"}`,
	} {
		authorization := "Bearer header." + base64.RawURLEncoding.EncodeToString([]byte(claims)) + ".signature"
		user, err := p.authenticateUser(context.Background(), authorization)
		if err != nil || user.UID != "user" {
			t.Fatalf("应兼容完整资料及仅包含 UID 的旧 Token: %+v %v", user, err)
		}
		if strings.Contains(claims, "preferred_username") && (user.Name != "张三" || user.Account != "zhangsan") {
			t.Fatal("应提取显示名称及登录账号")
		}
		raw, _ := json.Marshal(user)
		if strings.Contains(string(raw), "hidden-") || strings.Contains(string(raw), "signature") {
			t.Fatal("完整 Token、联系方式及角色声明不能进入身份上下文")
		}
	}
	if checks != 2 {
		t.Fatal("提取资料必须复用且仅执行一次平台鉴权")
	}
	p.Auth = fakeAuth{CheckFunc: func(*authv3.CheckRequest) (*authv3.CheckResponse, error) {
		return &authv3.CheckResponse{Status: &rpcstatus.Status{Code: 7}}, nil
	}}
	if user, err := p.authenticateUser(context.Background(), token("forged")); err == nil || user != (authenticatedUser{}) {
		t.Fatal("未经平台鉴权通过的 Claims 不可作为身份上下文")
	}
}

func TestRuntimeLoadsLocallyAndValidatesConfig(t *testing.T) {
	calls := 0
	p := &PlatformClient{LoadRuntime: func(ctx context.Context) (*types.AIAssistantRuntime, error) {
		calls++
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("配置数据库查询必须有超时")
		}
		return &types.AIAssistantRuntime{AIAssistantConfig: types.AIAssistantConfig{HistoryRetentionDays: 90}, ModelError: "模型已失效"}, nil
	}}
	cfg, err := p.Runtime(context.Background())
	if err != nil || calls != 1 || cfg.HistoryRetentionDays != 90 || cfg.ModelError == "" {
		t.Fatalf("模型失效不能阻止历史保留配置读取: %+v %v", cfg, err)
	}
	for _, load := range []func(context.Context) (*types.AIAssistantRuntime, error){
		nil,
		func(context.Context) (*types.AIAssistantRuntime, error) { return nil, nil },
		func(context.Context) (*types.AIAssistantRuntime, error) {
			return nil, errors.New("database credential")
		},
		func(context.Context) (*types.AIAssistantRuntime, error) { return &types.AIAssistantRuntime{}, nil },
	} {
		p.LoadRuntime = load
		if _, err := p.Runtime(context.Background()); err == nil {
			t.Fatal("配置缺失、查询失败或保留期无效时不能放行")
		}
	}
}

func TestSubscribeBackpressureDoesNotCancelRun(t *testing.T) {
	cancelled := false
	s := &AgentService{active: map[string]*activeRun{"run-1": {cancel: func() { cancelled = true }, subscribers: make(map[chan StreamEvent]struct{})}}}
	ch, unsubscribe, err := s.Subscribe("run-1")
	if err != nil {
		t.Fatal(err)
	}

	defer unsubscribe()
	for i := 0; i < 40; i++ {
		s.publish("run-1", StreamEvent{Type: "conversation_title", Data: map[string]string{"title": "文本"}})
	}

	resync := false
	for event := range ch {
		resync = resync || event.Type == "resync_required"
	}

	if cancelled || len(s.active) != 1 || !resync {
		t.Fatal("慢连接或断线不应取消业务运行")
	}
}

func TestSubscribeKeepsNewestConnectionWhenFull(t *testing.T) {
	s := &AgentService{active: map[string]*activeRun{"run-1": {subscribers: make(map[chan StreamEvent]struct{})}}}
	firstCh, first, err := s.Subscribe("run-1")
	if err != nil {
		t.Fatal(err)
	}

	_, second, err := s.Subscribe("run-1")
	if err != nil {
		t.Fatal(err)
	}

	defer second()
	// 未释放的旧连接不能把用户挡在运行之外：淘汰最早连接并提示其恢复，新连接照常建立。
	newest, third, err := s.Subscribe("run-1")
	if err != nil {
		t.Fatalf("额度满时仍应接受最新连接: %v", err)
	}

	defer third()
	resynced := false
	for event := range firstCh {
		resynced = resynced || event.Type == "resync_required"
	}

	if !resynced {
		t.Fatal("被淘汰的连接必须先收到恢复提示再断开")
	}

	first()
	if len(s.active["run-1"].subscribers) != 2 {
		t.Fatalf("退订后不应重复回收名额: %d", len(s.active["run-1"].subscribers))
	}

	// 最新连接必须仍能收到事件，淘汰不能误伤刚建立的订阅。
	s.publish("run-1", StreamEvent{Type: "conversation_title"})
	if event := <-newest; event.Type != "conversation_title" {
		t.Fatalf("最新连接应保持可用: %+v", event)
	}
}
