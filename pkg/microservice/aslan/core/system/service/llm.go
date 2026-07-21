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
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
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
	normalizeLLMIntegration(args)
	if err := validateLLMIntegration(args); err != nil {
		return e.ErrCreateLLMIntegration.AddErr(err)
	}

	err := runLLMIntegrationTransaction(ctx, func(txCtx context.Context, repo *commonrepo.LLMIntegrationColl) error {
		count, err := repo.Count(txCtx)
		if err != nil {
			return fmt.Errorf("count llm integration: %w", err)
		}

		setDefault := shouldSetDefault(count, args.IsDefault)
		args.IsDefault = false
		if err := repo.Create(txCtx, args); err != nil {
			return fmt.Errorf("create llm integration: %w", err)
		}
		if setDefault {
			if err := repo.SetDefault(txCtx, args.ID.Hex()); err != nil {
				return fmt.Errorf("set default llm integration: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		log.Errorf("create llm integration: %v", err)
		return e.ErrCreateLLMIntegration.AddErr(err)
	}
	return nil
}

func ValidateLLMIntegration(ctx context.Context, args *commonmodels.LLMIntegration) error {
	normalizeLLMIntegration(args)
	if err := validateLLMIntegration(args); err != nil {
		return fmt.Errorf("验证 LLM 集成失败: %s", err)
	}
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
	normalizeLLMIntegration(args)
	if err := validateLLMIntegration(args); err != nil {
		return e.ErrUpdateLLMIntegration.AddErr(err)
	}

	err := runLLMIntegrationTransaction(ctx, func(txCtx context.Context, repo *commonrepo.LLMIntegrationColl) error {
		current, err := repo.FindByID(txCtx, ID)
		if err != nil {
			return fmt.Errorf("find llm integration: %w", err)
		}
		setDefault := args.IsDefault || current.IsDefault
		args.IsDefault = false
		if err := repo.Update(txCtx, ID, args); err != nil {
			return fmt.Errorf("update llm integration: %w", err)
		}
		if setDefault {
			if err := repo.SetDefault(txCtx, ID); err != nil {
				return fmt.Errorf("set default llm integration: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		log.Errorf("update llm integration: %v", err)
		return e.ErrUpdateLLMIntegration.AddErr(err)
	}
	return nil
}

func runLLMIntegrationTransaction(ctx context.Context, fn func(context.Context, *commonrepo.LLMIntegrationColl) error) (err error) {
	session, cleanup, err := mongotool.SessionWithTransaction(ctx)
	if err != nil {
		session.EndSession(ctx)
		return err
	}
	defer func() { cleanup(err) }()

	txCtx := mongotool.SessionContext(ctx, session)
	if err = fn(txCtx, commonrepo.NewLLMIntegrationColl()); err != nil {
		return err
	}
	return mongotool.CommitTransaction(session)
}

func DeleteLLMIntegration(ctx context.Context, ID string) error {
	repo := commonrepo.NewLLMIntegrationColl()
	integration, err := repo.FindByID(ctx, ID)
	if err != nil {
		return e.ErrDeleteLLMIntegration.AddErr(fmt.Errorf("find llm integration: %w", err))
	}
	count, err := repo.Count(ctx)
	if err != nil {
		return e.ErrDeleteLLMIntegration.AddErr(fmt.Errorf("count llm integrations: %w", err))
	}
	if err := validateLLMIntegrationDeletion(integration, count); err != nil {
		return e.ErrDeleteLLMIntegration.AddErr(err)
	}
	if err := repo.Delete(ctx, ID); err != nil {
		fmtErr := fmt.Errorf("DeleteLLMIntegration err: %w", err)
		log.Error(fmtErr)
		return e.ErrDeleteLLMIntegration.AddErr(fmtErr)
	}
	return nil
}
