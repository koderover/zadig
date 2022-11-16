/*
Copyright 2022 The KodeRover Authors.

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
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	templatemodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"go.uber.org/zap"
)

func createSingleHelmProduct(templateProduct *templatemodels.Product, requestID, userName, registryID string, arg *CreateSingleProductArg, serviceTmplMap map[string]*commonmodels.Service, log *zap.SugaredLogger) error {
	productObj := &commonmodels.Product{
		ProductName:     templateProduct.ProductName,
		Revision:        templateProduct.Revision,
		Enabled:         false,
		EnvName:         arg.EnvName,
		UpdateBy:        userName,
		IsPublic:        true,
		ClusterID:       arg.ClusterID,
		Namespace:       commonservice.GetProductEnvNamespace(arg.EnvName, arg.ProductName, arg.Namespace),
		Source:          setting.SourceFromHelm,
		IsOpenSource:    templateProduct.IsOpensource,
		IsForkedProduct: false,
		RegistryID:      registryID,
		IsExisted:       arg.IsExisted,
		EnvConfigs:      arg.EnvConfigs,
		ShareEnv:        arg.ShareEnv,
	}

	// fill services and chart infos of product
	err := prepareHelmProductCreation(templateProduct, productObj, arg, serviceTmplMap, log)
	if err != nil {
		return err
	}
	return CreateProduct(userName, requestID, productObj, log)
}

func CreateHelmProduct(productName, userName, requestID string, args []*CreateSingleProductArg, log *zap.SugaredLogger) error {
	templateProduct, err := templaterepo.NewProductColl().Find(productName)
	if err != nil || templateProduct == nil {
		if err != nil {
			log.Errorf("failed to query product %s, err %s ", productName, err.Error())
		}
		return e.ErrCreateEnv.AddDesc(fmt.Sprintf("failed to query product %s ", productName))
	}

	// fill all chart infos from product renderset
	err = commonservice.FillProductTemplateValuesYamls(templateProduct, log)
	if err != nil {
		return e.ErrCreateEnv.AddDesc(err.Error())
	}

	// prepare data
	serviceTmpls, err := commonrepo.NewServiceColl().ListMaxRevisionsByProduct(productName)
	if err != nil {
		return e.ErrCreateEnv.AddErr(err)
	}
	templateServiceMap := make(map[string]*commonmodels.Service)
	for _, svc := range serviceTmpls {
		templateServiceMap[svc.ServiceName] = svc
	}

	errList := new(multierror.Error)
	for _, arg := range args {
		dataValid := true
		for _, cv := range arg.ChartValues {
			if _, ok := templateServiceMap[cv.ServiceName]; !ok {
				dataValid = false
				errList = multierror.Append(errList, fmt.Errorf("failed to find service tempalte, serviceName: %s", cv.ServiceName))
				break
			}
		}
		if !dataValid {
			continue
		}
		err = createSingleHelmProduct(templateProduct, requestID, userName, arg.RegistryID, arg, templateServiceMap, log)
		if err != nil {
			errList = multierror.Append(errList, err)
		}
	}
	return errList.ErrorOrNil()
}

func createSingleK8sProduct(templateProduct *templatemodels.Product, requestID, userName string, arg *CreateSingleProductArg, log *zap.SugaredLogger) error {
	productObj := &commonmodels.Product{
		ProductName:     templateProduct.ProductName,
		Revision:        templateProduct.Revision,
		Enabled:         false,
		EnvName:         arg.EnvName,
		UpdateBy:        userName,
		IsPublic:        true,
		ClusterID:       arg.ClusterID,
		Namespace:       commonservice.GetProductEnvNamespace(arg.EnvName, arg.ProductName, arg.Namespace),
		Source:          setting.SourceFromZadig,
		IsOpenSource:    templateProduct.IsOpensource,
		IsForkedProduct: false,
		RegistryID:      arg.RegistryID,
		IsExisted:       arg.IsExisted,
		EnvConfigs:      arg.EnvConfigs,
		ShareEnv:        arg.ShareEnv,
		Vars:            arg.Vars,
	}
	if len(arg.BaseEnvName) > 0 {
		productObj.BaseEnvName = arg.BaseEnvName
	}

	// fill services and chart infos of product
	err := prepareK8sProductCreation(templateProduct, productObj, arg, log)
	if err != nil {
		return err
	}
	return CreateProduct(userName, requestID, productObj, log)
}

func CreateK8sProduct(productName, userName, requestID string, args []*CreateSingleProductArg, log *zap.SugaredLogger) error {
	templateProduct, err := templaterepo.NewProductColl().Find(productName)
	if err != nil || templateProduct == nil {
		if err != nil {
			log.Errorf("failed to query product %s, err %s ", productName, err.Error())
		}
		return e.ErrCreateEnv.AddDesc(fmt.Sprintf("failed to query product %s ", productName))
	}

	// prepare data
	serviceTmpls, err := commonrepo.NewServiceColl().ListMaxRevisionsByProduct(productName)
	if err != nil {
		return e.ErrCreateEnv.AddErr(err)
	}
	templateServiceMap := make(map[string]*commonmodels.Service)
	for _, svc := range serviceTmpls {
		templateServiceMap[svc.ServiceName] = svc
	}

	errList := new(multierror.Error)
	for _, arg := range args {
		err = createSingleK8sProduct(templateProduct, requestID, userName, arg, log)
		if err != nil {
			errList = multierror.Append(errList, err)
		}
	}
	return errList.ErrorOrNil()
}
