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
	"fmt"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

func GetSystemAIReviewConfig(ctx *internalhandler.Context) (*commonmodels.AIReviewConfig, error) {
	reviewConfig, err := commonrepo.NewSystemSettingColl().GetAIReviewConfig()
	if err != nil {
		fmtErr := fmt.Errorf("GetAIReviewConfig err: %w", err)
		log.Error(fmtErr)
		return nil, e.ErrGetAIReviewConfig.AddErr(err)
	}

	return reviewConfig, nil
}

func UpdateSystemAIReviewConfig(ctx *internalhandler.Context, args *commonmodels.AIReviewConfig) error {
	if args == nil {
		return e.ErrUpdateAIReviewConfig.AddDesc("AI review config cannot be empty")
	}
	if err := commonservice.ValidateReviewRules(args.Rules); err != nil {
		return e.ErrUpdateAIReviewConfig.AddErr(err)
	}
	err := commonrepo.NewSystemSettingColl().UpdateAIReviewConfig(args)
	if err != nil {
		fmtErr := fmt.Errorf("UpdateAIReviewConfig err: %w", err)
		log.Error(fmtErr)
		return e.ErrUpdateAIReviewConfig.AddErr(err)
	}
	return nil
}
