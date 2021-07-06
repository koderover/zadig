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
)

func ListHelmRepos(log *zap.SugaredLogger) ([]*commonmodels.HelmRepo, error) {
	helmRepos, err := commonrepo.NewHelmRepoColl().List()
	if err != nil {
		log.Errorf("ListHelmRepos err:%v", err)
		return []*commonmodels.HelmRepo{}, nil
	}

	return helmRepos, nil
}

func CreateHelmRepo(args *commonmodels.HelmRepo, log *zap.SugaredLogger) error {
	if err := commonrepo.NewHelmRepoColl().Create(args); err != nil {
		log.Errorf("CreateHelmRepo err:%v", err)
		return err
	}
	return nil
}

func UpdateHelmRepo(id string, args *commonmodels.HelmRepo, log *zap.SugaredLogger) error {
	if err := commonrepo.NewHelmRepoColl().Update(id, args); err != nil {
		log.Errorf("UpdateHelmRepo err:%v", err)
		return err
	}
	return nil
}

func DeleteHelmRepo(id string, log *zap.SugaredLogger) error {
	if err := commonrepo.NewHelmRepoColl().Delete(id); err != nil {
		log.Errorf("DeleteHelmRepo err:%v", err)
		return err
	}
	return nil
}
