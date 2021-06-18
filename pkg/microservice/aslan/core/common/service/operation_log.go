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

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/dao/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/dao/repo"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func InsertOperation(args *models.OperationLog, log *zap.SugaredLogger) error {
	err := repo.NewOperationLogColl().Insert(args)
	if err != nil {
		log.Errorf("insert operation log error: %v", err)
		return e.ErrCreateOperationLog
	}
	return nil
}

func UpdateOperation(id string, status int, log *zap.SugaredLogger) error {
	err := repo.NewOperationLogColl().Update(id, status)
	if err != nil {
		log.Errorf("update operation log error: %v", err)
		return e.ErrUpdateOperationLog
	}
	return nil
}
