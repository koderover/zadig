package service

import (
	"context"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/llmservice"
	"github.com/koderover/zadig/v2/pkg/tool/llm"
)

func GetLLMClient(ctx context.Context, name string) (llm.ILLM, error) {
	return llmservice.GetLLMClient(ctx, name)
}

func GetDefaultLLMClient(ctx context.Context) (llm.ILLM, error) {
	return llmservice.GetDefaultLLMClient(ctx)
}

func NewLLMClient(llmIntegration *models.LLMIntegration) (llm.ILLM, error) {
	return llmservice.NewLLMClient(llmIntegration)
}
