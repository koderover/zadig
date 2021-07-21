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

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func ListDeployTarget(productName string, log *zap.SugaredLogger) ([]*commonmodels.ServiceModuleTarget, error) {
	serviceObjects := make([]*commonmodels.ServiceModuleTarget, 0)
	services, err := commonrepo.NewServiceColl().ListMaxRevisionsByProduct(productName)
	if err != nil {
		errMsg := fmt.Sprintf("[ServiceTmpl.List] error: %v", err)
		log.Error(errMsg)
		return serviceObjects, e.ErrListTemplate.AddDesc(errMsg)
	}

	//获取该项目下的的build的targets
	buildTargets := sets.NewString()
	builds, err := commonrepo.NewBuildColl().List(&commonrepo.BuildListOption{ProductName: productName})
	if err != nil {
		log.Errorf("[Build.List] %s error: %v", productName, err)
		return nil, e.ErrListBuildModule.AddErr(err)
	}
	for _, build := range builds {
		for _, serviceModuleTarget := range build.Targets {
			buildTargets.Insert(fmt.Sprintf("%s-%s", serviceModuleTarget.ServiceName, serviceModuleTarget.ServiceModule))
		}
	}

	for _, svc := range services {
		switch svc.Type {
		case setting.K8SDeployType, setting.HelmDeployType:
			for _, container := range svc.Containers {
				if !buildTargets.Has(fmt.Sprintf("%s-%s", svc.ServiceName, container.Name)) {
					serviceObjects = append(serviceObjects, &commonmodels.ServiceModuleTarget{
						ProductName:   svc.ProductName,
						ServiceName:   svc.ServiceName,
						ServiceModule: container.Name,
					})
				}
			}
		case setting.PMDeployType:
			if !buildTargets.Has(fmt.Sprintf("%s-%s", svc.ServiceName, svc.ServiceName)) {
				serviceObjects = append(serviceObjects, &commonmodels.ServiceModuleTarget{
					ProductName:   svc.ProductName,
					ServiceName:   svc.ServiceName,
					ServiceModule: svc.ServiceName,
				})
			}
		}
	}

	return serviceObjects, nil
}

func ListContainers(productName string, log *zap.SugaredLogger) ([]*commonmodels.ServiceModuleTarget, error) {
	var containerList []*commonmodels.ServiceModuleTarget
	// 获取该项目下的服务
	serviceTmpls, err := commonrepo.NewServiceColl().DistinctServices(&commonrepo.ServiceListOption{ProductName: productName, ExcludeStatus: setting.ProductStatusDeleting})
	if err != nil {
		log.Errorf("ServiceTmpl.ListServices error: %v", err)
		return containerList, e.ErrListTemplate.AddDesc(err.Error())
	}
	// 获取该项目使用到的所有服务组件
	for _, service := range serviceTmpls {
		if service.Type == setting.K8SDeployType || service.Type == setting.HelmDeployType {
			// service中没有container信息，需要重新从数据库获取
			opt := &commonrepo.ServiceFindOption{
				ProductName:   service.ProductName,
				ServiceName:   service.ServiceName,
				Revision:      service.Revision,
				ExcludeStatus: setting.ProductStatusDeleting,
			}
			serviceDetail, err := commonrepo.NewServiceColl().Find(opt)
			if err != nil {
				log.Errorf("ServiceTmpl.Find error: %v", err)
				continue
			}
			for _, container := range serviceDetail.Containers {
				containerList = append(containerList, &commonmodels.ServiceModuleTarget{
					ProductName:   service.ProductName,
					ServiceName:   service.ServiceName,
					ServiceModule: container.Name,
				})
			}
		} else if service.Type == setting.PMDeployType {
			containerList = append(containerList, &commonmodels.ServiceModuleTarget{
				ProductName:   service.ProductName,
				ServiceName:   service.ServiceName,
				ServiceModule: service.ServiceName,
			})
		}
	}
	return containerList, nil
}

type buildPreviewResp struct {
	Name    string                              `json:"name"`
	Targets []*commonmodels.ServiceModuleTarget `json:"targets"`
}

func ListBuildForProduct(productName string, containerList []*commonmodels.ServiceModuleTarget, log *zap.SugaredLogger) ([]*buildPreviewResp, error) {
	//获取当前项目下的构建信息
	opt := &commonrepo.BuildListOption{
		ProductName: productName,
	}
	currentProductBuilds, err := commonrepo.NewBuildColl().List(opt)
	if err != nil {
		log.Errorf("[Build.List] for product:%s error: %v", productName, err)
		return nil, e.ErrListBuildModule.AddErr(err)
	}

	containerMap := make(map[string]*commonmodels.ServiceModuleTarget)
	for _, container := range containerList {
		target := fmt.Sprintf("%s-%s-%s", container.ProductName, container.ServiceName, container.ServiceModule)
		containerMap[target] = container
	}

	resp := make([]*buildPreviewResp, 0)
	for _, build := range currentProductBuilds {
		// 确认该构建的服务组件是否在本项目中被使用，未被使用则为脏数据，不返回。
		used := false
		for _, target := range build.Targets {
			serviceModuleTarget := fmt.Sprintf("%s-%s-%s", target.ProductName, target.ServiceName, target.ServiceModule)
			if _, isExist := containerMap[serviceModuleTarget]; isExist {
				used = true
				break
			}
		}
		if !used {
			continue
		}

		b := &buildPreviewResp{
			Name:    build.Name,
			Targets: build.Targets,
		}
		resp = append(resp, b)
	}

	return resp, nil
}
