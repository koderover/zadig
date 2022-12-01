/*
 * Copyright 2022 The KodeRover Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package service

import (
	"context"

	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/lark"
)

func ListExternalApproval(log *zap.SugaredLogger) ([]*commonmodels.ExternalApproval, error) {
	resp, err := mongodb.NewExternalApprovalColl().List(context.Background())
	if err != nil {
		log.Errorf("list external approval error: %v", err)
		return nil, e.ErrListConfigurationManagement
	}

	return resp, nil
}

func CreateExternalApproval(args *commonmodels.ExternalApproval, log *zap.SugaredLogger) error {
	err := mongodb.NewExternalApprovalColl().Create(context.Background(), args)
	if err != nil {
		log.Errorf("create external approval error: %v", err)
		return e.ErrCreateExternalApproval.AddErr(err)
	}
	return nil
}

func UpdateExternalApproval(id string, args *commonmodels.ExternalApproval, log *zap.SugaredLogger) error {
	err := mongodb.NewExternalApprovalColl().Update(context.Background(), id, args)
	if err != nil {
		log.Errorf("update external approval error: %v", err)
		return e.ErrUpdateExternalApproval.AddErr(err)
	}
	return nil
}

func DeleteExternalApproval(id string, log *zap.SugaredLogger) error {
	err := mongodb.NewExternalApprovalColl().DeleteByID(context.Background(), id)
	if err != nil {
		log.Errorf("delete external approval error: %v", err)
		return e.ErrDeleteExternalApproval.AddErr(err)
	}
	return nil
}

func ValidateExternalApproval(approval *commonmodels.ExternalApproval, log *zap.SugaredLogger) error {
	switch approval.Type {
	case setting.IMLark:
		return lark.Validate(approval.AppID, approval.AppSecret)
	case setting.IMDingding:
	default:
		return e.ErrValidateExternalApproval.AddDesc("invalid type")
	}
	return nil
}
