/*
Copyright 2023 The KodeRover Authors.

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
	"errors"
	"fmt"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/util"
)

func ListProductionServices(productName string, log *zap.SugaredLogger) (*service.ServiceTmplResp, error) {
	var err error
	resp := new(service.ServiceTmplResp)
	resp.Data = make([]*service.ServiceProductMap, 0)
	productTmpl, err := templaterepo.NewProductColl().Find(productName)
	if err != nil {
		log.Errorf("Can not find project %s, error: %s", productName, err)
		return resp, e.ErrListTemplate.AddDesc(err.Error())
	}

	services, err := commonrepo.NewProductionServiceColl().ListMaxRevisions(setting.K8SDeployType)

	if err != nil {
		log.Errorf("Failed to list services by %+v, err: %s", productTmpl.AllServiceInfos(), err)
		return resp, e.ErrListTemplate.AddDesc(err.Error())
	}

	for _, serviceObject := range services {
		spmap := &service.ServiceProductMap{
			Service:     serviceObject.ServiceName,
			Type:        serviceObject.Type,
			Source:      serviceObject.Source,
			ProductName: serviceObject.ProductName,
			Containers:  serviceObject.Containers,
			Visibility:  serviceObject.Visibility,
		}
		resp.Data = append(resp.Data, spmap)
	}
	resp.Total = len(services)

	return resp, nil
}

func GetProductionK8sService(serviceName, productName string, log *zap.SugaredLogger) (*commonmodels.Service, error) {
	var err error
	productTmpl, err := templaterepo.NewProductColl().Find(productName)
	if err != nil {
		log.Errorf("Can not find project %s, error: %s", productName, err)
		return nil, e.ErrListTemplate.AddDesc(err.Error())
	}

	serviceObject, err := commonrepo.NewProductionServiceColl().Find(&commonrepo.ServiceFindOption{
		ServiceName: serviceName,
		ProductName: productName,
		Type:        setting.K8SDeployType,
	})

	if err != nil {
		log.Errorf("Failed to list services by %+v, err: %s", productTmpl.AllServiceInfos(), err)
		return nil, e.ErrListTemplate.AddDesc(err.Error())
	}
	return serviceObject, nil
}

func CreateK8sProductionService(productName string, serviceObject *models.Service, log *zap.SugaredLogger) (*ServiceOption, error) {
	serviceObject.ProductName = productName
	productTempl, err := templaterepo.NewProductColl().Find(serviceObject.ProductName)
	if err != nil {
		log.Errorf("Failed to find project %s, err: %s", serviceObject.ProductName, err)
		return nil, e.ErrInvalidParam.AddErr(err)
	}

	currentSvc, err := commonrepo.NewProductionServiceColl().Find(&commonrepo.ServiceFindOption{
		ServiceName:   serviceObject.ServiceName,
		ProductName:   serviceObject.ProductName,
		Type:          setting.K8SDeployType,
		ExcludeStatus: setting.ProductStatusDeleting,
	})
	if err == nil && currentSvc != nil {
		if currentSvc.Yaml == serviceObject.Yaml && currentSvc.VariableYaml == serviceObject.VariableYaml {
			return getProductionServiceOption(currentSvc, log)
		}
	}

	err = ensureProductionServiceTmpl(serviceObject, log)
	if err != nil {
		return nil, e.ErrCreateTeam.AddErr(err)
	}

	// delete the service with same revision
	if err := commonrepo.NewProductionServiceColl().Delete(serviceObject.ServiceName, serviceObject.ProductName, setting.ProductStatusDeleting, serviceObject.Revision); err != nil {
		log.Errorf("ServiceTmpl.delete %s error: %v", serviceObject.ServiceName, err)
	}

	if len(productTempl.ProductionServices) == 0 {
		productTempl.ProductionServices = [][]string{{}}
	}
	allSvcs := sets.NewString()
	for _, svcs := range productTempl.ProductionServices {
		allSvcs.Insert(svcs...)
	}
	if !allSvcs.Has(serviceObject.ServiceName) {
		productTempl.ProductionServices[0] = append(productTempl.ProductionServices[0], serviceObject.ServiceName)
		err = templaterepo.NewProductColl().Update(serviceObject.ProductName, productTempl)
		if err != nil {
			log.Errorf("CreateK8sProductionService Update %s error: %s", serviceObject.ServiceName, err)
			return nil, e.ErrCreateTemplate.AddDesc(err.Error())
		}
	}

	err = commonrepo.NewProductionServiceColl().Create(serviceObject)
	if err != nil {
		log.Errorf("Failed to create production service %s, err: %s", serviceObject.ServiceName, err)
		return nil, e.ErrCreateTemplate.AddDesc(err.Error())
	}
	return getProductionServiceOption(serviceObject, log)
}

func ensureProductionServiceTmpl(args *commonmodels.Service, log *zap.SugaredLogger) error {
	if len(args.ServiceName) == 0 {
		return errors.New("service name is empty")
	}
	if !config.ServiceNameRegex.MatchString(args.ServiceName) {
		return fmt.Errorf("service name only support letters, numbers, dashes and underscores")
	}

	args.RenderedYaml = args.Yaml

	var err error
	args.RenderedYaml, err = renderK8sSvcYaml(args.RenderedYaml, args.ProductName, args.ServiceName, args.VariableYaml)
	if err != nil {
		return fmt.Errorf("failed to render yaml, err: %s", err)
	}

	args.Yaml = util.ReplaceWrapLine(args.Yaml)
	args.RenderedYaml = util.ReplaceWrapLine(args.RenderedYaml)
	args.KubeYamls = SplitYaml(args.RenderedYaml)

	// since service may contain go-template grammar, errors may occur when parsing as k8s workloads
	// errors will only be logged here
	if err := setCurrentContainerImages(args); err != nil {
		log.Errorf("failed to ser set container images, err: %s", err)
	}
	log.Infof("find %d containers in service %s", len(args.Containers), args.ServiceName)

	serviceTemplate := fmt.Sprintf(setting.ProductionServiceTemplateCounterName, args.ServiceName, args.ProductName)
	rev, err := commonrepo.NewCounterColl().GetNextSeq(serviceTemplate)
	if err != nil {
		return fmt.Errorf("get next service template revision error: %v", err)
	}
	args.Revision = rev
	return nil
}

func UpdateProductionServiceVariables(args *commonservice.ServiceTmplObject) error {
	currentService, err := commonrepo.NewProductionServiceColl().Find(&commonrepo.ServiceFindOption{
		ProductName: args.ProductName,
		ServiceName: args.ServiceName,
	})
	if err != nil {
		return e.ErrUpdateService.AddErr(fmt.Errorf("failed to get production service info, err: %s", err))
	}
	if currentService.Type != setting.K8SDeployType {
		return e.ErrUpdateService.AddErr(fmt.Errorf("invalid service type: %v", currentService.Type))
	}

	currentService.VariableYaml = args.VariableYaml

	err = commonrepo.NewProductionServiceColl().UpdateServiceVariables(currentService)
	if err != nil {
		return e.ErrUpdateService.AddErr(err)
	}

	// reparse service, check if container changes
	currentService.RenderedYaml, err = renderK8sSvcYaml(currentService.Yaml, args.ProductName, args.ServiceName, currentService.VariableYaml)
	if err != nil {
		return fmt.Errorf("failed to render yaml, err: %s", err)
	}

	currentService.RenderedYaml = util.ReplaceWrapLine(currentService.RenderedYaml)
	currentService.KubeYamls = SplitYaml(currentService.RenderedYaml)
	oldContainers := currentService.Containers
	if err := setCurrentContainerImages(currentService); err != nil {
		log.Errorf("failed to ser set container images, err: %s", err)
		//return err
	} else if containersChanged(oldContainers, currentService.Containers) {
		err = commonrepo.NewProductionServiceColl().UpdateServiceContainers(currentService)
		if err != nil {
			log.Errorf("failed to update service containers")
		}
	}
	return nil
}

func DeleteProductionServiceTemplate(serviceName, productName string, log *zap.SugaredLogger) error {
	err := commonrepo.NewProductionServiceColl().UpdateStatus(serviceName, productName, setting.ProductStatusDeleting)
	if err != nil {
		errMsg := fmt.Sprintf("productuion service %s delete error: %v", serviceName, err)
		log.Error(errMsg)
		return e.ErrDeleteTemplate.AddDesc(errMsg)
	}

	if productTempl, err := commonservice.GetProductTemplate(productName, log); err == nil {
		newServices := make([][]string, len(productTempl.Services))
		for i, services := range productTempl.ProductionServices {
			for _, singleService := range services {
				if singleService != serviceName {
					newServices[i] = append(newServices[i], singleService)
				}
			}
		}
		productTempl.ProductionServices = newServices
		err = templaterepo.NewProductColl().Update(productName, productTempl)
		if err != nil {
			log.Errorf("DeleteServiceTemplate Update %s error: %v", serviceName, err)
			return e.ErrDeleteTemplate.AddDesc(err.Error())
		}
	}
	return nil
}

func getProductionServiceOption(args *models.Service, log *zap.SugaredLogger) (*ServiceOption, error) {
	serviceOption := new(ServiceOption)

	serviceModules := make([]*ServiceModule, 0)
	for _, container := range args.Containers {
		serviceModule := new(ServiceModule)
		serviceModule.Container = container
		serviceModule.ImageName = util.GetImageNameFromContainerInfo(container.ImageName, container.Name)
		serviceModules = append(serviceModules, serviceModule)
	}
	serviceOption.ServiceModules = serviceModules
	serviceOption.SystemVariable = []*Variable{
		{
			Key:   "$Product$",
			Value: args.ProductName},
		{
			Key:   "$Service$",
			Value: args.ServiceName},
		{
			Key:   "$Namespace$",
			Value: ""},
		{
			Key:   "$EnvName$",
			Value: ""},
	}

	serviceOption.VariableYaml = args.VariableYaml

	serviceOption.Yaml = args.Yaml
	return serviceOption, nil
}
