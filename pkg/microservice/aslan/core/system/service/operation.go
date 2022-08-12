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
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/system/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/system/repository/mongodb"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

type OperationLogArgs struct {
	Username     string `json:"username"`
	ProductName  string `json:"product_name"`
	ExactProduct string `json:"exact_product"`
	Function     string `json:"function"`
	Status       int    `json:"status"`
	PerPage      int    `json:"per_page"`
	Page         int    `json:"page"`
	Scene        string `json:"scene"`
	TargetID     string `json:"target_id"`
	Detail       string `json:"detail"`
}

func FindOperation(args *OperationLogArgs, log *zap.SugaredLogger) ([]*models.OperationLog, int, error) {
	resp, count, err := mongodb.NewOperationLogColl().Find(&mongodb.OperationLogArgs{
		Username:     args.Username,
		ProductName:  args.ProductName,
		ExactProduct: args.ExactProduct,
		Function:     args.Function,
		Status:       args.Status,
		PerPage:      args.PerPage,
		Page:         args.Page,
		Scene:        args.Scene,
		TargetID:     args.TargetID,
		Detail:       args.Detail,
	})
	if err != nil {
		log.Errorf("find operation log error: %v", err)
		return resp, count, e.ErrFindOperationLog.AddErr(err)
	}
	return resp, count, err
}

type AddAuditLogResp struct {
	OperationLogID string `json:"id"`
}

func InsertOperation(args *models.OperationLog, log *zap.SugaredLogger) (*AddAuditLogResp, error) {
	err := mongodb.NewOperationLogColl().Insert(args)
	if err != nil {
		log.Errorf("insert operation log error: %v", err)
		return nil, e.ErrCreateOperationLog
	}

	return &AddAuditLogResp{
		OperationLogID: args.ID.Hex(),
	}, nil
}

func UpdateOperation(id string, status int, log *zap.SugaredLogger) error {
	err := mongodb.NewOperationLogColl().Update(id, status)
	if err != nil {
		log.Errorf("update operation log error: %v", err)
		return e.ErrUpdateOperationLog
	}
	return nil
}
