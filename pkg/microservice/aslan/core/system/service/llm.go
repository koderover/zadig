/*
Copyright 2021 The KodeRover Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package service

import (
	"context"
	"fmt"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

func CheckLLMIntegration(ctx context.Context) (bool, error) {
	count, err := commonrepo.NewLLMIntegrationColl().Count(ctx)
	if err != nil {
		fmtErr := fmt.Errorf("CheckLLMIntegration err: %w", err)
		log.Error(fmtErr)
		return false, e.ErrListLLMIntegration.AddErr(fmtErr)
	}

	if count == 0 {
		return false, nil
	}

	return true, nil
}

func GetLLMIntegration(ctx context.Context, id string) (*commonmodels.LLMIntegration, error) {
	llmIntegration, err := commonrepo.NewLLMIntegrationColl().FindByID(ctx, id)
	if err != nil {
		fmtErr := fmt.Errorf("GetLLMIntegration err: %w", err)
		log.Error(fmtErr)
		return nil, e.ErrGetLLMIntegration.AddErr(fmtErr)
	}

	return llmIntegration, nil
}

func ListLLMIntegration(ctx context.Context) ([]*commonmodels.LLMIntegration, error) {
	llmIntegrations, err := commonrepo.NewLLMIntegrationColl().FindAll(ctx)
	if err != nil {
		fmtErr := fmt.Errorf("ListLLMIntegration err: %w", err)
		log.Error(fmtErr)
		return nil, e.ErrListLLMIntegration.AddErr(fmtErr)
	}

	return llmIntegrations, nil
}

func CreateLLMIntegration(ctx context.Context, args *commonmodels.LLMIntegration) error {
	count, err := commonrepo.NewLLMIntegrationColl().Count(ctx)
	if err != nil {
		fmtErr := fmt.Errorf("count llm intergration err: %w", err)
		log.Error(fmtErr)
		return e.ErrCreateLLMIntegration.AddErr(fmtErr)
	}
	if count > 0 {
		return e.ErrCreateLLMIntegration.AddDesc("llm integration already exists")
	}

	if err := commonrepo.NewLLMIntegrationColl().Create(ctx, args); err != nil {
		fmtErr := fmt.Errorf("CreateLLMIntegration err: %w", err)
		log.Error(fmtErr)
		return e.ErrCreateLLMIntegration.AddErr(fmtErr)
	}
	return nil
}

func ValidateLLMIntegration(ctx context.Context, args *commonmodels.LLMIntegration) error {
	llmClient, err := commonservice.NewLLMClient(args)
	if err != nil {
		return fmt.Errorf("验证 LLM 集成失败: %s", err)
	}

	_, err = llmClient.GetCompletion(ctx, "Hello")
	if err != nil {
		return fmt.Errorf("验证 LLM 集成失败: %s", err)
	}

	return nil
}

func UpdateLLMIntegration(ctx context.Context, ID string, args *commonmodels.LLMIntegration) error {
	if err := commonrepo.NewLLMIntegrationColl().Update(ctx, ID, args); err != nil {
		fmtErr := fmt.Errorf("UpdateLLMIntegration err: %w", err)
		log.Error(fmtErr)
		return e.ErrUpdateLLMIntegration.AddErr(fmtErr)
	}
	return nil
}

func DeleteLLMIntegration(ctx context.Context, ID string) error {
	if err := commonrepo.NewLLMIntegrationColl().Delete(ctx, ID); err != nil {
		fmtErr := fmt.Errorf("DeleteLLMIntegration err: %w", err)
		log.Error(fmtErr)
		return e.ErrDeleteCICDTools.AddErr(fmtErr)
	}
	return nil
}
