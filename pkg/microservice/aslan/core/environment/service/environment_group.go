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
	"sort"
	"strings"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/repository"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/kube/informer"
	"github.com/koderover/zadig/pkg/util"
	"go.uber.org/zap"
)

type EnvGroupRequest struct {
	ProjectName string `form:"projectName"`
	Page        int    `form:"page"`
	PerPage     int    `form:"perPage"`
	ServiceName string `form:"serviceName"`
}

func CalculateProductStatus(productInfo *commonmodels.Product, log *zap.SugaredLogger) (string, error) {
	envName, productName := productInfo.EnvName, productInfo.ProductName
	cls, err := kubeclient.GetKubeClientSet(config.HubServerAddress(), productInfo.ClusterID)
	if err != nil {
		log.Errorf("[%s][%s] error: %v", envName, productName, err)
		return setting.PodUnstable, e.ErrListGroups.AddDesc(err.Error())
	}
	inf, err := informer.NewInformer(productInfo.ClusterID, productInfo.Namespace, cls)
	if err != nil {
		log.Errorf("[%s][%s] error: %v", envName, productName, err)
		return setting.PodUnstable, e.ErrListGroups.AddDesc(err.Error())
	}

	k8sHandler := &K8sService{log: log}
	return k8sHandler.calculateProductStatus(productInfo, inf)
}

func ListGroups(serviceName, envName, productName string, perPage, page int, production bool, log *zap.SugaredLogger) ([]*commonservice.ServiceResp, int, error) {
	var (
		count           = 0
		allServices     = make([]*commonmodels.ProductService, 0)
		currentServices = make([]*commonmodels.ProductService, 0)
		resp            = make([]*commonservice.ServiceResp, 0)
	)

	projectType := getProjectType(productName)
	if projectType == setting.HelmDeployType {
		log.Infof("listing group for helm project is not supported: %s/%s", productName, envName)
		return resp, count, nil
	}

	opt := &commonrepo.ProductFindOptions{Name: productName, EnvName: envName, Production: util.GetBoolPointer(production)}
	productInfo, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		log.Errorf("[%s][%s] error: %v", envName, productName, err)
		return resp, count, e.ErrListGroups.AddDesc(err.Error())
	}

	for _, groupServices := range productInfo.Services {
		for _, service := range groupServices {
			if serviceName != "" && strings.Contains(service.ServiceName, serviceName) {
				allServices = append(allServices, service)
			} else if serviceName == "" {
				allServices = append(allServices, service)
			}
		}
	}
	count = len(allServices)
	//sort services by name
	sort.SliceStable(allServices, func(i, j int) bool { return allServices[i].ServiceName < allServices[j].ServiceName })

	// add updatable field
	latestSvcs, err := repository.GetMaxRevisionsServicesMap(productName, production)
	if err != nil {
		return resp, count, e.ErrListGroups.AddDesc(fmt.Sprintf("failed to find latest services for %s/%s", productName, envName))
	}
	for _, svc := range allServices {
		if latestSvc, ok := latestSvcs[svc.ServiceName]; ok {
			svc.Updatable = svc.Revision < latestSvc.Revision
		}
		svc.DeployStrategy = productInfo.ServiceDeployStrategy[svc.ServiceName]
	}

	cls, err := kubeclient.GetKubeClientSet(config.HubServerAddress(), productInfo.ClusterID)
	if err != nil {
		log.Errorf("[%s][%s] error: %v", envName, productName, err)
		return resp, count, e.ErrListGroups.AddDesc(err.Error())
	}
	inf, err := informer.NewInformer(productInfo.ClusterID, productInfo.Namespace, cls)
	if err != nil {
		log.Errorf("[%s][%s] error: %v", envName, productName, err)
		return resp, count, e.ErrListGroups.AddDesc(err.Error())
	}

	currentPage := page - 1
	if page*perPage < count {
		currentServices = allServices[currentPage*perPage : currentPage*perPage+perPage]
	} else {
		if currentPage*perPage > count {
			return resp, count, nil
		}
		currentServices = allServices[currentPage*perPage:]
	}
	resp = envHandleFunc(getProjectType(productName), log).listGroupServices(currentServices, envName, inf, productInfo)
	return resp, count, nil
}
