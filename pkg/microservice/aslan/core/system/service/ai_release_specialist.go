package service

import (
	"context"
	"fmt"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

var (
	readAIReleaseSpecialistConfig = func(ctx context.Context) (*commonmodels.AIReleaseSpecialistConfig, error) {
		return commonrepo.NewSystemSettingColl().GetAIReleaseSpecialistConfig(ctx)
	}
	writeAIReleaseSpecialistConfig = func(ctx context.Context, config *commonmodels.AIReleaseSpecialistConfig) error {
		return commonrepo.NewSystemSettingColl().UpdateAIReleaseSpecialistConfig(ctx, config)
	}
	findAIReleaseSpecialistModel = func(ctx context.Context, id string) (*commonmodels.LLMIntegration, error) {
		return commonrepo.NewLLMIntegrationColl().FindByID(ctx, id)
	}
)

func GetAIReleaseSpecialistConfig(ctx context.Context) (*commonmodels.AIReleaseSpecialistConfig, error) {
	config, err := readAIReleaseSpecialistConfig(ctx)
	if err != nil {
		return nil, e.NewWithDesc(e.ErrInternalError, fmt.Sprintf("读取发布专员配置失败: %v", err))
	}
	return config, nil
}

func UpdateAIReleaseSpecialistConfig(ctx context.Context, args *commonmodels.AIReleaseSpecialistConfig) error {
	if args == nil {
		return e.NewWithDesc(e.ErrInvalidParam, "发布专员配置不能为空")
	}
	if args.LLMIntegrationID != "" {
		if _, err := findAIReleaseSpecialistModel(ctx, args.LLMIntegrationID); err != nil {
			return e.NewWithDesc(e.ErrInvalidParam, "所选模型集成不存在或无法读取")
		}
	}
	if err := writeAIReleaseSpecialistConfig(ctx, args); err != nil {
		return e.NewWithDesc(e.ErrInternalError, fmt.Sprintf("保存发布专员配置失败: %v", err))
	}
	return nil
}
