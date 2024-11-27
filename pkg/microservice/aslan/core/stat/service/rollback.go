/*
Copyright 2023 The KodeRover Authors.

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

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	"github.com/koderover/zadig/v2/pkg/types"
)

type GetRollbackStatDetailResponse struct {
	Total int64                        `json:"total" validate:"required"`
	Data  []*types.OpenAPIRollBackStat `json:"data" validate:"required"`
}

func GetRollbackStatDetail(ctx *internalhandler.Context, projectName, envName, serviceName string, startDate, endDate int64, pageNum, pageSize int) (*GetRollbackStatDetailResponse, error) {
	opt := &commonrepo.ListEnvInfoOption{
		ProjectName: projectName,
		EnvName:     envName,
		ServiceName: serviceName,
		StartTime:   startDate,
		EndTime:     endDate,
		Operation:   config.EnvOperationRollback,
		PageNum:     pageNum,
		PageSize:    pageSize,
	}

	rollbackStats, total, err := commonrepo.NewEnvInfoColl().List(ctx, opt)
	if err != nil {
		err = fmt.Errorf("failed to list rollback stats, error: %s", err)
		ctx.Logger.Error(err)
		return nil, err
	}

	resp := &GetRollbackStatDetailResponse{
		Total: total,
		Data:  util.ConvertEnvInfoToOpenAPIRollBackStat(rollbackStats),
	}

	return resp, nil
}
