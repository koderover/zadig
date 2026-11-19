package service

import (
	"context"
	"fmt"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

var (
	readAIAuditConfig = func(ctx context.Context) (*commonmodels.AIAuditConfig, error) {
		return commonrepo.NewSystemSettingColl().GetAIAuditConfig(ctx)
	}
	writeAIAuditConfig = func(ctx context.Context, config *commonmodels.AIAuditConfig) error {
		return commonrepo.NewSystemSettingColl().UpdateAIAuditConfig(ctx, config)
	}
	findAIAuditModel = func(ctx context.Context, id string) (*commonmodels.LLMIntegration, error) {
		return commonrepo.NewLLMIntegrationColl().FindByID(ctx, id)
	}
)

func GetAIAuditConfig(ctx context.Context) (*commonmodels.AIAuditConfig, error) {
	config, err := readAIAuditConfig(ctx)
	if err != nil {
		return nil, e.NewWithDesc(e.ErrInternalError, fmt.Sprintf("读取审计分析配置失败: %v", err))
	}
	return config, nil
}

func UpdateAIAuditConfig(ctx context.Context, args *commonmodels.AIAuditConfig) error {
	if args == nil {
		return e.NewWithDesc(e.ErrInvalidParam, "审计分析配置不能为空")
	}
	if args.LLMIntegrationID != "" {
		if _, err := findAIAuditModel(ctx, args.LLMIntegrationID); err != nil {
			return e.NewWithDesc(e.ErrInvalidParam, "所选模型集成不存在或无法读取")
		}
	}
	if err := writeAIAuditConfig(ctx, args); err != nil {
		return e.NewWithDesc(e.ErrInternalError, fmt.Sprintf("保存审计分析配置失败: %v", err))
	}
	return nil
}
