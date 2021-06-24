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

func FindDeliveryDeploy(args *commonrepo.DeliveryDeployArgs, log *zap.SugaredLogger) ([]*commonmodels.DeliveryDeploy, error) {
	resp, err := commonrepo.NewDeliveryDeployColl().Find(args)
	if err != nil {
		log.Errorf("find deliveryDeploy error: %v", err)
		return resp, e.ErrFindDeliveryDeploy
	}
	return resp, err
}

func DeleteDeliveryDeploy(args *commonrepo.DeliveryDeployArgs, log *zap.SugaredLogger) error {
	err := commonrepo.NewDeliveryDeployColl().Delete(args.ReleaseID)
	if err != nil {
		log.Errorf("delete deliveryDeploy error: %v", err)
		return e.ErrDeleteDeliveryDeploy
	}
	return nil
}
