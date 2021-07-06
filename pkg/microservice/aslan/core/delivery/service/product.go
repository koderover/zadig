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

func FindDeliveryProduct(orgID int, log *zap.SugaredLogger) ([]string, error) {
	productNames, err := commonrepo.NewDeliveryVersionColl().FindProducts(orgID)
	if err != nil {
		log.Errorf("find FindDeliveryProduct error: %v", err)
		return []string{}, e.ErrFindDeliveryProducts
	}

	return productNames, err
}

func GetProductByDeliveryInfo(username, releaseID string, log *zap.SugaredLogger) (*commonmodels.Product, error) {
	version := new(commonrepo.DeliveryVersionArgs)
	version.ID = releaseID
	deliveryVersion, err := GetDeliveryVersion(version, log)
	if err != nil {
		log.Errorf("[User:%s][releaseID:%s] GetDeliveryVersion error: %v", username, releaseID, err)
		return nil, e.ErrGetEnv
	}
	return deliveryVersion.ProductEnvInfo, nil
}
