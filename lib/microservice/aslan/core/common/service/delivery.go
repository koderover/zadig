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
	"fmt"

	"github.com/hashicorp/go-multierror"

	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/tool/xlog"
)

func DeleteDeliveryInfos(productName string, log *xlog.Logger) error {
	deliveryVersions, err := repo.NewDeliveryVersionColl().ListDeliveryVersions(productName, 1)
	if err != nil {
		log.Errorf("delete DeleteDeliveryInfo error: %v", err)
		return e.ErrDeleteDeliveryVersion
	}
	errList := new(multierror.Error)
	for _, deliveryVersion := range deliveryVersions {
		err = repo.NewDeliveryVersionColl().Delete(deliveryVersion.ID.Hex())
		if err != nil {
			errList = multierror.Append(errList, fmt.Errorf("DeliveryVersion delete %s error: %v", deliveryVersion.ID.String(), err))
		}
		err = repo.NewDeliveryBuildColl().Delete(deliveryVersion.ID.Hex())
		if err != nil {
			errList = multierror.Append(errList, fmt.Errorf("DeliveryBuild delete %s error: %v", deliveryVersion.ID.String(), err))
		}
		err = repo.NewDeliveryDeployColl().Delete(deliveryVersion.ID.Hex())
		if err != nil {
			errList = multierror.Append(errList, fmt.Errorf("DeliveryDeploy delete %s error: %v", deliveryVersion.ID.String(), err))
		}
		err = repo.NewDeliveryTestColl().Delete(deliveryVersion.ID.Hex())
		if err != nil {
			errList = multierror.Append(errList, fmt.Errorf("DeliveryTest delete %s error: %v", deliveryVersion.ID.String(), err))
		}
		err = repo.NewDeliveryDistributeColl().Delete(deliveryVersion.ID.Hex())
		if err != nil {
			errList = multierror.Append(errList, fmt.Errorf("DeliveryDistribute delete %s error: %v", deliveryVersion.ID.String(), err))
		}
	}
	if err := errList.ErrorOrNil(); err != nil {
		log.Error(err)
		return err
	}
	return nil
}
