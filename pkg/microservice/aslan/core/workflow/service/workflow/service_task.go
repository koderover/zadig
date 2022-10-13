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
	"sort"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	taskmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/task"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/base"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/s3"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/types"
)

const SplitSymbol = "&"

type ServiceTaskPreview struct {
	Workflows []*Preview                         `json:"workflows"`
	Targets   []commonmodels.ServiceModuleTarget `json:"targets"`
}

type Preview struct {
	Name         string              `json:"name"`
	DisplayName  string              `json:"display_name"`
	WorkflowType config.PipelineType `json:"workflow_type"`
}

func ListServiceWorkflows(productName, envName, serviceName, serviceType string, log *zap.SugaredLogger) (*ServiceTaskPreview, error) {
	resp := &ServiceTaskPreview{
		Workflows: make([]*Preview, 0),
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
		ProductName:   productName,
		Type:          serviceType,
		ExcludeStatus: setting.ProductStatusDeleting,
	}
	service, err := commonrepo.NewServiceColl().Find(serviceOpt)
	if err != nil {
		log.Errorf("ServiceTmpl.Find failed, productName:%s, serviceName:%s, serviceType:%s, err:%v", productName, serviceName, serviceType, err)
		return resp, e.ErrGetService.AddErr(err)
	}

	if service.Visibility != setting.PublicService && service.ProductName != productName {
		log.Errorf("Find service failed, productName:%s, serviceName:%s, serviceType:%s", productName, serviceName, serviceType)
		return resp, e.ErrGetService
	}

	// 获取支持升级此服务的产品工作流
	workflowOpt := &commonrepo.ListWorkflowOption{Projects: []string{productName}}
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
		workflowPreview := &Preview{Name: workflow.Name, WorkflowType: config.WorkflowType, DisplayName: workflow.DisplayName}
		resp.Workflows = append(resp.Workflows, workflowPreview)
	}

	// 如果是k8s服务，还需要获取支持更新此服务的单服务工作流
	if serviceType == setting.K8SDeployType {
		pipelineOpt := &commonrepo.PipelineListOption{
			ProductName: productName,
		}
		pipelines, err := commonrepo.NewPipelineColl().List(pipelineOpt)
		if err != nil {
			log.Errorf("Pipeline.List failed, productName:%s, serviceName:%s, serviceType:%s, err:%v", productName, serviceName, serviceType, err)
			return resp, e.ErrListPipeline.AddErr(err)
		}
		for _, p := range pipelines {
			// 单服务工作的部署的服务只会有一个，不需考虑重复插入的问题
			for _, subTask := range p.SubTasks {
				pre, err := base.ToPreview(subTask)
				if err != nil {
					log.Errorf("subTask.ToPreview error: %v", err)
					continue
				}
				if pre.TaskType != config.TaskDeploy {
					continue
				}
				deploy, err := base.ToDeployTask(subTask)
				if err != nil || deploy == nil {
					log.Errorf("subTask.ToDeployTask error: %v", err)
					continue
				}
				if deploy.Enabled && deploy.Namespace == prod.Namespace && deploy.ServiceName == serviceName {
					workflowPreview := &Preview{Name: p.Name, WorkflowType: config.SingleType}
					resp.Workflows = append(resp.Workflows, workflowPreview)
				}
			}
		}
	}

	allModules, err := ListBuildDetail("", "", log)
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
			moBuild, _ := findModuleByTargetAndVersion(allModules, serviceModuleTarget)
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
		moBuild, _ := findModuleByTargetAndVersion(allModules, serviceModuleTarget)
		if moBuild != nil {
			resp.Targets = append(resp.Targets, target)
		}
	}

	return resp, nil
}

type CreateTaskResp struct {
	ProjectName  string `json:"project_name"`
	PipelineName string `json:"pipeline_name"`
	TaskID       int64  `json:"task_id"`
}

func CreateServiceTask(args *commonmodels.ServiceTaskArgs, log *zap.SugaredLogger) ([]*CreateTaskResp, error) {
	if args.BuildName == "" && args.Revision == 0 {
		return nil, fmt.Errorf("服务[%s]的构建名称和服务版本必须有一个存在", args.ServiceName)
	} else if args.BuildName == "" && args.Revision > 0 {
		serviceTmpl, err := commonservice.GetServiceTemplate(
			args.ServiceName, setting.PMDeployType, args.ProductName, setting.ProductStatusDeleting, args.Revision, log,
		)
		if err != nil {
			return nil, fmt.Errorf("GetServiceTemplate servicename:%s revision:%d err:%v ", args.ServiceName, args.Revision, err)
		}
		args.BuildName = serviceTmpl.BuildName
		if args.BuildName == "" {
			return nil, fmt.Errorf("未找到服务[%s]版本对应的构建", args.ServiceName)
		}
	}
	// 获取全局configpayload
	configPayload := commonservice.GetConfigPayload(0)

	defaultS3, err := s3.FindDefaultS3()
	if err != nil {
		err = e.ErrFindDefaultS3Storage.AddDesc("default storage is required by distribute task")
		return nil, err
	}

	defaultURL, err := defaultS3.GetEncryptedURL()
	if err != nil {
		err = e.ErrS3Storage.AddErr(err)
		return nil, err
	}

	stages := make([]*commonmodels.Stage, 0)
	buildModuleArgs := &commonmodels.BuildModuleArgs{
		Target:      args.ServiceName,
		ServiceName: args.ServiceName,
		ProductName: args.ProductName,
	}
	subTasks, err := BuildModuleToSubTasks(buildModuleArgs, log)
	if err != nil {
		return nil, e.ErrCreateTask.AddErr(err)
	}

	task := &taskmodels.Task{
		Type:          config.ServiceType,
		ProductName:   args.ProductName,
		TaskCreator:   args.ServiceTaskCreator,
		Status:        config.StatusCreated,
		SubTasks:      subTasks,
		TaskArgs:      serviceTaskArgsToTaskArgs(args),
		ConfigPayload: configPayload,
		StorageURI:    defaultURL,
	}
	sort.Sort(ByTaskKind(task.SubTasks))

	if err := ensurePipelineTask(&taskmodels.TaskOpt{
		Task: task,
	}, log); err != nil {
		log.Errorf("CreateServiceTask ensurePipelineTask err : %v", err)
		return nil, err
	}

	for _, subTask := range task.SubTasks {
		AddSubtaskToStage(&stages, subTask, args.ServiceName)
	}
	sort.Sort(ByStageKind(stages))
	task.Stages = stages
	if len(task.Stages) == 0 {
		return nil, e.ErrCreateTask.AddDesc(e.PipelineSubTaskNotFoundErrMsg)
	}

	createTaskResps := make([]*CreateTaskResp, 0)
	for _, envName := range args.EnvNames {
		pipelineName := fmt.Sprintf("%s-%s-%s", args.ServiceName, envName, "job")

		nextTaskID, err := commonrepo.NewCounterColl().GetNextSeq(fmt.Sprintf(setting.ServiceTaskFmt, pipelineName))
		if err != nil {
			log.Errorf("CreateServiceTask Counter.GetNextSeq error: %v", err)
			return nil, e.ErrGetCounter.AddDesc(err.Error())
		}

		product, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
			Name:    args.ProductName,
			EnvName: envName,
		})
		if err != nil {
			return nil, e.ErrCreateTask.AddDesc(
				fmt.Sprintf("找不到 %s 的 %s 环境 ", args.ProductName, args.Namespace),
			)
		}

		args.Namespace = envName
		args.K8sNamespace = product.Namespace

		task.TaskID = nextTaskID
		task.ServiceTaskArgs = args
		task.SubTasks = []map[string]interface{}{}
		task.PipelineName = pipelineName

		if err := CreateTask(task); err != nil {
			log.Error(err)
			return nil, e.ErrCreateTask
		}

		createTaskResps = append(createTaskResps, &CreateTaskResp{ProjectName: product.ProductName, PipelineName: pipelineName, TaskID: nextTaskID})
	}

	return createTaskResps, nil
}

func serviceTaskArgsToTaskArgs(serviceTaskArgs *commonmodels.ServiceTaskArgs) *commonmodels.TaskArgs {
	resp := &commonmodels.TaskArgs{ProductName: serviceTaskArgs.ProductName, TaskCreator: serviceTaskArgs.ServiceTaskCreator}
	opt := &commonrepo.BuildFindOption{
		Name:        serviceTaskArgs.BuildName,
		ProductName: serviceTaskArgs.ProductName,
	}
	buildObj, err := commonrepo.NewBuildColl().Find(opt)
	if err != nil {
		resp.Builds = make([]*types.Repository, 0)
		return resp
	}
	resp.Builds = commonservice.FindReposByTarget(serviceTaskArgs.ProductName, serviceTaskArgs.ServiceName, serviceTaskArgs.ServiceName, buildObj)
	return resp
}
