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

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/llm"
	"github.com/koderover/zadig/pkg/tool/log"
)

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
		return e.ErrDeleteJenkinsIntegration.AddErr(fmtErr)
	}
	return nil
}

func GetLLMClient(ctx context.Context, name string) (llm.ILLM, error) {
	llmIntegration, err := commonrepo.NewLLMIntegrationColl().FindByName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("Could find the llm integration for %s: %w", name, err)
	}

	config := llm.LLMConfig{
		Name:    llmIntegration.Name,
		Token:   llmIntegration.Token,
		BaseURL: llmIntegration.BaseURL,
	}
	llmClient, err := llm.NewClient(name)
	if err != nil {
		return nil, fmt.Errorf("Could not create the llm client for %s: %w", name, err)
	}

	err = llmClient.Configure(config)
	if err != nil {
		return nil, fmt.Errorf("Could not configure the llm client for %s: %w", name, err)
	}

	return llmClient, nil
}

func GetDefaultLLMClient(ctx context.Context) (llm.ILLM, error) {
	return GetLLMClient(ctx, "openai")
}
