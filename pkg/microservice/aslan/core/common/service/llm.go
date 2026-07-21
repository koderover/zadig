package service

import (
	"context"
	"fmt"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/tool/llm"
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

	err = llmClient.Configure(llmConfig)
	if err != nil {
		return nil, fmt.Errorf("could not configure the llm client for %s: %w", llmConfig.ProviderName, err)
	}

	return llmClient, nil
}
