package mongodb

import (
	"cmp"
	"context"
	"fmt"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/tool/language"
	"github.com/koderover/zadig/v2/pkg/tool/llm"
	"github.com/koderover/zadig/v2/pkg/types"
)

// GetAIAssistantRuntime 供 halman 直接加载配置，模型及代理凭据只在进程内使用。
func (c *SystemSettingColl) GetAIAssistantRuntime(ctx context.Context) (*types.AIAssistantRuntime, error) {
	return c.GetAIAssistantRuntimeForModel(ctx, "")
}

func (c *SystemSettingColl) GetAIAssistantRuntimeForModel(ctx context.Context, modelID string) (*types.AIAssistantRuntime, error) {
	document, err := c.getAIAssistantSettings(ctx)
	if err != nil {
		return nil, err
	}
	settings := document.AIAssistantConfig
	if err := settings.Validate(); err != nil {
		return nil, err
	}

	lang := language.Normalize(document.Language)
	runtime := &types.AIAssistantRuntime{AIAssistantConfig: *settings, Language: string(lang)}
	if !settings.Enabled {
		return runtime, nil
	}
	if modelID != "" {
		if !settings.AllowUserModelSelection || !settings.AllowsModel(modelID) {
			return nil, fmt.Errorf("模型未在 AI 助手中启用")
		}
		runtime.LLMIntegrationID = modelID
	}
	if runtime.LLMIntegrationID == "" {
		return runtime, nil
	}

	integrations := &LLMIntegrationColl{Collection: c.Database().Collection(models.LLMIntegration{}.TableName())}
	integration, err := integrations.FindByID(ctx, runtime.LLMIntegrationID)
	if err != nil {
		runtime.ModelError = lang.Text("所选 AI 助手模型集成已失效或暂不可用", "The selected AI assistant model is no longer available")
		return runtime, nil
	}

	runtime.Protocol = string(integration.Protocol)
	runtime.Provider = string(integration.ProviderName)
	runtime.Model = integration.Model
	runtime.BaseURL = integration.BaseURL
	runtime.Token = integration.Token
	runtime.ContextWindow = integration.ContextWindow
	runtime.MaxOutputTokens = integration.MaxOutputTokens
	runtime.ReasoningEffort = integration.ReasoningEffort
	if integration.EnableProxy {
		proxies := &ProxyColl{Collection: c.Database().Collection(models.Proxy{}.TableName())}
		items, err := proxies.ListWithContext(ctx, &ProxyArgs{})
		if err != nil {
			runtime.ModelError = lang.Text("AI 助手代理配置读取失败", "Failed to load the AI assistant proxy configuration")
			return runtime, nil
		}
		// 沿用现有代理选择规则，直接读取数据库以响应其他进程中的配置变更。
		if len(items) > 0 && items[0].EnableRepoProxy {
			switch items[0].Type {
			case "http", "https", "socks5":
				runtime.Proxy = items[0].GetProxyURL()
			}
		}
	}
	return runtime, nil
}

func (c *SystemSettingColl) ListAIAssistantModels(ctx context.Context, ids []string) ([]types.AIAssistantModelOption, error) {
	options := make([]types.AIAssistantModelOption, 0, len(ids))
	if len(ids) == 0 {
		return options, nil
	}
	integrations := &LLMIntegrationColl{Collection: c.Database().Collection(models.LLMIntegration{}.TableName())}
	selected, err := integrations.FindByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		item := selected[id]
		if item == nil || !item.SupportsAIAssistant() {
			continue
		}
		options = append(options, types.AIAssistantModelOption{
			ID: id, Name: cmp.Or(item.Name, item.Model, id), DefaultReasoningEffort: item.ReasoningEffort, ReasoningEfforts: llm.ReasoningEfforts(item.Protocol),
		})
	}
	return options, nil
}
