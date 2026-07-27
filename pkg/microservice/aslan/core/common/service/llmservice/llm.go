package llmservice

import (
	"context"
	"fmt"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/tool/llm"
)

const (
	agentAccessKeyHeader = "X-Access-Key"
	agentSecretKeyHeader = "X-Secret-Key"
)

func GetLLMClient(ctx context.Context, name string) (llm.ILLM, error) {
	llmIntegration, err := commonrepo.NewLLMIntegrationColl().FindByName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find the llm integration for %s, err: %w", name, err)
	}

	return NewLLMClient(llmIntegration)
}

func GetDefaultLLMClient(ctx context.Context) (llm.ILLM, error) {
	llmIntegration, err := commonrepo.NewLLMIntegrationColl().FindDefault(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to find default llm integration, err: %w", err)
	}

	return NewLLMClient(llmIntegration)
}

func NewLLMClient(llmIntegration *models.LLMIntegration) (llm.ILLM, error) {
	llmConfig := llm.LLMConfig{
		Name:         llmIntegration.Name,
		Protocol:     llmIntegration.Protocol,
		ProviderName: llmIntegration.ProviderName,
		Token:        llmIntegration.Token,
		BaseURL:      llmIntegration.BaseURL,
		Model:        llmIntegration.Model,
	}
	if llmIntegration.EnableProxy {
		llmConfig.Proxy = config.ProxyHTTPSAddr()
	}

	llmClient, err := llm.NewClientByProtocol(llmConfig.Protocol)
	if err != nil {
		return nil, fmt.Errorf("could not create the llm client for protocol %s: %w", llmConfig.Protocol, err)
	}

	if err := llmClient.Configure(llmConfig); err != nil {
		return nil, fmt.Errorf("could not configure the llm client for %s: %w", llmConfig.ProviderName, err)
	}

	return llmClient, nil
}

func NewAgentClient(integration *models.AgentIntegration) (llm.ILLM, error) {
	if integration == nil {
		return nil, fmt.Errorf("agent integration is required")
	}

	llmConfig := llm.LLMConfig{
		Name:         integration.Name,
		Protocol:     integration.Protocol,
		ProviderName: llm.ProviderOther,
		BaseURL:      integration.BaseURL,
		Model:        integration.Model,
	}
	switch integration.AuthType {
	case models.AgentAuthTypeAPIKey:
		llmConfig.Token = integration.APIKey
	case models.AgentAuthTypeAKSK:
		llmConfig.DisableAuth = true
		llmConfig.Headers = map[string]string{
			agentAccessKeyHeader: integration.AccessKey,
			agentSecretKeyHeader: integration.SecretKey,
		}
	default:
		return nil, fmt.Errorf("agent auth type %s is not supported", integration.AuthType)
	}

	client, err := llm.NewClientByProtocol(llmConfig.Protocol)
	if err != nil {
		return nil, fmt.Errorf("could not create the llm client for protocol %s: %w", llmConfig.Protocol, err)
	}
	if err := client.Configure(llmConfig); err != nil {
		return nil, fmt.Errorf("could not configure the llm client for %s: %w", llmConfig.ProviderName, err)
	}
	return client, nil
}
