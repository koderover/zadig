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

	"github.com/pkg/errors"
	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/lark"
)

func ListIMApp(_type string, log *zap.SugaredLogger) ([]*commonmodels.IMApp, error) {
	resp, err := mongodb.NewIMAppColl().List(context.Background(), _type)
	if err != nil {
		log.Errorf("list external approval error: %v", err)
		return nil, e.ErrListIMApp.AddErr(err)
	}

	return resp, nil
}

func CreateIMApp(args *commonmodels.IMApp, log *zap.SugaredLogger) (string, error) {
	oid, err := mongodb.NewIMAppColl().Create(context.Background(), args)
	if err != nil {
		log.Errorf("create external approval error: %v", err)
		return "", e.ErrCreateIMApp.AddErr(err)
	}

	client := lark.NewClient(args.AppID, args.AppSecret)

	approvalCode, err := createLarkDefaultApprovalDefinition(client)
	if err != nil {
		return "", e.ErrCreateIMApp.AddErr(errors.Wrap(err, "create definition"))
	}
	err = client.SubscribeApprovalDefinition(&lark.SubscribeApprovalDefinitionArgs{
		ApprovalID: approvalCode,
	})
	if err != nil {
		return "", e.ErrCreateIMApp.AddErr(errors.Wrap(err, "subscribe"))
	}

	args.LarkDefaultApprovalCode = approvalCode
	err = mongodb.NewIMAppColl().Update(context.Background(), oid, args)
	if err != nil {
		return "", errors.Wrap(err, "update approval with approval code")
	}
	return "", nil
}

func UpdateIMApp(id string, args *commonmodels.IMApp, log *zap.SugaredLogger) error {
	if err := lark.Validate(args.AppID, args.AppSecret); err != nil {
		return e.ErrUpdateIMApp.AddErr(errors.Wrap(err, "validate"))
	}

	client := lark.NewClient(args.AppID, args.AppSecret)
	approvalCode, err := createLarkDefaultApprovalDefinition(client)
	if err != nil {
		return e.ErrUpdateIMApp.AddErr(errors.Wrap(err, "create definition"))
	}
	err = client.SubscribeApprovalDefinition(&lark.SubscribeApprovalDefinitionArgs{
		ApprovalID: approvalCode,
	})
	if err != nil {
		return e.ErrUpdateIMApp.AddErr(errors.Wrap(err, "subscribe"))
	}
	args.LarkDefaultApprovalCode = approvalCode

	err = mongodb.NewIMAppColl().Update(context.Background(), id, args)
	if err != nil {
		log.Errorf("update external approval error: %v", err)
		return e.ErrUpdateIMApp.AddErr(err)
	}
	return nil
}

func DeleteIMApp(id string, log *zap.SugaredLogger) error {
	err := mongodb.NewIMAppColl().DeleteByID(context.Background(), id)
	if err != nil {
		log.Errorf("delete external approval error: %v", err)
		return e.ErrDeleteIMApp.AddErr(err)
	}
	return nil
}

func ValidateIMApp(approval *commonmodels.IMApp, log *zap.SugaredLogger) error {
	switch approval.Type {
	case setting.IMLark:
		return lark.Validate(approval.AppID, approval.AppSecret)
	case setting.IMDingding:
	default:
		return e.ErrValidateIMApp.AddDesc("invalid type")
	}
	return nil
}

func createLarkDefaultApprovalDefinition(client *lark.Client) (string, error) {
	return client.CreateApprovalDefinition(&lark.CreateApprovalDefinitionArgs{
		Name:        "Zadig 工作流",
		Description: "Zadig 工作流",
		Type:        lark.ApproveTypeOr,
	})
}
