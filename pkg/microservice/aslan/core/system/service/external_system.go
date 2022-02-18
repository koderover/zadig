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

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func CreateExternalSystem(args *ExternalSystemDetail, log *zap.SugaredLogger) error {
	err := commonrepo.NewExternalSystemColl().Create(&commonmodels.ExternalSystem{
		Name:     args.Name,
		Server:   args.Server,
		APIToken: args.APIToken,
	})
	if err != nil {
		log.Errorf("Create external system error: %s", err)
		return e.ErrCreateExternalLink.AddErr(err)
	}
	return nil
}

func ListExternalSystem(pageNum, pageSize int64, log *zap.SugaredLogger) ([]*ExternalSystemDetail, int64, error) {
	resp, length, err := commonrepo.NewExternalSystemColl().List(pageNum, pageSize)
	if err != nil {
		log.Errorf("Failed to list external system from db, the error is: %s", err)
		return nil, 0, err
	}
	systemList := make([]*ExternalSystemDetail, 0)
	for _, item := range resp {
		systemList = append(systemList, &ExternalSystemDetail{
			ID:     item.ID.Hex(),
			Name:   item.Name,
			Server: item.Server,
		})
	}
	return systemList, length, nil
}

func GetExternalSystemDetail(id string, log *zap.SugaredLogger) (*ExternalSystemDetail, error) {
	resp := new(ExternalSystemDetail)
	externalSystem, err := commonrepo.NewExternalSystemColl().GetByID(id)
	if err != nil {
		log.Errorf("Failed to get external system detail from id %s, the error is: %s", id, err)
		return nil, err
	}
	resp.ID = externalSystem.ID.Hex()
	resp.Name = externalSystem.Name
	resp.Server = externalSystem.Server
	resp.APIToken = externalSystem.APIToken
	return resp, nil
}

func UpdateExternalSystem(id string, system *ExternalSystemDetail, log *zap.SugaredLogger) error {
	err := commonrepo.NewExternalSystemColl().Update(
		id,
		&commonmodels.ExternalSystem{
			Name:     system.Name,
			Server:   system.Server,
			APIToken: system.APIToken,
		},
	)
	if err != nil {
		log.Errorf("update external system error: %s", err)
	}
	return err
}

func DeleteExternalSystem(id string, log *zap.SugaredLogger) error {
	err := commonrepo.NewExternalSystemColl().DeleteByID(id)
	if err != nil {
		log.Errorf("Failed to delete external system of id: %s, the error is: %s", id, err)
	}
	return err
}
