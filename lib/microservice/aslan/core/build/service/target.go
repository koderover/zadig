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

	"k8s.io/apimachinery/pkg/util/sets"

	commonmodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	commonrepo "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	"github.com/koderover/zadig/lib/setting"
	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/tool/xlog"
)

func ListDeployTarget(productName string, log *xlog.Logger) ([]*commonmodels.ServiceModuleTarget, error) {
	serviceObjects := make([]*commonmodels.ServiceModuleTarget, 0)
	targetMap := map[string]bool{}
	serviceTmpls, err := commonrepo.NewServiceColl().ListMaxRevisions()
	if err != nil {
		errMsg := fmt.Sprintf("[ServiceTmpl.List] error: %v", err)
		log.Error(errMsg)
		return serviceObjects, e.ErrListTemplate.AddDesc(errMsg)
	}

	//获取该项目下的的build的targets
	buildTargets := sets.String{}
	builds, err := commonrepo.NewBuildColl().List(&commonrepo.BuildListOption{ProductName: productName})
	if err != nil {
		log.Errorf("[Build.List] %s error: %v", productName, err)
		return nil, e.ErrListBuildModule.AddErr(err)
	}
	for _, build := range builds {
		for _, serviceModuleTarget := range build.Targets {
			target := fmt.Sprintf("%s-%s-%s", serviceModuleTarget.ProductName, serviceModuleTarget.ServiceName, serviceModuleTarget.ServiceModule)
			if !buildTargets.Has(target) {
				buildTargets.Insert(target)
			}
		}
	}

	for _, serviceTmpl := range serviceTmpls {
		if serviceTmpl.ProductName != productName {
			continue
		}
		opt := &commonrepo.ServiceFindOption{
			ServiceName:   serviceTmpl.ServiceName,
			Type:          serviceTmpl.Type,
			Revision:      serviceTmpl.Revision,
			ProductName:   serviceTmpl.ProductName,
			ExcludeStatus: setting.ProductStatusDeleting,
		}
		_, err := commonrepo.NewServiceColl().Find(opt)
		if err != nil {
			log.Errorf("ServiceTmpl Find error: %v", err)
			continue
		}
		switch serviceTmpl.Type {
		case setting.K8SDeployType, setting.HelmDeployType:
			for _, container := range serviceTmpl.Containers {
				target := fmt.Sprintf("%s-%s-%s", serviceTmpl.ProductName, serviceTmpl.ServiceName, container.Name)
				if _, ok := targetMap[target]; !ok {
					targetMap[target] = true
					ServiceObject := &commonmodels.ServiceModuleTarget{
						ProductName:   serviceTmpl.ProductName,
						ServiceName:   serviceTmpl.ServiceName,
						ServiceModule: container.Name,
					}
					if !buildTargets.Has(target) {
						serviceObjects = append(serviceObjects, ServiceObject)
					}
				}
			}
		}
	}
	return serviceObjects, nil
}

func ListContainers(productName string, log *xlog.Logger) ([]*commonmodels.ServiceModuleTarget, error) {
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
		}
	}
	return containerList, nil
}

type buildPreviewResp struct {
	Name    string                              `json:"name"`
	Targets []*commonmodels.ServiceModuleTarget `json:"targets"`
}

func ListBuildForProduct(productName string, containerList []*commonmodels.ServiceModuleTarget, log *xlog.Logger) ([]*buildPreviewResp, error) {
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
