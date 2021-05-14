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

package workflow

import (
	"errors"
	"fmt"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	commonrepo "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	commonservice "github.com/koderover/zadig/lib/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/lib/setting"
	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/tool/xlog"
)

const SplitSymbol = "&"

type ServiceTaskPreview struct {
	Workflows []*WorkflowPreview                 `json:"workflows"`
	Targets   []commonmodels.ServiceModuleTarget `json:"targets"`
}

type WorkflowPreview struct {
	Name         string              `json:"name"`
	WorkflowType config.PipelineType `json:"workflow_type"`
}

func ListServiceWorkflows(productName, envName, serviceName, serviceType string, log *xlog.Logger) (*ServiceTaskPreview, error) {
	resp := &ServiceTaskPreview{
		Workflows: make([]*WorkflowPreview, 0),
		Targets:   make([]commonmodels.ServiceModuleTarget, 0),
	}

	// helm类型的服务不支持使用工作流升级
	if serviceType == setting.HelmDeployType {
		log.Error("helm service cannot be upgraded by workflow")
		return resp, errors.New("helm service cannot be upgraded by workflow")
	}

	// 校验环境是否存在
	productOpt := &commonrepo.ProductFindOptions{
		Name:    productName,
		EnvName: envName,
	}
	prod, err := commonrepo.NewProductColl().Find(productOpt)
	if err != nil {
		log.Errorf("Product.Find failed, productName:%s, namespace:%s, err:%v", productName, envName, err)
		return resp, e.ErrFindProduct.AddErr(err)
	}

	// 校验服务是否存在
	serviceOpt := &commonrepo.ServiceFindOption{
		ServiceName:   serviceName,
		Type:          serviceType,
		ExcludeStatus: setting.ProductStatusDeleting,
	}
	service, err := commonrepo.NewServiceColl().Find(serviceOpt)
	if err != nil {
		log.Errorf("ServiceTmpl.Find failed, productName:%s, serviceName:%s, serviceType:%s, err:%v", productName, serviceName, serviceType, err)
		return resp, e.ErrGetService.AddErr(err)
	}

	if service.Visibility != setting.PUBLICSERVICE && service.ProductName != productName {
		log.Errorf("Find service failed, productName:%s, serviceName:%s, serviceType:%s", productName, serviceName, serviceType)
		return resp, e.ErrGetService
	}

	// 获取支持升级此服务的产品工作流
	workflowOpt := &commonrepo.ListWorkflowOption{ProductName: productName}
	workflows, err := commonrepo.NewWorkflowColl().List(workflowOpt)
	if err != nil {
		log.Errorf("Workflow.List failed, productName:%s, serviceName:%s, serviceType:%s, err:%v", productName, serviceName, serviceType, err)
		return resp, e.ErrListWorkflow.AddErr(err)
	}
	for _, workflow := range workflows {
		if workflow.BuildStage == nil {
			continue
		}
		// 如果工作流已经指定了环境，校验环境是否匹配
		if workflow.EnvName != "" && workflow.EnvName != envName {
			continue
		}
		workflowPreview := &WorkflowPreview{Name: workflow.Name, WorkflowType: config.WorkflowType}
		resp.Workflows = append(resp.Workflows, workflowPreview)
	}

	// 如果是k8s服务，还需要获取支持更新此服务的单服务工作流
	if serviceType == setting.K8SDeployType {
		pipelineOpt := &commonrepo.PipelineListOption{
			ProductName: productName,
			IsDeleted:   false,
		}
		pipelines, err := commonrepo.NewPipelineColl().List(pipelineOpt)
		if err != nil {
			log.Errorf("Pipeline.List failed, productName:%s, serviceName:%s, serviceType:%s, err:%v", productName, serviceName, serviceType, err)
			return resp, e.ErrListPipeline.AddErr(err)
		}
		for _, p := range pipelines {
			// 单服务工作的部署的服务只会有一个，不需考虑重复插入的问题
			for _, subTask := range p.SubTasks {
				pre, err := commonservice.ToPreview(subTask)
				if err != nil {
					log.Errorf("subTask.ToPreview error: %v", err)
					continue
				}
				if pre.TaskType != config.TaskDeploy {
					continue
				}
				deploy, err := commonservice.ToDeployTask(subTask)
				if err != nil || deploy == nil {
					log.Errorf("subTask.ToDeployTask error: %v", err)
					continue
				}
				if deploy.Enabled && deploy.Namespace == prod.Namespace && deploy.ServiceName == serviceName {
					workflowPreview := &WorkflowPreview{Name: p.Name, WorkflowType: config.SingleType}
					resp.Workflows = append(resp.Workflows, workflowPreview)
				}
			}
		}
	}

	allModules, err := ListBuildDetail("", "", "", log)
	if err != nil {
		log.Errorf("BuildModule.ListDetail error: %v", err)
		return resp, e.ErrListBuildModule.AddDesc(err.Error())
	}

	if serviceType == setting.K8SDeployType {
		for _, container := range service.Containers {
			// 不存在构建的服务组件直接跳过，不返回
			target := commonmodels.ServiceModuleTarget{
				ProductName:   service.ProductName,
				ServiceName:   service.ServiceName,
				ServiceModule: container.Name,
			}
			serviceModuleTarget := fmt.Sprintf("%s%s%s%s%s", service.ProductName, SplitSymbol, service.ServiceName, SplitSymbol, container.Name)
			moBuild := findModuleByTargetAndVersion(allModules, serviceModuleTarget, setting.Version)
			if moBuild == nil {
				continue
			}
			resp.Targets = append(resp.Targets, target)
		}
	} else {
		target := commonmodels.ServiceModuleTarget{
			ProductName:   service.ProductName,
			ServiceName:   service.ServiceName,
			ServiceModule: service.ServiceName,
		}
		serviceModuleTarget := fmt.Sprintf("%s%s%s%s%s", service.ProductName, SplitSymbol, service.ServiceName, SplitSymbol, service.ServiceName)
		moBuild := findModuleByTargetAndVersion(allModules, serviceModuleTarget, setting.Version)
		if moBuild != nil {
			resp.Targets = append(resp.Targets, target)
		}
	}

	return resp, nil
}
