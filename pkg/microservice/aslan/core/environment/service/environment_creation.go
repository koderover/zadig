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
	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	templatemodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/util"
)

// fill product services and chart infos and insert renderset data
func prepareHelmProductCreation(templateProduct *templatemodels.Product, productObj *commonmodels.Product, arg *CreateSingleProductArg, serviceTmplMap map[string]*commonmodels.Service, log *zap.SugaredLogger) error {
	err := validateArgs(arg.ValuesData)
	if err != nil {
		return fmt.Errorf("failed to validate args: %s", err)
	}

	productObj.ServiceRenders = make([]*templatemodels.ServiceRender, 0)
	// chart infos in template product
	templateChartInfoMap := make(map[string]*templatemodels.ServiceRender)
	for _, tc := range templateProduct.ChartInfos {
		templateChartInfoMap[tc.ServiceName] = tc
	}

	serviceDeployStrategy := make(map[string]string)

	// user custom chart values
	cvMap := make(map[string]*templatemodels.ServiceRender)
	for _, singleCV := range arg.ChartValues {
		tc, ok := templateChartInfoMap[singleCV.ServiceName]
		if !ok {
			return fmt.Errorf("failed to find chart info in product, serviceName: %s productName: %s", singleCV.ServiceName, templateProduct.ProjectName)
		}
		chartInfo := &templatemodels.ServiceRender{}
		singleCV.FillRenderChartModel(chartInfo, tc.ChartVersion)
		chartInfo.ValuesYaml = tc.ValuesYaml
		productObj.ServiceRenders = append(productObj.ServiceRenders, chartInfo)
		cvMap[singleCV.ServiceName] = chartInfo
		serviceDeployStrategy[singleCV.ServiceName] = singleCV.DeployStrategy
	}

	productObj.ServiceDeployStrategy = serviceDeployStrategy

	// default values
	defaultValuesYaml := arg.DefaultValues

	// generate service group data
	var serviceGroup [][]*commonmodels.ProductService
	for _, names := range templateProduct.Services {
		servicesResp := make([]*commonmodels.ProductService, 0)
		for _, serviceName := range names {
			// only the services chosen by use can be applied into product
			rc, ok := cvMap[serviceName]
			if !ok {
				continue
			}

			serviceTmpl, ok := serviceTmplMap[serviceName]
			if !ok {
				return e.ErrCreateEnv.AddDesc(fmt.Sprintf("failed to find service info in template_service, serviceName: %s", serviceName))
			}

			serviceResp := &commonmodels.ProductService{
				ServiceName: serviceTmpl.ServiceName,
				ProductName: serviceTmpl.ProductName,
				Type:        serviceTmpl.Type,
				Revision:    serviceTmpl.Revision,
			}
			serviceResp.Containers = make([]*commonmodels.Container, 0)
			var err error
			for _, c := range serviceTmpl.Containers {
				image := c.Image
				image, err = genImageFromYaml(c, rc.ValuesYaml, defaultValuesYaml, rc.GetOverrideYaml(), rc.OverrideValues)
				if err != nil {
					errMsg := fmt.Sprintf("genImageFromYaml product template %s,service name:%s,error:%s", productObj.ProductName, rc.ServiceName, err)
					log.Error(errMsg)
					return e.ErrCreateEnv.AddDesc(errMsg)
				}
				container := &commonmodels.Container{
					Name:      c.Name,
					ImageName: util.GetImageNameFromContainerInfo(c.ImageName, c.Name),
					Image:     image,
					ImagePath: c.ImagePath,
				}
				serviceResp.Containers = append(serviceResp.Containers, container)
			}
			servicesResp = append(servicesResp, serviceResp)
		}
		serviceGroup = append(serviceGroup, servicesResp)
	}
	productObj.Services = serviceGroup

	// insert renderset info into db
	err = commonservice.CreateK8sHelmRenderSet(&commonmodels.RenderSet{
		Name:          commonservice.GetProductEnvNamespace(arg.EnvName, arg.ProductName, arg.Namespace),
		EnvName:       arg.EnvName,
		ProductTmpl:   arg.ProductName,
		UpdateBy:      productObj.UpdateBy,
		IsDefault:     false,
		DefaultValues: arg.DefaultValues,
		ChartInfos:    productObj.ServiceRenders,
		YamlData:      geneYamlData(arg.ValuesData),
	}, log)
	if err != nil {
		log.Errorf("rennderset create fail when copy creating helm product, productName: %s,envname:%s,err:%s", arg.ProductName, arg.EnvName, err)
		return e.ErrCreateEnv.AddDesc(fmt.Sprintf("failed to save chart values, productName: %s,envname:%s,err:%s", arg.ProductName, arg.EnvName, err))
	}

	renderset, _, err := commonrepo.NewRenderSetColl().FindRenderSet(&commonrepo.RenderSetFindOption{
		Name:        commonservice.GetProductEnvNamespace(arg.EnvName, arg.ProductName, arg.Namespace),
		EnvName:     arg.EnvName,
		IsDefault:   false,
		ProductTmpl: productObj.ProductName,
	})
	if err != nil {
		return fmt.Errorf("failed to find renderset of product: %s/%s, err: %s", arg.ProductName, arg.EnvName, err)
	}

	if renderset != nil {
		productObj.Render = &commonmodels.RenderInfo{
			Name:        renderset.Name,
			Revision:    renderset.Revision,
			ProductTmpl: productObj.ProductName,
		}
	}

	return nil
}

func createSingleHostProduct(templateProduct *templatemodels.Product, requestID, userName, registryID string, arg *CreateSingleProductArg, log *zap.SugaredLogger) error {
	productObj := &commonmodels.Product{
		ProductName:     templateProduct.ProductName,
		Enabled:         false,
		EnvName:         arg.EnvName,
		UpdateBy:        userName,
		IsPublic:        true,
		ClusterID:       arg.ClusterID,
		Namespace:       arg.Namespace,
		Source:          setting.SourceFromExternal,
		IsOpenSource:    templateProduct.IsOpensource,
		IsForkedProduct: false,
		RegistryID:      registryID,
		IsExisted:       arg.IsExisted,
		Production:      arg.Production,
		Alias:           arg.Alias,
	}

	return CreateProduct(userName, requestID, productObj, log)
}

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
		Production:      arg.Production,
		Alias:           arg.Alias,
	}

	// fill services and chart infos of product
	err := prepareHelmProductCreation(templateProduct, productObj, arg, serviceTmplMap, log)
	if err != nil {
		return err
	}
	return CreateProduct(userName, requestID, productObj, log)
}

// CreateHostProductionProduct creates environment for host project, this function only creates production environment
func CreateHostProductionProduct(productName, userName, requestID string, args []*CreateSingleProductArg, log *zap.SugaredLogger) error {
	templateProduct, err := templaterepo.NewProductColl().Find(productName)
	if err != nil || templateProduct == nil {
		if err != nil {
			log.Errorf("failed to query product %s, err %s ", productName, err.Error())
		}
		return e.ErrCreateEnv.AddDesc(fmt.Sprintf("failed to query product %s ", productName))
	}

	errList := new(multierror.Error)
	for _, arg := range args {
		arg.Production = true
		err = createSingleHostProduct(templateProduct, requestID, userName, arg.RegistryID, arg, log)
		if err != nil {
			errList = multierror.Append(errList, err)
		}
	}
	return errList.ErrorOrNil()
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

func prepareK8sProductCreation(templateProduct *templatemodels.Product, productObj *commonmodels.Product, arg *CreateSingleProductArg, log *zap.SugaredLogger) error {
	templateChartInfoMap := templateProduct.AllServiceInfoMap()
	// validate the service is in product
	for _, createdSvcGroup := range arg.Services {
		for _, createdSvc := range createdSvcGroup {
			if createdSvc.ProductName != templateProduct.ProductName {
				continue
			}
			if _, ok := templateChartInfoMap[createdSvc.ServiceName]; !ok {
				return fmt.Errorf("failed to find service info in product, serviceName: %s productName: %s", createdSvc.ServiceName, templateProduct.ProjectName)
			}
		}
	}

	serviceDeployStrategy := make(map[string]string)
	// build product services
	productObj.Services = make([][]*commonmodels.ProductService, 0)
	for _, svcGroup := range arg.Services {
		sg := make([]*commonmodels.ProductService, 0)
		for _, svc := range svcGroup {
			sg = append(sg, svc.ProductService)
			serviceDeployStrategy[svc.ServiceName] = svc.DeployStrategy
		}
		productObj.Services = append(productObj.Services, sg)
	}
	productObj.ServiceDeployStrategy = serviceDeployStrategy

	// insert renderset info into db
	err := commonservice.CreateK8sHelmRenderSet(&commonmodels.RenderSet{
		Name:             commonservice.GetProductEnvNamespace(arg.EnvName, arg.ProductName, arg.Namespace),
		EnvName:          arg.EnvName,
		ProductTmpl:      arg.ProductName,
		UpdateBy:         productObj.UpdateBy,
		IsDefault:        false,
		DefaultValues:    arg.DefaultValues,
		ServiceVariables: productObj.ServiceRenders,
	}, log)
	if err != nil {
		log.Errorf("rennderset create fail when copy creating helm product, productName: %s,envname:%s,err:%s", arg.ProductName, arg.EnvName, err)
		return e.ErrCreateEnv.AddDesc(fmt.Sprintf("failed to save chart values, productName: %s,envname:%s,err:%s", arg.ProductName, arg.EnvName, err))
	}
	return nil
}

func createSingleYamlProduct(templateProduct *templatemodels.Product, requestID, userName string, arg *CreateSingleProductArg, log *zap.SugaredLogger) error {
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
		Production:      arg.Production,
		Alias:           arg.Alias,
		//Vars:            arg.Vars,
	}
	if len(arg.BaseEnvName) > 0 {
		productObj.BaseEnvName = arg.BaseEnvName
	}

	for _, svg := range arg.Services {
		for _, sv := range svg {
			productObj.ServiceRenders = append(productObj.ServiceRenders, &templatemodels.ServiceRender{
				ServiceName:  sv.ServiceName,
				OverrideYaml: &templatemodels.CustomYaml{YamlContent: sv.VariableYaml},
			})
		}
	}

	// fill services and chart infos of product
	err := prepareK8sProductCreation(templateProduct, productObj, arg, log)
	if err != nil {
		return err
	}
	return CreateProduct(userName, requestID, productObj, log)
}

func CreateYamlProduct(productName, userName, requestID string, args []*CreateSingleProductArg, log *zap.SugaredLogger) error {
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
		err = createSingleYamlProduct(templateProduct, requestID, userName, arg, log)
		if err != nil {
			errList = multierror.Append(errList, err)
		}
	}
	return errList.ErrorOrNil()
}
