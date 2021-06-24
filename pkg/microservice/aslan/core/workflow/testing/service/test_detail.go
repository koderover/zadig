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
	"github.com/koderover/zadig/pkg/types"
)

type TestingDetail struct {
	Name        string                 `json:"name"`
	Envs        []*commonmodels.KeyVal `json:"envs"`
	Repos       []*types.Repository    `json:"repos"`
	Desc        string                 `json:"desc"`
	ProductName string                 `json:"product_name"`
}

func ListTestingDetails(productName string, log *zap.SugaredLogger) ([]*TestingDetail, error) {
	testings, err := commonrepo.NewTestingColl().List(&commonrepo.ListTestOption{ProductName: productName})
	if err != nil {
		log.Errorf("TestingModule.List error: %v", err)
		return nil, e.ErrListTestModule.AddDesc(err.Error())
	}

	testingOpts := make([]*TestingDetail, 0)
	for _, t := range testings {
		testingOpt := &TestingDetail{
			Name:        t.Name,
			Repos:       t.Repos,
			ProductName: t.ProductName,
			Desc:        t.Desc,
		}
		if t.PreTest != nil {
			testingOpt.Envs = t.PreTest.Envs
		}
		testingOpts = append(testingOpts, testingOpt)
	}
	return testingOpts, nil
}
