package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	authv3 "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"

	userclient "github.com/koderover/zadig/v2/pkg/shared/client/user"
	"github.com/koderover/zadig/v2/pkg/types"
)

type PlatformClient struct {
	Users                  *userclient.Client
	Auth                   authv3.AuthorizationClient
	HTTP                   *http.Client
	CheckEnterpriseLicense func() error
	LoadRuntime            func(context.Context) (*types.AIAssistantRuntime, error)
}

// 仅提取身份展示所需字段，不向模型传递完整 Token 或其他 Claims。
type authenticatedUser struct {
	UID     string `json:"uid"`
	Name    string `json:"name,omitempty"`
	Account string `json:"preferred_username,omitempty"`
}

func (p *PlatformClient) Authenticate(ctx context.Context, authorization string) (string, error) {
	user, err := p.authenticateUser(ctx, authorization)
	return user.UID, err
}

func (p *PlatformClient) authenticateUser(ctx context.Context, authorization string) (authenticatedUser, error) {
	parts := strings.Fields(authorization)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return authenticatedUser{}, fmt.Errorf("需要有效的用户登录态")
	}
	if err := p.checkUserAuthorization(ctx, authorization); err != nil {
		return authenticatedUser{}, err
	}

	jwtParts := strings.Split(parts[1], ".")
	if len(jwtParts) != 3 {
		return authenticatedUser{}, fmt.Errorf("用户登录态格式无效")
	}

	data, err := base64.RawURLEncoding.DecodeString(jwtParts[1])
	if err != nil {
		return authenticatedUser{}, fmt.Errorf("用户登录态格式无效")
	}

	var claims authenticatedUser
	if json.Unmarshal(data, &claims) != nil || claims.UID == "" || len(claims.UID) > 128 {
		return authenticatedUser{}, fmt.Errorf("用户登录态缺少有效用户标识")
	}

	return claims, nil
}

func (p *PlatformClient) Runtime(ctx context.Context) (*types.AIAssistantRuntime, error) {
	if p.LoadRuntime == nil {
		return nil, fmt.Errorf("AI 助手配置读取未初始化")
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cfg, err := p.LoadRuntime(ctx)
	if err != nil || cfg == nil {
		return nil, fmt.Errorf("读取 AI 助手运行配置失败")
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// 直连内部 API 前仍由 user 验证原始 Token、撤销状态及 MFA，不以服务身份执行用户操作。
func (p *PlatformClient) checkUserAuthorization(ctx context.Context, authorization string) error {
	if p.Auth == nil {
		return fmt.Errorf("用户鉴权服务未配置")
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	// 复用 user 的登录态、撤销及 MFA 检查；不信任客户端传入的用户 ID 或内部服务身份。
	resp, err := p.Auth.Check(
		ctx,
		&authv3.CheckRequest{Attributes: &authv3.AttributeContext{Request: &authv3.AttributeContext_Request{Http: &authv3.AttributeContext_HttpRequest{
			Method:  http.MethodGet,
			Path:    "/api/halman/conversations",
			Headers: map[string]string{"authorization": authorization},
		}}}},
	)
	if err != nil || resp.GetStatus() == nil || resp.GetStatus().GetCode() != 0 || resp.GetOkResponse() == nil {
		return fmt.Errorf("用户登录态无效或已失效")
	}

	return nil
}
