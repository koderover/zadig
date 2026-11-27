package service

import (
	"context"

	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
)

func GetAIAssistantConfig(ctx context.Context) (*types.AIAssistantConfig, error) {
	config, err := commonrepo.NewSystemSettingColl().GetAIAssistantConfig(ctx)
	if err != nil {
		log.Errorf("读取 AI 助手配置失败: %v", err)
		return nil, e.NewWithDesc(e.ErrGetAIAssistantConfig, "配置读取失败，请稍后重试")
	}
	return config, nil
}

func UpdateAIAssistantConfig(ctx context.Context, args *types.AIAssistantConfig) error {
	if args == nil {
		return e.NewWithDesc(e.ErrInvalidParam, "AI 助手配置不能为空")
	}

	if err := args.Validate(); err != nil {
		return e.NewWithDesc(e.ErrInvalidParam, err.Error())
	}
	*args = args.WithRunDefaults()

	// 关闭功能不依赖模型仍然存在，失效的模型配置也必须能被停用。
	if ids := args.ModelIDs(); args.Enabled && len(ids) > 0 {
		models, err := commonrepo.NewLLMIntegrationColl().FindByIDs(ctx, ids)
		if err != nil || len(models) != len(ids) {
			return e.NewWithDesc(e.ErrInvalidParam, "所选模型集成不存在")
		}
		for _, model := range models {
			if len(args.LLMIntegrationIDs) != 0 && !model.SupportsAIAssistant() {
				return e.NewWithDesc(e.ErrInvalidParam, "AI 助手仅支持非 Azure 的 OpenAI 兼容模型")
			}
		}
	}

	if err := commonrepo.NewSystemSettingColl().UpdateAIAssistantConfig(ctx, args); err != nil {
		log.Errorf("保存 AI 助手配置失败: %v", err)
		return e.NewWithDesc(e.ErrUpdateAIAssistantConfig, "配置保存失败，请稍后重试")
	}
	return nil
}
