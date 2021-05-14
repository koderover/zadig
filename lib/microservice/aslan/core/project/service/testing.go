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

	commonrepo "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	commonservice "github.com/koderover/zadig/lib/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/lib/tool/xlog"
)

func DeleteTestModules(productName string, log *xlog.Logger) error {
	testings, err := commonrepo.NewTestingColl().List(&commonrepo.ListTestOption{ProductName: productName})
	if err != nil {
		log.Errorf("test.List error: %v", err)
		return fmt.Errorf("DeleteTestModules productName %s test.List error: %v", productName, err)
	}
	errList := new(multierror.Error)
	for _, testing := range testings {
		if err = commonservice.DeleteTestModule(testing.Name, productName, log); err != nil {
			errList = multierror.Append(errList, fmt.Errorf("productName %s test delete %s error: %v", productName, testing.Name, err))
		}
	}
	if err := errList.ErrorOrNil(); err != nil {
		log.Error(err)
		return err
	}
	return nil
}
