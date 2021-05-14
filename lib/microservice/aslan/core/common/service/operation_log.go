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
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/tool/xlog"
)

func InsertOperation(args *models.OperationLog, log *xlog.Logger) error {
	err := repo.NewOperationLogColl().Insert(args)
	if err != nil {
		log.Errorf("insert operation log error: %v", err)
		return e.ErrCreateOperationLog
	}
	return nil
}

func UpdateOperation(id string, status int, log *xlog.Logger) error {
	err := repo.NewOperationLogColl().Update(id, status)
	if err != nil {
		log.Errorf("update operation log error: %v", err)
		return e.ErrUpdateOperationLog
	}
	return nil
}

func FindOperation(args *repo.OperationLogArgs, log *xlog.Logger) ([]*models.OperationLog, int, error) {
	resp, count, err := repo.NewOperationLogColl().Find(args)
	if err != nil {
		log.Errorf("find operation log error: %v", err)
		return resp, count, e.ErrFindOperationLog
	}
	return resp, count, err
}
