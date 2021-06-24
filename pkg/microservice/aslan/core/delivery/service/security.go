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

func FindDeliverySecurity(args *commonrepo.DeliverySecurityArgs, log *zap.SugaredLogger) ([]*commonmodels.DeliverySecurity, error) {
	resp, err := commonrepo.NewDeliverySecurityColl().Find(args)
	if err != nil {
		log.Errorf("find deliverySecurity error: %v", err)
		return resp, e.ErrFindDeliverySecurity
	}
	return resp, err
}

func InsertDeliverySecurity(args *commonmodels.DeliverySecurity, log *zap.SugaredLogger) error {
	err := commonrepo.NewDeliverySecurityColl().Insert(args)
	if err != nil {
		log.Errorf("find deliverySecurity error: %v", err)
		return e.ErrCreateDeliverySecurity
	}
	return nil
}

func FindDeliverySecurityStatistics(imageID string, log *zap.SugaredLogger) (map[string]int, error) {
	resp, err := commonrepo.NewDeliverySecurityColl().FindStatistics(imageID)
	if err != nil {
		log.Errorf("find FindDeliverySecurityStatistics error: %v", err)
		return resp, e.ErrFindDeliverySecurityStats
	}
	return resp, err
}
