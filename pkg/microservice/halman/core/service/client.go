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

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
	userclient "github.com/koderover/zadig/v2/pkg/shared/client/user"
	"github.com/koderover/zadig/v2/pkg/tool/llm"
	"github.com/koderover/zadig/v2/pkg/types"
)

type PlatformClient struct {
	Users            *userclient.Client
	Auth             authv3.AuthorizationClient
	HTTP             *http.Client
	CheckAILicense   func() error
	LoadRuntime      func(context.Context) (*types.AIAssistantRuntime, error)
	LoadModelRuntime func(context.Context, string) (*types.AIAssistantRuntime, error)
	ListModels       func(context.Context, []string) ([]types.AIAssistantModelOption, error)
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

func (p *PlatformClient) ModelOptions(ctx context.Context, cfg types.AIAssistantConfig) (types.AIAssistantModelOptions, error) {
	if p.ListModels == nil {
		return types.AIAssistantModelOptions{}, fmt.Errorf("AI 助手模型列表读取未初始化")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	ids := cfg.ModelIDs()
	if !cfg.AllowUserModelSelection {
		ids = nil
		if cfg.LLMIntegrationID != "" {
			ids = []string{cfg.LLMIntegrationID}
		}
	}
	options, err := p.ListModels(ctx, ids)
	if !cfg.AllowUserModelSelection {
		for i := range options {
			options[i].ReasoningEfforts = []string{}
		}
	}
	return types.AIAssistantModelOptions{DefaultModelID: cfg.LLMIntegrationID, Models: options, AllowUserModelSelection: cfg.AllowUserModelSelection}, err
}

func validateModelPreference(cfg *types.AIAssistantRuntime, selection models.ModelSelection) error {
	id := selection.LLMIntegrationID
	if selection.ReasoningEffort != "" {
		if id == "" {
			return fmt.Errorf("%w: 保存思考强度时必须指定模型", common.ErrInvalidRequest)
		}
		if err := (llm.ModelParameters{ReasoningEffort: selection.ReasoningEffort}).Validate(llm.ProtocolOpenAI); err != nil {
			return fmt.Errorf("%w: %v", common.ErrInvalidRequest, err)
		}
	}
	if id != "" && !cfg.AllowsModel(id) {
		return common.ErrInvalidModelPreference
	}
	return nil
}

func (p *PlatformClient) selectModel(ctx context.Context, cfg *types.AIAssistantRuntime, selection models.ModelSelection) (*types.AIAssistantRuntime, error) {
	if err := validateModelPreference(cfg, selection); err != nil {
		return nil, err
	}
	id := selection.LLMIntegrationID
	if id == "" {
		return cfg, nil
	}
	if id != cfg.LLMIntegrationID {
		if p.LoadModelRuntime == nil {
			return nil, common.ErrModelUnavailable
		}
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		selected, err := p.LoadModelRuntime(ctx, id)
		if err != nil || selected == nil {
			return nil, common.ErrModelUnavailable
		}
		if !selected.Enabled {
			return nil, common.ErrAIDisabled
		}
		if !selected.AllowUserModelSelection || selected.LLMIntegrationID != id {
			return nil, common.ErrInvalidModelPreference
		}
		cfg = selected
	}
	if _, err := modelEndpoint(cfg); err != nil {
		return nil, common.ErrInvalidModelPreference
	}
	override := *cfg
	override.UsingModelPreference = true
	if selection.ReasoningEffort != "" {
		override.ReasoningEffort = selection.ReasoningEffort
	}
	return &override, nil
}

func (p *PlatformClient) modelPreferenceAllowed(ctx context.Context, cfg, current *types.AIAssistantRuntime) bool {
	if !cfg.UsingModelPreference {
		return true
	}
	id := cfg.LLMIntegrationID
	if !current.AllowUserModelSelection || !current.AllowsModel(id) {
		return false
	}
	if id == current.LLMIntegrationID {
		_, err := modelEndpoint(current)
		return err == nil
	}
	available, err := p.modelAvailable(ctx, id)
	return err == nil && available
}

func (p *PlatformClient) modelAvailable(ctx context.Context, id string) (bool, error) {
	if p.ListModels == nil {
		return false, common.ErrModelUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	options, err := p.ListModels(ctx, []string{id})
	return len(options) == 1 && options[0].ID == id, err
}

func (s *AgentService) UpdateModelPreference(ctx context.Context, user string, cfg *types.AIAssistantRuntime, preference models.ModelSelection) error {
	if err := validateModelPreference(cfg, preference); err != nil {
		return err
	}
	if id := preference.LLMIntegrationID; id != "" {
		available, err := s.Platform.modelAvailable(ctx, id)
		if err != nil {
			return err
		}
		if !available {
			return common.ErrInvalidModelPreference
		}
	}
	return s.Store.SaveModelPreference(ctx, user, preference)
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
