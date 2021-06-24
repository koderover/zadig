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

func FindDeliveryTest(args *commonrepo.DeliveryTestArgs, log *zap.SugaredLogger) ([]*commonmodels.DeliveryTest, error) {
	resp, err := commonrepo.NewDeliveryTestColl().Find(args)
	if err != nil {
		log.Errorf("find deliveryTest error: %v", err)
		return resp, e.ErrFindDeliveryTest
	}
	return resp, err
}

func DeleteDeliveryTest(args *commonrepo.DeliveryTestArgs, log *zap.SugaredLogger) error {
	err := commonrepo.NewDeliveryTestColl().Delete(args.ReleaseID)
	if err != nil {
		log.Errorf("delete deliveryTest error: %v", err)
		return e.ErrDeleteDeliveryTest
	}
	return nil
}
