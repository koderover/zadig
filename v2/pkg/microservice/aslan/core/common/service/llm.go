package service

import (
	"context"
	"fmt"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/tool/llm"
)

func GetLLMClient(ctx context.Context, name string) (llm.ILLM, error) {
	llmIntegration, err := commonrepo.NewLLMIntegrationColl().FindByName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("Could find the llm integration for %s: %w", name, err)
	}

	llmConfig := llm.LLMConfig{
		Name:    llmIntegration.Name,
		Token:   llmIntegration.Token,
		BaseURL: llmIntegration.BaseURL,
	}
	if llmIntegration.EnableProxy {
		llmConfig.Proxy = config.ProxyHTTPSAddr()
	}

	llmClient, err := llm.NewClient(name)
	if err != nil {
		return nil, fmt.Errorf("Could not create the llm client for %s: %w", name, err)
	}

	err = llmClient.Configure(llmConfig)
	if err != nil {
		return nil, fmt.Errorf("Could not configure the llm client for %s: %w", name, err)
	}

	return llmClient, nil
}

func GetDefaultLLMClient(ctx context.Context) (llm.ILLM, error) {
	return GetLLMClient(ctx, "openai")
}
