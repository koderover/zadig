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
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"

	configbase "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/task"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/base"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/s3"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/scmnotify"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/poetry"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/types"
	"github.com/koderover/zadig/pkg/types/permission"
	"github.com/koderover/zadig/pkg/util"
)

const (
	ClusterStorageEP = "nfs-server"
)

type CronjobWorkflowArgs struct {
	Target []*commonmodels.TargetArgs `bson:"targets"                      json:"targets"`
}

// GetWorkflowArgs 返回工作流详细信息
func GetWorkflowArgs(productName, namespace string, log *zap.SugaredLogger) (*CronjobWorkflowArgs, error) {
	resp := &CronjobWorkflowArgs{}
	opt := &commonrepo.ProductFindOptions{Name: productName, EnvName: namespace}
	product, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		log.Errorf("Product.Find error: %v", err)
		return resp, e.ErrFindProduct.AddDesc(err.Error())
	}

	allModules, err := ListBuildDetail("", "", "", log)
	if err != nil {
		log.Errorf("BuildModule.List error: %v", err)
		return resp, e.ErrListBuildModule.AddDesc(err.Error())
	}

	targetMap := getProductTargetMap(product)
	productTemplMap := getProductTemplTargetMap(product.ProductName)
	targets := make([]*commonmodels.TargetArgs, 0)
	for container := range productTemplMap {
		if _, ok := targetMap[container]; !ok {
			continue
		}
		containerArr := strings.Split(container, SplitSymbol)
		if len(containerArr) != 3 {
			continue
		}
		target := &commonmodels.TargetArgs{Name: containerArr[2], ServiceName: containerArr[1], Deploy: targetMap[container], Build: &commonmodels.BuildArgs{}, HasBuild: true}

		moBuild := findModuleByTargetAndVersion(allModules, container, setting.Version)
		if moBuild == nil {
			moBuild = &commonmodels.Build{}
			target.HasBuild = false
		}
		target.Version = setting.Version

		if len(moBuild.Repos) == 0 {
			target.Build.Repos = make([]*types.Repository, 0)
		} else {
			target.Build.Repos = moBuild.Repos
		}

		if moBuild.PreBuild != nil {
			EnsureBuildResp(moBuild)
			target.Envs = moBuild.PreBuild.Envs
		}

		if moBuild.JenkinsBuild != nil {
			jenkinsBuildParams := make([]*commonmodels.JenkinsBuildParam, 0)
			for _, jenkinsBuildParam := range moBuild.JenkinsBuild.JenkinsBuildParam {
				jenkinsBuildParams = append(jenkinsBuildParams, &commonmodels.JenkinsBuildParam{
					Name:  jenkinsBuildParam.Name,
					Value: jenkinsBuildParam.Value,
				})
			}
			target.JenkinsBuildArgs = &commonmodels.JenkinsBuildArgs{
				JobName:            moBuild.JenkinsBuild.JobName,
				JenkinsBuildParams: jenkinsBuildParams,
			}
		}

		targets = append(targets, target)
	}
	resp.Target = targets
	return resp, nil
}

func getProductTargetMap(prod *commonmodels.Product) map[string][]commonmodels.DeployEnv {
	resp := make(map[string][]commonmodels.DeployEnv)
	for _, services := range prod.Services {
		for _, serviceObj := range services {
			switch serviceObj.Type {
			case setting.K8SDeployType:
				for _, container := range serviceObj.Containers {
					env := serviceObj.ServiceName + "/" + container.Name
					deployEnv := commonmodels.DeployEnv{Type: setting.K8SDeployType, Env: env}
					target := fmt.Sprintf("%s%s%s%s%s", prod.ProductName, SplitSymbol, serviceObj.ServiceName, SplitSymbol, container.Name)
					resp[target] = append(resp[target], deployEnv)
				}
			case setting.PMDeployType:
				deployEnv := commonmodels.DeployEnv{Type: setting.PMDeployType, Env: serviceObj.ServiceName}
				target := fmt.Sprintf("%s%s%s%s%s", prod.ProductName, SplitSymbol, serviceObj.ServiceName, SplitSymbol, serviceObj.ServiceName)
				resp[target] = append(resp[target], deployEnv)
			case setting.HelmDeployType:
				for _, container := range serviceObj.Containers {
					env := serviceObj.ServiceName + "/" + container.Name
					deployEnv := commonmodels.DeployEnv{Type: setting.HelmDeployType, Env: env}
					target := fmt.Sprintf("%s%s%s%s%s", prod.ProductName, SplitSymbol, serviceObj.ServiceName, SplitSymbol, container.Name)
					resp[target] = append(resp[target], deployEnv)
				}
			}
		}
	}
	return resp
}

func getProductTemplTargetMap(productName string) map[string][]commonmodels.DeployEnv {
	targets := make(map[string][]commonmodels.DeployEnv)
	productTmpl, err := template.NewProductColl().Find(productName)
	if err != nil {
		log.Errorf("[%s] ProductTmpl.Find error: %v", productName, err)
		return targets
	}
	var svcNames []string
	for _, ss := range productTmpl.Services {
		svcNames = append(svcNames, ss...)
	}
	services, err := commonrepo.NewServiceColl().ListMaxRevisionsForServices(svcNames, "")
	if err != nil {
		log.Errorf("ServiceTmpl.ListMaxRevisions error: %v", err)
		return targets
	}

	for _, serviceTmpl := range services {
		switch serviceTmpl.Type {
		case setting.K8SDeployType:
			for _, container := range serviceTmpl.Containers {
				deployEnv := commonmodels.DeployEnv{Env: serviceTmpl.ServiceName + "/" + container.Name, Type: setting.K8SDeployType, ProductName: serviceTmpl.ProductName}
				target := fmt.Sprintf("%s%s%s%s%s", productTmpl.ProductName, SplitSymbol, serviceTmpl.ServiceName, SplitSymbol, container.Name)
				targets[target] = append(targets[target], deployEnv)
			}
		case setting.PMDeployType:
			deployEnv := commonmodels.DeployEnv{Env: serviceTmpl.ServiceName, Type: setting.PMDeployType}
			target := fmt.Sprintf("%s%s%s%s%s", productTmpl.ProductName, SplitSymbol, serviceTmpl.ServiceName, SplitSymbol, serviceTmpl.ServiceName)
			targets[target] = append(targets[target], deployEnv)
		case setting.HelmDeployType:
			for _, container := range serviceTmpl.Containers {
				deployEnv := commonmodels.DeployEnv{Env: serviceTmpl.ServiceName + "/" + container.Name, Type: setting.HelmDeployType, ProductName: serviceTmpl.ProductName}
				target := fmt.Sprintf("%s%s%s%s%s", productTmpl.ProductName, SplitSymbol, serviceTmpl.ServiceName, SplitSymbol, container.Name)
				targets[target] = append(targets[target], deployEnv)
			}
		}
	}

	return targets
}

func findModuleByTargetAndVersion(allModules []*commonmodels.Build, serviceModuleTarget string, version string) *commonmodels.Build {
	containerArr := strings.Split(serviceModuleTarget, SplitSymbol)
	if len(containerArr) != 3 {
		return nil
	}

	opt := &commonrepo.ServiceFindOption{
		ServiceName:   containerArr[1],
		ExcludeStatus: setting.ProductStatusDeleting,
	}
	serviceObj, _ := commonrepo.NewServiceColl().Find(opt)
	if serviceObj != nil && serviceObj.Visibility == setting.PUBLICSERVICE {
		containerArr[0] = serviceObj.ProductName
	}
	for _, mo := range allModules {
		if mo.Version != version {
			continue
		}
		for _, target := range mo.Targets {
			targetStr := fmt.Sprintf("%s%s%s%s%s", target.ProductName, SplitSymbol, target.ServiceName, SplitSymbol, target.ServiceModule)
			if targetStr == strings.Join(containerArr, SplitSymbol) {
				return mo
			}
		}
	}
	return nil
}

func EnsureBuildResp(mb *commonmodels.Build) {
	if len(mb.Targets) == 0 {
		mb.Targets = make([]*commonmodels.ServiceModuleTarget, 0)
	}

	if len(mb.Repos) == 0 {
		mb.Repos = make([]*types.Repository, 0)
	}

	if mb.PreBuild != nil {
		if len(mb.PreBuild.Installs) == 0 {
			mb.PreBuild.Installs = make([]*commonmodels.Item, 0)
		}

		if len(mb.PreBuild.Envs) == 0 {
			mb.PreBuild.Envs = make([]*commonmodels.KeyVal, 0)
		}

		// 隐藏用户设置的敏感信息
		for k := range mb.PreBuild.Envs {
			if mb.PreBuild.Envs[k].IsCredential {
				mb.PreBuild.Envs[k].Value = setting.MaskValue
			}
		}

		if len(mb.PreBuild.Parameters) == 0 {
			mb.PreBuild.Parameters = make([]*commonmodels.Parameter, 0)
		}
	}
}

func ListBuildDetail(name, version, targets string, log *zap.SugaredLogger) ([]*commonmodels.Build, error) {
	opt := &commonrepo.BuildListOption{
		Name:    name,
		Version: version,
	}

	if len(strings.TrimSpace(targets)) != 0 {
		opt.Targets = strings.Split(targets, ",")
	}

	resp, err := commonrepo.NewBuildColl().List(opt)
	if err != nil {
		log.Errorf("[Build.List] %s:%s error: %v", name, version, err)
		return nil, e.ErrListBuildModule.AddErr(err)
	}

	return resp, nil
}

// PresetWorkflowArgs 返回工作流详细信息
func PresetWorkflowArgs(namespace, workflowName string, log *zap.SugaredLogger) (*commonmodels.WorkflowTaskArgs, error) {
	resp := &commonmodels.WorkflowTaskArgs{Namespace: namespace, WorkflowName: workflowName}
	workflow, err := commonrepo.NewWorkflowColl().Find(workflowName)
	if err != nil {
		log.Errorf("Workflow.Find error: %v", err)
		return resp, e.ErrFindWorkflow.AddDesc(err.Error())
	}
	if workflow.DistributeStage != nil {
		resp.DistributeEnabled = workflow.DistributeStage.Enabled
	}
	resp.ProductTmplName = workflow.ProductTmplName
	opt := &commonrepo.ProductFindOptions{Name: workflow.ProductTmplName, EnvName: namespace}
	product, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		log.Errorf("Product.Find error: %v", err)
		return resp, e.ErrFindProduct.AddDesc(err.Error())
	}

	allModules, err := ListBuildDetail("", "", "", log)
	if err != nil {
		log.Errorf("BuildModule.List error: %v", err)
		return resp, e.ErrListBuildModule.AddDesc(err.Error())
	}

	allTestings, err := commonrepo.NewTestingColl().List(&commonrepo.ListTestOption{ProductName: "", TestType: ""})
	if err != nil {
		log.Errorf("TestingModule.List error: %v", err)
		return resp, e.ErrListTestModule.AddDesc(err.Error())
	}

	targetMap := getProductTargetMap(product)
	productTemplMap := getProductTemplTargetMap(product.ProductName)
	targets := make([]*commonmodels.TargetArgs, 0)
	if (workflow.BuildStage != nil && workflow.BuildStage.Enabled) || (workflow.ArtifactStage != nil && workflow.ArtifactStage.Enabled) {
		for container := range productTemplMap {
			if _, ok := targetMap[container]; !ok {
				continue
			}

			containerArr := strings.Split(container, SplitSymbol)
			if len(containerArr) != 3 {
				continue
			}
			target := &commonmodels.TargetArgs{Name: containerArr[2], ServiceName: containerArr[1], Deploy: targetMap[container], Build: &commonmodels.BuildArgs{}, HasBuild: true}
			moBuild := findModuleByTargetAndVersion(allModules, container, setting.Version)
			if moBuild == nil {
				moBuild = &commonmodels.Build{}
				target.HasBuild = false
			}
			target.Version = setting.Version

			if len(moBuild.Repos) == 0 {
				target.Build.Repos = make([]*types.Repository, 0)
			} else {
				target.Build.Repos = moBuild.Repos
			}

			if moBuild.PreBuild != nil {
				EnsureBuildResp(moBuild)
				target.Envs = moBuild.PreBuild.Envs
			}

			if moBuild.JenkinsBuild != nil {
				jenkinsBuildParams := make([]*commonmodels.JenkinsBuildParam, 0)
				for _, jenkinsBuildParam := range moBuild.JenkinsBuild.JenkinsBuildParam {
					jenkinsBuildParams = append(jenkinsBuildParams, &commonmodels.JenkinsBuildParam{
						Name:  jenkinsBuildParam.Name,
						Value: jenkinsBuildParam.Value,
					})
				}
				target.JenkinsBuildArgs = &commonmodels.JenkinsBuildArgs{
					JobName:            moBuild.JenkinsBuild.JobName,
					JenkinsBuildParams: jenkinsBuildParams,
				}

			}

			targets = append(targets, target)
		}
	}

	resp.Target = targets
	testArgs := make([]*commonmodels.TestArgs, 0)
	testsMap := make(map[string]*commonmodels.Testing)
	if workflow.TestStage != nil && workflow.TestStage.Enabled {
		for _, testing := range allTestings {
			testsMap[testing.Name] = testing
		}
		for _, workflowTestArgs := range workflow.TestStage.Tests {
			if test, ok := testsMap[workflowTestArgs.Name]; ok {
				testArg := &commonmodels.TestArgs{Namespace: namespace}
				if len(test.Repos) == 0 {
					testArg.Builds = make([]*types.Repository, 0)
				} else {
					testArg.Builds = test.Repos
				}
				envKeyMap := make(map[string]string)
				testArg.Envs = workflowTestArgs.Envs
				for _, env := range workflowTestArgs.Envs {
					envKeyMap[env.Key] = env.Value
				}

				for _, moduleEnv := range test.PreTest.Envs {
					if _, ok := envKeyMap[moduleEnv.Key]; !ok {
						testArg.Envs = append(testArg.Envs, &commonmodels.KeyVal{
							Key:          moduleEnv.Key,
							Value:        moduleEnv.Value,
							IsCredential: moduleEnv.IsCredential,
						})
					}
				}
				testArg.TestModuleName = workflowTestArgs.Name
				testArgs = append(testArgs, testArg)
			}
		}

		for _, testName := range workflow.TestStage.TestNames {
			if test, ok := testsMap[testName]; ok {
				EnsureTestingResp(test)
				testArg := &commonmodels.TestArgs{Namespace: namespace}
				if len(test.Repos) == 0 {
					testArg.Builds = make([]*types.Repository, 0)
				} else {
					testArg.Builds = test.Repos
				}

				if test.PreTest != nil {
					testArg.Envs = test.PreTest.Envs
				}

				testArg.TestModuleName = testName
				testArgs = append(testArgs, testArg)
			}
		}
	}
	resp.Tests = testArgs
	return resp, nil
}

func CreateWorkflowTask(args *commonmodels.WorkflowTaskArgs, taskCreator string, userID int, superUser bool, log *zap.SugaredLogger) (*CreateTaskResp, error) {
	if args == nil {
		return nil, fmt.Errorf("args should not be nil")
	}

	// RequestMode=openAPI表示外部客户调用API，部分数据需要获取并补充到args中
	if args.RequestMode == "openAPI" {
		log.Info("CreateWorkflowTask from openAPI")
		err := AddDataToArgs(args, log)
		if err != nil {
			log.Errorf("AddDataToArgs error: %v", err)
			return nil, err
		}
	}

	productTempl, err := template.NewProductColl().Find(args.ProductTmplName)
	if err != nil {
		log.Errorf("productTempl.Find error: %v", err)
		return nil, e.ErrFindWorkflow.AddDesc(err.Error())
	}

	workflow, err := commonrepo.NewWorkflowColl().Find(args.WorkflowName)
	if err != nil {
		log.Errorf("Workflow.Find error: %v", err)
		return nil, e.ErrFindWorkflow.AddDesc(err.Error())
	}
	args.IsParallel = workflow.IsParallel

	if !HasPermission(workflow.ProductTmplName, workflow.EnvName, args, userID, superUser, log) {
		log.Warnf("该工作流[%s]绑定的环境您没有权限,用户[%s]不能执行该工作流!", workflow.Name, taskCreator)
		return nil, e.ErrCreateTask.AddDesc("该工作流绑定的环境您没有更新环境或者环境管理权限,不能执行该工作流!")
	}

	var env *commonmodels.Product
	if args.Namespace != "" {
		// 处理namespace，避免开头或者结尾出现多余的逗号
		dealWithNamespace(args)
		namespaces := strings.Split(args.Namespace, ",")
		//webhook触发的情况处理
		if len(namespaces) > 1 {
			getNotBusyEnv(args, log)
		}
		env, err = commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
			Name:    args.ProductTmplName,
			EnvName: args.Namespace,
		})

		if err != nil {
			log.Errorf("找不到 项目:[%s]的环境:[%s]", args.ProductTmplName, args.Namespace)
			return nil, e.ErrCreateTask.AddDesc(
				fmt.Sprintf("找不到 %s 的 %s 环境 ", args.ProductTmplName, args.Namespace),
			)
		}
	}

	nextTaskID, err := commonrepo.NewCounterColl().GetNextSeq(fmt.Sprintf(setting.WorkflowTaskFmt, args.WorkflowName))
	if err != nil {
		log.Errorf("Counter.GetNextSeq error: %v", err)
		return nil, e.ErrGetCounter.AddDesc(err.Error())
	}

	// 获取全局configpayload
	configPayload := commonservice.GetConfigPayload(args.CodehostID)
	repos, err := commonrepo.NewRegistryNamespaceColl().FindAll(&commonrepo.FindRegOps{})
	if err == nil {
		configPayload.RepoConfigs = make(map[string]*commonmodels.RegistryNamespace)
		for _, repo := range repos {
			// if the registry is SWR, we need to modify ak/sk according to the rule
			if repo.RegProvider == config.SWRProvider {
				ak := fmt.Sprintf("%s@%s", repo.Region, repo.AccessKey)
				sk := util.ComputeHmacSha256(repo.AccessKey, repo.SecretKey)
				repo.AccessKey = ak
				repo.SecretKey = sk
			}
			configPayload.RepoConfigs[repo.ID.Hex()] = repo
		}
	}

	configPayload.IgnoreCache = args.IgnoreCache
	configPayload.ResetCache = args.ResetCache

	distributeS3StoreURL, defaultS3StoreURL, err := getDefaultAndDestS3StoreURL(workflow, log)
	if err != nil {
		log.Errorf("getDefaultAndDestS3StoreUrl workflow name:[%s] err:%v", workflow.Name, err)
		return nil, e.ErrCreateTask.AddErr(err)
	}

	stages := make([]*commonmodels.Stage, 0)
	for _, target := range args.Target {
		var subTasks []map[string]interface{}
		var err error
		// should always be stable
		target.Version = "stable"
		if target.JenkinsBuildArgs == nil {
			subTasks, err = BuildModuleToSubTasks("", target.Version, target.Name, target.ServiceName, args.ProductTmplName, target.Envs, env, log)
		} else {
			subTasks, err = JenkinsBuildModuleToSubTasks(&JenkinsBuildOption{
				Version:          target.Version,
				Target:           target.Name,
				ServiceName:      target.ServiceName,
				ProductName:      args.ProductTmplName,
				JenkinsBuildArgs: target.JenkinsBuildArgs,
			}, log)
		}
		if err != nil {
			log.Errorf("buildModuleToSubTasks target:[%s] err:%v", target.Name, err)
			return nil, e.ErrCreateTask.AddErr(err)
		}

		if env != nil {
			// 生成部署的subtask
			for _, deployEnv := range target.Deploy {
				if deployEnv.Type == setting.PMDeployType {
					continue
				}
				deployTask, err := deployEnvToSubTasks(deployEnv, env, productTempl.Timeout)
				if err != nil {
					log.Errorf("deploy env to subtask error: %v", err)
					return nil, e.ErrCreateTask.AddErr(err)
				}

				if workflow.ResetImage {
					resetImageTask, err := resetImageTaskToSubTask(deployEnv, env)
					if err != nil {
						log.Errorf("resetImageTaskToSubTask deploy env:[%s] err:%v ", deployEnv.Env, err)
						return nil, e.ErrCreateTask.AddErr(err)
					}
					if resetImageTask != nil {
						subTasks = append(subTasks, resetImageTask)
					}
				}

				subTasks = append(subTasks, deployTask)
			}
		}

		// 生成分发的subtask
		if workflow.DistributeStage != nil && workflow.DistributeStage.Enabled {
			var distributeTasks []map[string]interface{}
			var err error

			for _, distribute := range workflow.DistributeStage.Distributes {
				serviceModule := &commonmodels.ServiceModuleTarget{
					ProductName:   args.ProductTmplName,
					ServiceName:   target.ServiceName,
					ServiceModule: target.Name,
				}
				if distribute.Target != nil {
					serviceModule.ProductName = distribute.Target.ProductName
				}
				if reflect.DeepEqual(distribute.Target, serviceModule) {
					distributeTasks, err = formatDistributeSubtasks(
						workflow.DistributeStage.Releases,
						workflow.DistributeStage.ImageRepo,
						workflow.DistributeStage.JumpBoxHost,
						distributeS3StoreURL,
						distribute,
					)
					if err != nil {
						log.Errorf("distrbiute stages to subtasks error: %v", err)
						return nil, e.ErrCreateTask.AddErr(err)
					}
				}
			}
			subTasks = append(subTasks, distributeTasks...)
		}

		jiraInfo, _ := poetry.GetJiraInfo(config.PoetryAPIServer(), config.PoetryAPIRootKey())
		if jiraInfo != nil {
			jiraTask, err := AddJiraSubTask("", target.Version, target.Name, target.ServiceName, args.ProductTmplName, log)
			if err != nil {
				log.Errorf("add jira task error: %v", err)
				return nil, e.ErrCreateTask.AddErr(fmt.Errorf("add jira task error: %v", err))
			}
			subTasks = append(subTasks, jiraTask)
		}

		if workflow.SecurityStage != nil && workflow.SecurityStage.Enabled {
			securityTask, err := addSecurityToSubTasks()
			if err != nil {
				log.Errorf("add security task error: %v", err)
				return nil, e.ErrCreateTask.AddErr(err)
			}
			if _, err := commonservice.GetServiceTemplate(
				target.Name, setting.PMDeployType, args.ProductTmplName, setting.ProductStatusDeleting, 0, log,
			); err != nil {
				subTasks = append(subTasks, securityTask)
			}
		}

		// 填充subtask之间关联内容
		task := &task.Task{
			TaskID:        nextTaskID,
			PipelineName:  args.WorkflowName,
			TaskCreator:   taskCreator,
			ReqID:         args.ReqID,
			SubTasks:      subTasks,
			ServiceName:   target.Name,
			TaskArgs:      workFlowArgsToTaskArgs(target.Name, args),
			ConfigPayload: configPayload,
			ProductName:   args.ProductTmplName,
		}
		sort.Sort(ByTaskKind(task.SubTasks))

		if err := ensurePipelineTask(task, log); err != nil {
			log.Errorf("workflow_task ensurePipelineTask taskID:[%d] pipelineName:[%s] err:%v", task.ID, task.PipelineName, err)
			if err, ok := err.(*ContainerNotFound); ok {
				err := e.NewWithExtras(
					e.ErrCreateTaskFailed.AddErr(err),
					"container doesn't exists", map[string]interface{}{
						"productName":   err.ProductName,
						"envName":       err.EnvName,
						"serviceName":   err.ServiceName,
						"containerName": err.Container,
					})
				return nil, err
			}
			if _, ok := err.(*ImageIllegal); ok {
				return nil, e.ErrCreateTask.AddDesc("IMAGE is illegal")
			}
			return nil, e.ErrCreateTask.AddDesc(err.Error())
		}

		for _, stask := range task.SubTasks {
			AddSubtaskToStage(&stages, stask, target.Name)
		}
	}

	testTask := &task.Task{
		TaskID:       nextTaskID,
		PipelineName: args.WorkflowName,
		ProductName:  args.ProductTmplName,
	}
	testTasks, err := testArgsToSubtask(args, testTask, log)
	if err != nil {
		log.Errorf("workflow_task testArgsToSubtask args:[%v] err:%v", args, err)
		return nil, e.ErrCreateTask.AddDesc(err.Error())
	}

	for _, testTask := range testTasks {
		FmtBuilds(testTask.JobCtx.Builds, log)
		testSubTask, err := testTask.ToSubTask()
		if err != nil {
			log.Errorf("workflow_task ToSubTask err:%v", err)
			return nil, e.ErrCreateTask.AddDesc(err.Error())
		}

		err = SetCandidateRegistry(configPayload, log)
		if err != nil {
			log.Errorf("workflow_task setCandidateRegistry configPayload:[%v] err:%v", configPayload, err)
			return nil, err
		}

		AddSubtaskToStage(&stages, testSubTask, testTask.TestModuleName)
	}

	sort.Sort(ByStageKind(stages))
	triggerBy := &commonmodels.TriggerBy{
		CodehostID:     args.CodehostID,
		RepoOwner:      args.RepoOwner,
		RepoName:       args.RepoName,
		Source:         args.Source,
		MergeRequestID: args.MergeRequestID,
		CommitID:       args.CommitID,
	}
	task := &task.Task{
		TaskID:        nextTaskID,
		Type:          config.WorkflowType,
		ProductName:   workflow.ProductTmplName,
		PipelineName:  args.WorkflowName,
		Description:   args.Description,
		TaskCreator:   taskCreator,
		ReqID:         args.ReqID,
		Status:        config.StatusCreated,
		Stages:        stages,
		WorkflowArgs:  args,
		ConfigPayload: configPayload,
		StorageURI:    defaultS3StoreURL,
		ResetImage:    workflow.ResetImage,
		TriggerBy:     triggerBy,
	}

	if len(task.Stages) <= 0 {
		return nil, e.ErrCreateTask.AddDesc(e.PipelineSubTaskNotFoundErrMsg)
	}

	endpoint := fmt.Sprintf("%s-%s:9000", config.Namespace(), ClusterStorageEP)

	task.StorageEndpoint = endpoint

	if env != nil {
		task.Services = env.Services
		task.Render = env.Render
		task.ConfigPayload.DeployClusterID = env.ClusterID
	}

	if config.EnableGitCheck() {
		if err := createGitCheck(task, log); err != nil {
			log.Errorf("workflow createGitCheck task:[%v] err:%v", task, err)
		}
	}

	if err := CreateTask(task); err != nil {
		log.Errorf("workflow Create task:[%v] err:%v", task, err)
		return nil, e.ErrCreateTask
	}

	_ = scmnotify.NewService().UpdateWebhookComment(task, log)
	resp := &CreateTaskResp{PipelineName: args.WorkflowName, TaskID: nextTaskID}
	return resp, nil
}

func AddDataToArgs(args *commonmodels.WorkflowTaskArgs, log *zap.SugaredLogger) error {
	if args.Target == nil || len(args.Target) == 0 {
		return errors.New("target cannot be empty")
	}

	builds, err := commonrepo.NewBuildColl().List(&commonrepo.BuildListOption{})
	if err != nil {
		log.Errorf("[Build.List] error: %v", err)
		return e.ErrListBuildModule.AddErr(err)
	}

	workflow, err := commonrepo.NewWorkflowColl().Find(args.WorkflowName)
	if err != nil {
		log.Errorf("[Workflow.Find] error: %v", err)
		return e.ErrFindWorkflow.AddErr(err)
	}

	args.ProductTmplName = workflow.ProductTmplName

	// 补全target信息
	for _, target := range args.Target {
		target.HasBuild = true
		// openAPI模式，传入的name是服务名称
		// 只支持k8s服务
		opt := &commonrepo.ServiceFindOption{ServiceName: target.Name, Type: setting.K8SDeployType, ExcludeStatus: setting.ProductStatusDeleting}
		serviceTmpl, err := commonrepo.NewServiceColl().Find(opt)
		if err != nil {
			log.Errorf("[ServiceTmpl.Find] error: %v", err)
			return e.ErrGetService.AddErr(err)
		}

		// 补全target中的deploy信息
		deploys := make([]commonmodels.DeployEnv, 0)
		for _, container := range serviceTmpl.Containers {
			// 如果服务中的服务组件存在构建，将该服务组件放进deploy
			for _, build := range builds {
				for _, moduleTarget := range build.Targets {
					serviceModuleTarget := &commonmodels.ServiceModuleTarget{
						ProductName:   serviceTmpl.ProductName,
						ServiceName:   serviceTmpl.ServiceName,
						ServiceModule: container.Name,
					}
					if reflect.DeepEqual(moduleTarget, serviceModuleTarget) {
						deployEnv := commonmodels.DeployEnv{Env: serviceTmpl.ServiceName + "/" + container.Name, Type: serviceTmpl.Type, ProductName: serviceTmpl.ProductName}
						deploys = append(deploys, deployEnv)
					}
				}
			}
		}
		target.Deploy = deploys

		// 补全target中的build信息
		for _, build := range builds {
			if len(build.Targets) == 0 {
				continue
			}
			// 如果该构建拥有的服务组件 包含 该服务的任一服务组件，则认为匹配成功
			match := false
			for _, container := range serviceTmpl.Containers {
				serviceModuleTarget := &commonmodels.ServiceModuleTarget{
					ProductName:   serviceTmpl.ProductName,
					ServiceName:   serviceTmpl.ServiceName,
					ServiceModule: container.Name,
				}
				for _, buildTarget := range build.Targets {
					if reflect.DeepEqual(buildTarget, serviceModuleTarget) {
						target.Name = container.Name
						match = true
						break
					}
				}
			}
			if !match {
				continue
			}
			// 服务组件匹配成功，则从该构建的仓库列表 匹配仓库名称，并添加repo信息
			if target.Build == nil {
				continue
			}
			for _, buildRepo := range build.Repos {
				for _, targetRepo := range target.Build.Repos {
					if targetRepo.RepoName == buildRepo.RepoName {
						// openAPI仅上传了repoName、branch、pr
						targetRepo.Source = buildRepo.Source
						targetRepo.RepoOwner = buildRepo.RepoOwner
						targetRepo.RemoteName = buildRepo.RemoteName
						targetRepo.CodehostID = buildRepo.CodehostID
						targetRepo.CheckoutPath = buildRepo.CheckoutPath
					}
				}
			}
		}
	}
	// 补全test信息
	if workflow.TestStage != nil && workflow.TestStage.Enabled {
		tests := make([]*commonmodels.TestArgs, 0)
		for _, testName := range workflow.TestStage.TestNames {
			moduleTest, err := commonrepo.NewTestingColl().Find(testName, "")
			if err != nil {
				log.Errorf("[Testing.Find] TestModuleName:%s, error:%v", testName, err)
				continue
			}
			test := &commonmodels.TestArgs{
				TestModuleName: testName,
				Namespace:      args.Namespace,
				Builds:         moduleTest.Repos,
			}
			tests = append(tests, test)
		}

		for _, testEntity := range workflow.TestStage.Tests {
			moduleTest, err := commonrepo.NewTestingColl().Find(testEntity.Name, "")
			if err != nil {
				log.Errorf("[Testing.Find] TestModuleName:%s, error:%v", testEntity.Name, err)
				continue
			}
			test := &commonmodels.TestArgs{
				TestModuleName: testEntity.Name,
				Namespace:      args.Namespace,
				Builds:         moduleTest.Repos,
			}
			tests = append(tests, test)
		}
		args.Tests = tests
	}

	return nil
}

func HasPermission(productName, envName string, args *commonmodels.WorkflowTaskArgs, userID int, superUser bool, log *zap.SugaredLogger) bool {
	//排除触发器触发和工作流没有绑定环境的情况
	if userID == permission.AnonymousUserID || envName == "" {
		return true
	}
	// 只有测试任务的情况
	if len(args.Target) == 0 && len(args.Tests) > 0 {
		return true
	}
	//权限判断
	if superUser {
		return true
	}
	poetryClient := poetry.New(config.PoetryAPIServer(), config.PoetryAPIRootKey())
	productNameMap, err := poetryClient.GetUserProject(userID, log)
	if err != nil {
		log.Errorf("Collection.Product.List GetUserProject error: %v", err)
		return false
	}
	//判断环境是类生产环境还是测试环境
	isProd := false
	opt := &commonrepo.ProductFindOptions{Name: productName, EnvName: envName}
	prod, _ := commonrepo.NewProductColl().Find(opt)
	if prod != nil && prod.ClusterID != "" {
		if cluster, _ := commonrepo.NewK8SClusterColl().Get(prod.ClusterID); cluster != nil {
			isProd = cluster.Production
		}
	}
	for _, roleID := range productNameMap[productName] {
		if roleID == poetry.ProjectOwner {
			return true
		}
		if envRolePermissions, _ := poetryClient.ListEnvRolePermission(productName, envName, roleID, log); len(envRolePermissions) > 0 {
			for _, envRolePermission := range envRolePermissions {
				if envRolePermission.PermissionUUID == permission.TestEnvManageUUID {
					return true
				}
			}
		} else {
			if !isProd {
				if poetryClient.HasOperatePermission(productName, permission.TestUpdateEnvUUID, userID, superUser, log) {
					return true
				}
				if poetryClient.HasOperatePermission(productName, permission.TestEnvManageUUID, userID, superUser, log) {
					return true
				}

				envRolePermissions, _ := poetryClient.ListEnvRolePermission(productName, envName, 0, log)
				for _, envRolePermission := range envRolePermissions {
					if envRolePermission.PermissionUUID == permission.TestEnvManageUUID {
						return true
					}
				}
			} else {
				if poetryClient.HasOperatePermission(productName, permission.ProdEnvManageUUID, userID, superUser, log) {
					return true
				}

				envRolePermissions, _ := poetryClient.ListEnvRolePermission(productName, envName, 0, log)
				for _, envRolePermission := range envRolePermissions {
					if envRolePermission.PermissionUUID == permission.ProdEnvManageUUID {
						return true
					}
				}
			}
		}
	}
	// 如果该项目设置过all-users,判断all-users的权限
	productRole, _ := poetryClient.ListRoles(productName, log)
	if productRole != nil {
		permissionUUIDs, _ := poetryClient.GetUserPermissionUUIDs(productRole.ID, productName, log)
		if isProd && sets.NewString(permissionUUIDs...).Has(permission.ProdEnvManageUUID) {
			return true
		} else if !isProd && sets.NewString(permissionUUIDs...).Has(permission.TestEnvManageUUID) {
			return true
		}
	}
	return false
}

func dealWithNamespace(args *commonmodels.WorkflowTaskArgs) {
	args.Namespace = strings.TrimPrefix(args.Namespace, ",")
	args.Namespace = strings.TrimSuffix(args.Namespace, ",")
}

var mutex sync.Mutex

func getNotBusyEnv(args *commonmodels.WorkflowTaskArgs, log *zap.SugaredLogger) {
	namespaces := strings.Split(args.Namespace, ",")
	opt := new(commonrepo.ListQueueOption)
	queueTasks, err := commonrepo.NewQueueColl().List(opt)
	if err != nil {
		log.Errorf("getNotBusyEnv pipelineQueue.list err: %v", err)
		args.Namespace = namespaces[0]
		return
	}

	if len(queueTasks) == 0 {
		args.Namespace = namespaces[0]
		return
	}

	mutex.Lock()
	defer func() {
		mutex.Unlock()
	}()

	sameProductQueueTasks := make(map[string][]*commonmodels.Queue)
	for _, t := range queueTasks {
		if t.Status != config.StatusRunning && t.Status != config.StatusQueued && t.Status != config.StatusBlocked {
			continue
		}
		if t.ProductName != args.ProductTmplName {
			continue
		}
		sameProductQueueTasks[t.WorkflowArgs.Namespace] = append(sameProductQueueTasks[t.WorkflowArgs.Namespace], t)
	}

	for _, namespace := range namespaces {
		if _, isExist := sameProductQueueTasks[namespace]; !isExist {
			args.Namespace = namespace
			return
		}
	}

	// 找到当前队列中排队最少的那个环境
	envTaskMap := make(map[int][]string)
	for _, namespace := range namespaces {
		envQueueNum := len(sameProductQueueTasks[namespace])
		envTaskMap[envQueueNum] = append(envTaskMap[envQueueNum], namespace)
	}

	keys := make([]int, 0)
	for key := range envTaskMap {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	minNamespaces := envTaskMap[keys[0]]
	args.Namespace = minNamespaces[0]
}

func getDefaultAndDestS3StoreURL(workflow *commonmodels.Workflow, log *zap.SugaredLogger) (destURL, defaultURL string, err error) {
	var distributeS3Store *commonmodels.S3Storage
	if workflow.DistributeStage != nil && workflow.DistributeStage.IsDistributeS3Enabled() && workflow.DistributeStage.S3StorageID != "" {
		distributeS3Store, err = GetS3Storage(workflow.DistributeStage.S3StorageID, log)
		if err != nil {
			msg := "failed to find s3 storage with id " + workflow.DistributeStage.S3StorageID
			log.Errorf(msg)
			err = e.ErrS3Storage.AddDesc(msg)
			return
		}
		defaultS3 := s3.S3{
			S3Storage: distributeS3Store,
		}

		destURL, err = defaultS3.GetEncryptedURL()
		if err != nil {
			return
		}
	}

	defaultS3, err := s3.FindDefaultS3()
	if err != nil {
		err = e.ErrFindDefaultS3Storage.AddDesc("default storage is required by distribute task")
		return
	}

	defaultURL, err = defaultS3.GetEncryptedURL()
	if err != nil {
		err = e.ErrS3Storage.AddErr(err)
		return
	}

	return
}

func GetS3Storage(id string, logger *zap.SugaredLogger) (*commonmodels.S3Storage, error) {
	store, err := commonrepo.NewS3StorageColl().Find(id)
	if err != nil {
		logger.Infof("can't find store by id %s", id)
		if err == mongo.ErrNoDocuments {
			err = e.ErrNotFound.AddDesc("not found")
		}

		return nil, err
	}

	return store, nil
}

func deployEnvToSubTasks(env commonmodels.DeployEnv, prodEnv *commonmodels.Product, timeout int) (map[string]interface{}, error) {
	var (
		resp       map[string]interface{}
		deployTask = task.Deploy{
			TaskType:    config.TaskDeploy,
			Enabled:     true,
			Namespace:   prodEnv.Namespace,
			ProductName: prodEnv.ProductName,
			EnvName:     prodEnv.EnvName,
			Timeout:     timeout,
		}
	)

	envList := strings.Split(env.Env, "/")
	if len(envList) != 2 {
		err := fmt.Errorf("[%s]split target env error", env.Env)
		log.Error(err)
		return nil, err
	}
	deployTask.ServiceName = envList[0]
	deployTask.ContainerName = envList[1]

	switch env.Type {
	case setting.K8SDeployType:
		deployTask.ServiceType = setting.K8SDeployType
		return deployTask.ToSubTask()
	case setting.HelmDeployType:
		deployTask.ServiceType = setting.HelmDeployType
		for _, services := range prodEnv.Services {
			for _, service := range services {
				if service.ServiceName == deployTask.ServiceName {
					deployTask.ServiceRevision = service.Revision
					return deployTask.ToSubTask()
				}
			}
		}
		return deployTask.ToSubTask()
	}
	return resp, fmt.Errorf("env type not match")
}

func resetImageTaskToSubTask(env commonmodels.DeployEnv, prodEnv *commonmodels.Product) (map[string]interface{}, error) {
	switch env.Type {
	case setting.K8SDeployType:
		deployTask := task.Deploy{TaskType: config.TaskResetImage, Enabled: true}
		deployTask.Namespace = prodEnv.Namespace
		deployTask.ProductName = prodEnv.ProductName
		deployTask.SkipWaiting = true
		deployTask.EnvName = prodEnv.EnvName
		envList := strings.Split(env.Env, "/")
		if len(envList) != 2 {
			err := fmt.Errorf("[%s]split target env error", env.Env)
			log.Error(err)
			return nil, err
		}
		deployTask.ServiceName = envList[0]
		deployTask.ContainerName = envList[1]
		return deployTask.ToSubTask()
	default:
		return nil, nil
	}
}

func artifactToSubTasks(name, image string) (map[string]interface{}, error) {
	artifactTask := task.Artifact{TaskType: config.TaskArtifact, Enabled: true}
	artifactTask.Name = name
	artifactTask.Image = image

	return artifactTask.ToSubTask()
}

func formatDistributeSubtasks(releaseImages []commonmodels.RepoImage, imageRepo, jumpboxHost, destStorageURL string, distribute *commonmodels.ProductDistribute) ([]map[string]interface{}, error) {
	var resp []map[string]interface{}

	if distribute.ImageDistribute {
		t := task.ReleaseImage{
			TaskType:  config.TaskReleaseImage,
			Enabled:   true,
			ImageRepo: imageRepo,
			Releases:  releaseImages,
		}
		subtask, err := t.ToSubTask()
		if err != nil {
			return resp, err
		}
		resp = append(resp, subtask)
	}
	if distribute.QstackDistribute && destStorageURL != "" {
		task := task.DistributeToS3{
			TaskType:       config.TaskDistributeToS3,
			Enabled:        true,
			DestStorageURL: destStorageURL,
			//SrcStorageUrl:  srcStorageUrl,
		}
		subtask, err := task.ToSubTask()
		if err != nil {
			return resp, err
		}
		resp = append(resp, subtask)
	}
	return resp, nil
}

func AddJiraSubTask(moduleName, version, target, serviceName, productName string, log *zap.SugaredLogger) (map[string]interface{}, error) {
	repos := make([]*types.Repository, 0)

	opt := &commonrepo.BuildListOption{
		Name:        moduleName,
		Version:     version,
		ServiceName: serviceName,
		ProductName: productName,
	}

	if len(target) > 0 {
		opt.Targets = []string{target}
	}

	modules, err := commonrepo.NewBuildColl().List(opt)
	if err != nil {
		return nil, e.ErrConvertSubTasks.AddErr(err)
	}
	jira := &task.Jira{
		TaskType: config.TaskJira,
		Enabled:  true,
	}
	for _, module := range modules {
		repos = append(repos, module.Repos...)
	}
	jira.Builds = repos
	return jira.ToSubTask()
}

func addSecurityToSubTasks() (map[string]interface{}, error) {
	securityTask := task.Security{TaskType: config.TaskSecurity, Enabled: true}
	return securityTask.ToSubTask()
}

func workFlowArgsToTaskArgs(target string, workflowArgs *commonmodels.WorkflowTaskArgs) *commonmodels.TaskArgs {
	resp := &commonmodels.TaskArgs{PipelineName: workflowArgs.WorkflowName, TaskCreator: workflowArgs.WorklowTaskCreator}
	for _, build := range workflowArgs.Target {
		if build.Name == target {
			if build.Build != nil {
				resp.Builds = build.Build.Repos
			}
		}
	}
	return resp
}

// TODO 和validation中转化testsubtask合并为一个方法
func testArgsToSubtask(args *commonmodels.WorkflowTaskArgs, pt *task.Task, log *zap.SugaredLogger) ([]*task.Testing, error) {
	var resp []*task.Testing

	// 创建任务的测试参数为脱敏数据，需要转换为实际数据
	for _, test := range args.Tests {
		existed, err := commonrepo.NewTestingColl().Find(test.TestModuleName, args.ProductTmplName)
		if err == nil && existed.PreTest != nil {
			commonservice.EnsureSecretEnvs(existed.PreTest.Envs, test.Envs)
		}
	}

	testArgs := args.Tests
	testCreator := args.WorklowTaskCreator

	registries, err := commonservice.ListRegistryNamespaces(log)
	if err != nil {
		log.Errorf("ListRegistryNamespaces err:%v", err)
	}

	for _, testArg := range testArgs {
		//if _, ok := legalTests[testArg.TestModuleName]; !ok {
		//	// filter illegal test names
		//	continue
		//}
		testModule, err := GetRaw(testArg.TestModuleName, "", log)
		if err != nil {
			log.Errorf("[%s]get TestingModule error: %v", testArg.TestModuleName, err)
			return resp, err
		}

		testTask := &task.Testing{
			TaskType: config.TaskTestingV2,
			Enabled:  true,
			TestName: "test",
			Timeout:  testModule.Timeout,
		}
		testTask.TestModuleName = testModule.Name
		testTask.JobCtx.TestType = testModule.TestType
		testTask.JobCtx.Builds = testModule.Repos
		testTask.JobCtx.BuildSteps = append(testTask.JobCtx.BuildSteps, &task.BuildStep{BuildType: "shell", Scripts: testModule.Scripts})

		testTask.JobCtx.ArtifactPaths = testModule.ArtifactPaths
		testTask.JobCtx.TestThreshold = testModule.Threshold
		testTask.JobCtx.Caches = testModule.Caches
		testTask.JobCtx.TestResultPath = testModule.TestResultPath
		testTask.JobCtx.TestReportPath = testModule.TestReportPath

		if testTask.Registries == nil {
			testTask.Registries = registries
		}

		if testModule.PreTest != nil {
			testTask.InstallItems = testModule.PreTest.Installs
			testTask.JobCtx.CleanWorkspace = testModule.PreTest.CleanWorkspace
			testTask.JobCtx.EnableProxy = testModule.PreTest.EnableProxy

			envs := testModule.PreTest.Envs[:]

			for _, env := range envs {
				for _, overwrite := range testArg.Envs {
					if overwrite.Key == env.Key {
						env.Value = overwrite.Value
						env.IsCredential = overwrite.IsCredential
						break
					}
				}
			}
			envs = append(envs, &commonmodels.KeyVal{Key: "TEST_URL", Value: GetLink(pt, configbase.SystemAddress(), config.WorkflowType)})
			testTask.JobCtx.EnvVars = envs
			testTask.ImageID = testModule.PreTest.ImageID
			testTask.BuildOS = testModule.PreTest.BuildOS
			testTask.ImageFrom = testModule.PreTest.ImageFrom
			// 自定义基础镜像的镜像名称可能会被更新，需要使用ID获取最新的镜像名称
			if testModule.PreTest.ImageID != "" {
				basicImage, err := commonrepo.NewBasicImageColl().Find(testModule.PreTest.ImageID)
				if err != nil {
					log.Errorf("BasicImage.Find failed, id:%s, err:%v", testModule.PreTest.ImageID, err)
				} else {
					testTask.BuildOS = basicImage.Value
				}
			}
			testTask.ResReq = testModule.PreTest.ResReq
		}
		// 设置 build 安装脚本
		testTask.InstallCtx, err = buildInstallCtx(testTask.InstallItems)
		if err != nil {
			log.Errorf("buildInstallCtx error: %v", err)
			return resp, err
		}

		// Iterate test jobctx builds, and replace it if params specified from task.
		// 外部触发的pipeline
		if testCreator == setting.WebhookTaskCreator || testCreator == setting.CronTaskCreator {
			_ = SetTriggerBuilds(testTask.JobCtx.Builds, testArg.Builds)
		} else {
			_ = setManunalBuilds(testTask.JobCtx.Builds, testArg.Builds)
		}

		resp = append(resp, testTask)
	}

	return resp, nil
}

func CreateArtifactWorkflowTask(args *commonmodels.WorkflowTaskArgs, taskCreator string, userID int, superUser bool, log *zap.SugaredLogger) (*CreateTaskResp, error) {
	if args == nil {
		return nil, fmt.Errorf("args should not be nil")
	}
	productTempl, err := template.NewProductColl().Find(args.ProductTmplName)
	if err != nil {
		log.Errorf("productTempl.Find error: %v", err)
		return nil, e.ErrFindWorkflow.AddDesc(err.Error())
	}

	workflow, err := commonrepo.NewWorkflowColl().Find(args.WorkflowName)
	if err != nil {
		log.Errorf("Workflow.Find error: %v", err)
		return nil, e.ErrFindWorkflow.AddDesc(err.Error())
	}

	if !HasPermission(workflow.ProductTmplName, workflow.EnvName, args, userID, superUser, log) {
		log.Warnf("该工作流[%s]绑定的环境您没有权限,用户[%s]不能执行该工作流!", workflow.Name, taskCreator)
		return nil, e.ErrCreateTask.AddDesc("该工作流绑定的环境您没有更新环境或者环境管理权限,不能执行该工作流")
	}

	var env *commonmodels.Product
	if args.Namespace != "" {
		// 查找要部署的环境
		env, err = commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
			Name:    args.ProductTmplName,
			EnvName: args.Namespace,
		})

		if err != nil {
			log.Errorf("找不到 项目:[%s]的环境:[%s]", args.ProductTmplName, args.Namespace)
			return nil, e.ErrCreateTask.AddDesc(
				fmt.Sprintf("找不到 %s 的 %s 环境 ", args.ProductTmplName, args.Namespace),
			)
		}
	}

	nextTaskID, err := commonrepo.NewCounterColl().GetNextSeq(fmt.Sprintf(setting.WorkflowTaskFmt, args.WorkflowName))
	if err != nil {
		log.Errorf("Counter.GetNextSeq error: %v", err)
		return nil, e.ErrGetCounter.AddDesc(err.Error())
	}

	// 获取全局configpayload
	configPayload := commonservice.GetConfigPayload(args.CodehostID)
	repos, err := commonrepo.NewRegistryNamespaceColl().FindAll(&commonrepo.FindRegOps{})
	if err == nil {
		configPayload.RepoConfigs = make(map[string]*commonmodels.RegistryNamespace)
		for _, repo := range repos {
			configPayload.RepoConfigs[repo.ID.Hex()] = repo
		}
	}

	configPayload.IgnoreCache = args.IgnoreCache
	configPayload.ResetCache = args.ResetCache

	distributeS3StoreURL, defaultS3StoreURL, err := getDefaultAndDestS3StoreURL(workflow, log)
	if err != nil {
		log.Errorf("getDefaultAndDestS3StoreUrl workflow name:[%s] err:%v", workflow.Name, err)
		return nil, err
	}

	stages := make([]*commonmodels.Stage, 0)
	for _, artifact := range args.Artifact {
		subTasks := make([]map[string]interface{}, 0)
		artifactSubtask, err := artifactToSubTasks(artifact.Name, artifact.Image)
		if err != nil {
			log.Errorf("artifactToSubTasks artifact.Name:[%s] err:%v", artifact.Name, err)
			return nil, e.ErrCreateTask.AddErr(err)
		}
		subTasks = append(subTasks, artifactSubtask)
		if env != nil {
			// 生成部署的subtask
			for _, deployEnv := range artifact.Deploy {
				deployTask, err := deployEnvToSubTasks(deployEnv, env, productTempl.Timeout)
				if err != nil {
					log.Errorf("deploy env to subtask error: %v", err)
					return nil, err
				}

				if workflow.ResetImage {
					resetImageTask, err := resetImageTaskToSubTask(deployEnv, env)
					if err != nil {
						log.Errorf("resetImageTaskToSubTask deploy env:[%s] err:%v ", deployEnv.Env, err)
						return nil, err
					}
					if resetImageTask != nil {
						subTasks = append(subTasks, resetImageTask)
					}
				}

				subTasks = append(subTasks, deployTask)
			}
		}

		// 生成分发的subtask
		if workflow.DistributeStage != nil && workflow.DistributeStage.Enabled {
			var distributeTasks []map[string]interface{}
			var err error

			for _, distribute := range workflow.DistributeStage.Distributes {
				serviceModule := &commonmodels.ServiceModuleTarget{
					ProductName:   args.ProductTmplName,
					ServiceName:   artifact.ServiceName,
					ServiceModule: artifact.Name,
				}
				if distribute.Target != nil {
					serviceModule.ProductName = distribute.Target.ProductName
				}
				if reflect.DeepEqual(distribute.Target, serviceModule) {
					distributeTasks, err = formatDistributeSubtasks(
						workflow.DistributeStage.Releases,
						workflow.DistributeStage.ImageRepo,
						workflow.DistributeStage.JumpBoxHost,
						distributeS3StoreURL,
						distribute,
					)
					if err != nil {
						log.Errorf("distrbiute stages to subtasks error: %v", err)
						return nil, err
					}
				}
			}
			subTasks = append(subTasks, distributeTasks...)
		}

		if workflow.SecurityStage != nil && workflow.SecurityStage.Enabled {
			securityTask, err := addSecurityToSubTasks()
			if err != nil {
				log.Errorf("add security task error: %v", err)
				return nil, err
			}
			if _, err := commonservice.GetServiceTemplate(
				artifact.Name, setting.PMDeployType, args.ProductTmplName, setting.ProductStatusDeleting, 0, log,
			); err != nil {
				subTasks = append(subTasks, securityTask)
			}
		}

		// 填充subtask之间关联内容
		task := &task.Task{
			TaskID:        nextTaskID,
			TaskCreator:   taskCreator,
			ReqID:         args.ReqID,
			SubTasks:      subTasks,
			ServiceName:   artifact.Name,
			ConfigPayload: configPayload,
			WorkflowArgs:  args,
		}
		sort.Sort(ByTaskKind(task.SubTasks))

		if err := ensurePipelineTask(task, log); err != nil {
			log.Errorf("workflow_task ensurePipelineTask task:[%v] err:%v", task, err)
			if err, ok := err.(*ContainerNotFound); ok {
				err := e.NewWithExtras(
					e.ErrCreateTaskFailed,
					"container doesn't exists", map[string]interface{}{
						"productName":   err.ProductName,
						"envName":       err.EnvName,
						"serviceName":   err.ServiceName,
						"containerName": err.Container,
					})
				return nil, err
			}

			return nil, e.ErrCreateTask.AddDesc(err.Error())
		}

		for _, stask := range task.SubTasks {
			AddSubtaskToStage(&stages, stask, artifact.Name)
		}
	}

	testTask := &task.Task{
		TaskID:       nextTaskID,
		PipelineName: args.WorkflowName,
		ProductName:  args.ProductTmplName,
	}
	testTasks, err := testArgsToSubtask(args, testTask, log)
	if err != nil {
		log.Errorf("workflow_task testArgsToSubtask args:[%v] err:%v", args, err)
		return nil, e.ErrCreateTask.AddDesc(err.Error())
	}

	for _, testTask := range testTasks {
		FmtBuilds(testTask.JobCtx.Builds, log)
		testSubTask, err := testTask.ToSubTask()
		if err != nil {
			log.Errorf("workflow_task ToSubTask err:%v", err)
			return nil, e.ErrCreateTask.AddDesc(err.Error())
		}

		err = SetCandidateRegistry(configPayload, log)
		if err != nil {
			log.Errorf("workflow_task setCandidateRegistry configPayload:[%v] err:%v", configPayload, err)
			return nil, err
		}

		AddSubtaskToStage(&stages, testSubTask, testTask.TestModuleName)
	}

	sort.Sort(ByStageKind(stages))
	triggerBy := &commonmodels.TriggerBy{
		Source:         args.Source,
		MergeRequestID: args.MergeRequestID,
		CommitID:       args.CommitID,
	}
	task := &task.Task{
		TaskID:        nextTaskID,
		Type:          config.WorkflowType,
		ProductName:   workflow.ProductTmplName,
		PipelineName:  args.WorkflowName,
		Description:   args.Description,
		TaskCreator:   taskCreator,
		ReqID:         args.ReqID,
		Status:        config.StatusCreated,
		Stages:        stages,
		WorkflowArgs:  args,
		ConfigPayload: configPayload,
		StorageURI:    defaultS3StoreURL,
		ResetImage:    workflow.ResetImage,
		TriggerBy:     triggerBy,
	}

	if len(task.Stages) <= 0 {
		return nil, e.ErrCreateTask.AddDesc(e.PipelineSubTaskNotFoundErrMsg)
	}

	if env != nil {
		task.Services = env.Services
		task.Render = env.Render
		task.ConfigPayload.DeployClusterID = env.ClusterID
	}

	if config.EnableGitCheck() {
		if err := createGitCheck(task, log); err != nil {
			log.Errorf("workflow createGitCheck task:[%v] err:%v", task, err)
		}
	}

	if err := CreateTask(task); err != nil {
		log.Errorf("workflow Create task:[%v] err:%v", task, err)
		return nil, e.ErrCreateTask
	}

	_ = scmnotify.NewService().UpdateWebhookComment(task, log)
	resp := &CreateTaskResp{PipelineName: args.WorkflowName, TaskID: nextTaskID}
	return resp, nil
}

func BuildModuleToSubTasks(moduleName, version, target, serviceName, productName string, envs []*commonmodels.KeyVal, pro *commonmodels.Product, log *zap.SugaredLogger) ([]map[string]interface{}, error) {
	var (
		subTasks    = make([]map[string]interface{}, 0)
		serviceTmpl *commonmodels.Service
	)

	serviceTmpl, _ = commonservice.GetServiceTemplate(
		serviceName, "", "", setting.ProductStatusDeleting, 0, log,
	)
	if serviceTmpl != nil && serviceTmpl.Visibility == setting.PUBLICSERVICE {
		productName = serviceTmpl.ProductName
	}
	serviceTmpl = nil

	opt := &commonrepo.BuildListOption{
		Name:        moduleName,
		Version:     version,
		ServiceName: serviceName,
		ProductName: productName,
	}

	if len(target) > 0 {
		opt.Targets = []string{target}
	}

	if pro != nil {
		serviceTmpl, _ = commonservice.GetServiceTemplate(
			target, setting.PMDeployType, productName, setting.ProductStatusDeleting, 0, log,
		)
	}

	modules, err := commonrepo.NewBuildColl().List(opt)
	if err != nil {
		return nil, e.ErrConvertSubTasks.AddErr(err)
	}

	registries, err := commonservice.ListRegistryNamespaces(log)
	if err != nil {
		return nil, e.ErrConvertSubTasks.AddErr(err)
	}

	for _, module := range modules {
		build := &task.Build{
			TaskType:     config.TaskBuild,
			Enabled:      true,
			InstallItems: module.PreBuild.Installs,
			ServiceName:  target,
			Service:      serviceName,
			JobCtx:       task.JobCtx{},
			ImageID:      module.PreBuild.ImageID,
			BuildOS:      module.PreBuild.BuildOS,
			ImageFrom:    module.PreBuild.ImageFrom,
			ResReq:       module.PreBuild.ResReq,
			Timeout:      module.Timeout,
			Registries:   registries,
		}

		// 自定义基础镜像的镜像名称可能会被更新，需要使用ID获取最新的镜像名称
		if module.PreBuild.ImageID != "" {
			basicImage, err := commonrepo.NewBasicImageColl().Find(module.PreBuild.ImageID)
			if err != nil {
				log.Errorf("BasicImage.Find failed, id:%s, err:%v", module.PreBuild.ImageID, err)
			} else {
				build.BuildOS = basicImage.Value
			}
		}

		if build.ImageFrom == "" {
			build.ImageFrom = commonmodels.ImageFromKoderover
		}

		if serviceTmpl != nil {
			build.Namespace = pro.Namespace
			build.ServiceType = setting.PMDeployType
		}

		if pro != nil {
			build.EnvName = pro.EnvName
		}

		if build.InstallItems == nil {
			build.InstallItems = make([]*commonmodels.Item, 0)
		}

		build.JobCtx.Builds = module.Repos
		if len(build.JobCtx.Builds) == 0 {
			build.JobCtx.Builds = make([]*types.Repository, 0)
		}

		build.JobCtx.BuildSteps = []*task.BuildStep{}
		if module.Scripts != "" {
			build.JobCtx.BuildSteps = append(build.JobCtx.BuildSteps, &task.BuildStep{BuildType: "shell", Scripts: module.Scripts})
		}

		if module.PMDeployScripts != "" && build.ServiceType == setting.PMDeployType {
			build.JobCtx.PMDeployScripts = module.PMDeployScripts
		}

		if len(module.SSHs) > 0 && build.ServiceType == setting.PMDeployType {
			privateKeys := make([]*task.SSH, 0)
			for _, sshID := range module.SSHs {
				//私钥信息可能被更新，而构建中存储的信息是旧的，需要根据id获取最新的私钥信息
				latestKeyInfo, err := commonrepo.NewPrivateKeyColl().Find(sshID)
				if err != nil || latestKeyInfo == nil {
					log.Errorf("PrivateKey.Find failed, id:%s, err:%v", sshID, err)
					continue
				}
				ssh := new(task.SSH)
				ssh.Name = latestKeyInfo.Name
				ssh.UserName = latestKeyInfo.UserName
				ssh.IP = latestKeyInfo.IP
				ssh.PrivateKey = latestKeyInfo.PrivateKey

				privateKeys = append(privateKeys, ssh)
			}
			build.JobCtx.SSHs = privateKeys
		}

		build.JobCtx.EnvVars = module.PreBuild.Envs

		if len(module.PreBuild.Envs) == 0 {
			build.JobCtx.EnvVars = make([]*commonmodels.KeyVal, 0)
		}

		if len(envs) > 0 {
			for _, envVar := range build.JobCtx.EnvVars {
				for _, overwrite := range envs {
					if overwrite.Key == envVar.Key && overwrite.Value != setting.MaskValue {
						envVar.Value = overwrite.Value
						envVar.IsCredential = overwrite.IsCredential
						break
					}
				}
			}
		}

		build.JobCtx.UploadPkg = module.PreBuild.UploadPkg
		build.JobCtx.CleanWorkspace = module.PreBuild.CleanWorkspace
		build.JobCtx.EnableProxy = module.PreBuild.EnableProxy

		if module.PostBuild != nil && module.PostBuild.DockerBuild != nil {
			build.JobCtx.DockerBuildCtx = &task.DockerBuildCtx{
				WorkDir:    module.PostBuild.DockerBuild.WorkDir,
				DockerFile: module.PostBuild.DockerBuild.DockerFile,
				BuildArgs:  module.PostBuild.DockerBuild.BuildArgs,
			}
		}

		if module.PostBuild != nil && module.PostBuild.FileArchive != nil {
			build.JobCtx.FileArchiveCtx = &task.FileArchiveCtx{
				FileLocation: module.PostBuild.FileArchive.FileLocation,
			}
		}

		if module.PostBuild != nil && module.PostBuild.Scripts != "" {
			build.JobCtx.PostScripts = module.PostBuild.Scripts
		}

		build.JobCtx.Caches = module.Caches

		bst, err := build.ToSubTask()
		if err != nil {
			return subTasks, e.ErrConvertSubTasks.AddErr(err)
		}
		subTasks = append(subTasks, bst)
	}

	return subTasks, nil
}

func ensurePipelineTask(pt *task.Task, log *zap.SugaredLogger) error {
	var (
		buildEnvs []*commonmodels.KeyVal
	)

	// 验证 Subtask payload
	err := validateSubTaskSetting(pt.PipelineName, pt.SubTasks)
	if err != nil {
		log.Errorf("Validate subtask setting failed: %+v", err)
		return err
	}

	//设置执行任务时参数
	for i, subTask := range pt.SubTasks {

		pre, err := base.ToPreview(subTask)
		if err != nil {
			return errors.New(e.InterfaceToTaskErrMsg)
		}

		switch pre.TaskType {

		case config.TaskBuild:
			t, err := base.ToBuildTask(subTask)
			fmtBuildsTask(t, log)
			if err != nil {
				log.Error(err)
				return err
			}

			if t.Enabled {
				//for _, arg := range pt.TaskArgs.BuildArgs {
				//	for _, k := range t.JobCtx.EnvVars {
				//		if arg.Key == k.Key {
				//			if !(arg.Value == env.MaskValue && arg.IsCredential) {
				//				k.Value = arg.Value
				//			}
				//
				//			break
				//		}
				//	}
				//}

				// 设置Pipeline对应的服务名称
				if t.ServiceName != "" {
					pt.ServiceName = t.ServiceName
				}

				// 设置 build 安装脚本
				t.InstallCtx, err = buildInstallCtx(t.InstallItems)
				if err != nil {
					log.Error(err)
					return err
				}

				// 外部触发的pipeline
				if pt.TaskCreator == setting.WebhookTaskCreator || pt.TaskCreator == setting.CronTaskCreator {
					SetTriggerBuilds(t.JobCtx.Builds, pt.TaskArgs.Builds)
				} else {
					setManunalBuilds(t.JobCtx.Builds, pt.TaskArgs.Builds)
				}

				// 生成默认镜像tag后缀
				pt.TaskArgs.Deploy.Tag = releaseCandidateTag(t, pt.TaskID)

				// 设置镜像名称
				// 编译任务使用 t.JobCtx.Image
				// 注意: 其他任务从 pt.TaskArgs.Deploy.Image 获取, 必须要有编译任务
				reg, err := commonservice.FindDefaultRegistry(log)
				if err != nil {
					log.Errorf("can't find default candidate registry: %v", err)
					return e.ErrFindRegistry.AddDesc(err.Error())
				}

				t.JobCtx.Image = GetImage(reg, t.ServiceName, pt.TaskArgs.Deploy.Tag)
				pt.TaskArgs.Deploy.Image = t.JobCtx.Image

				if pt.ConfigPayload != nil {
					pt.ConfigPayload.Registry.Addr = reg.RegAddr
					pt.ConfigPayload.Registry.AccessKey = reg.AccessKey
					pt.ConfigPayload.Registry.SecretKey = reg.SecretKey
					pt.ConfigPayload.Registry.Namespace = reg.Namespace
				}

				// 二进制文件名称
				// 编译任务使用 t.JobCtx.PackageFile
				// 注意: 其他任务从 pt.TaskArgs.Deploy.PackageFile 获取, 必须要有编译任务
				t.JobCtx.PackageFile = GetPackageFile(t.ServiceName, pt.TaskArgs.Deploy.Tag)
				pt.TaskArgs.Deploy.PackageFile = t.JobCtx.PackageFile

				// 注入编译模块中用户定义环境变量
				// 注意: 需要在pt.TaskArgs.Deploy设置完之后再设置环境变量
				t.JobCtx.EnvVars = append(t.JobCtx.EnvVars, prepareTaskEnvs(pt, log)...)
				// 如果其他模块需要使用编译模块的环境变量进行渲染，需要设置buildEnvs
				buildEnvs = t.JobCtx.EnvVars

				if t.JobCtx.DockerBuildCtx != nil {
					t.JobCtx.DockerBuildCtx.ImageName = t.JobCtx.Image
				}

				if t.JobCtx.FileArchiveCtx != nil {
					//t.JobCtx.FileArchiveCtx.FileName = t.ServiceName
					t.JobCtx.FileArchiveCtx.FileName = GetPackageFile(t.ServiceName, pt.TaskArgs.Deploy.Tag)
				}

				// TODO: generic
				//if t.JobCtx.StorageUri == "" && pt.Type == pipe.SingleType {
				//	if store, err := s.GetDefaultS3Storage(log); err != nil {
				//		return e.ErrFindDefaultS3Storage.AddDesc("default s3 storage is required by package building")
				//	} else {
				//		if t.JobCtx.StorageUri, err = store.GetEncryptedUrl(types.S3STORAGE_AES_KEY); err != nil {
				//			log.Errorf("failed encrypt storage uri %v", err)
				//			return err
				//		}
				//	}
				//}

				t.DockerBuildStatus = &task.DockerBuildStatus{
					ImageName:    t.JobCtx.Image,
					RegistryRepo: reg.RegAddr + "/" + reg.Namespace,
				}
				t.UTStatus = &task.UTStatus{}
				t.StaticCheckStatus = &task.StaticCheckStatus{}
				t.BuildStatus = &task.BuildStatus{}

				pt.SubTasks[i], err = t.ToSubTask()

				if err != nil {
					return err
				}
			}
		case config.TaskJenkinsBuild:
			t, err := base.ToJenkinsBuildTask(subTask)
			if err != nil {
				log.Error(err)
				return err
			}
			if t.Enabled {
				// 分析镜像名称
				image := ""
				for _, jenkinsBuildParams := range t.JenkinsBuildArgs.JenkinsBuildParams {
					if jenkinsBuildParams.Name != "IMAGE" {
						continue
					}
					if value, ok := jenkinsBuildParams.Value.(string); ok {
						image = value
						break
					}
				}

				if image == "" || !strings.Contains(image, ":") {
					return &ImageIllegal{}
				}

				pt.TaskArgs.Deploy.Image = image
				t.Image = image
				pt.SubTasks[i], err = t.ToSubTask()
				if err != nil {
					return err
				}
			}
		case config.TaskArtifact:
			t, err := base.ToArtifactTask(subTask)
			if err != nil {
				log.Error(err)
				return err
			}
			if t.Enabled {
				if pt.TaskArgs == nil {
					pt.TaskArgs = &commonmodels.TaskArgs{PipelineName: pt.WorkflowArgs.WorkflowName, TaskCreator: pt.WorkflowArgs.WorklowTaskCreator}
				}
				registry := pt.ConfigPayload.RepoConfigs[pt.WorkflowArgs.RegistryID]
				t.Image = fmt.Sprintf("%s/%s/%s", util.GetURLHostName(registry.RegAddr), registry.Namespace, t.Image)
				pt.TaskArgs.Deploy.Image = t.Image
				t.RegistryID = pt.WorkflowArgs.RegistryID

				pt.SubTasks[i], err = t.ToSubTask()
				if err != nil {
					return err
				}
			}
		case config.TaskDockerBuild:
			t, err := base.ToDockerBuildTask(subTask)
			if err != nil {
				log.Error(err)
				return err
			}
			if t.Enabled {
				t.Image = pt.TaskArgs.Deploy.Image

				// 使用环境变量KEY渲染 docker build的各个参数
				for _, env := range buildEnvs {
					if !env.IsCredential {
						t.DockerFile = strings.Replace(t.DockerFile, fmt.Sprintf("$%s", env.Key), env.Value, -1)
						t.WorkDir = strings.Replace(t.WorkDir, fmt.Sprintf("$%s", env.Key), env.Value, -1)
						t.BuildArgs = strings.Replace(t.BuildArgs, fmt.Sprintf("$%s", env.Key), env.Value, -1)
					}
				}

				err = SetCandidateRegistry(pt.ConfigPayload, log)
				if err != nil {
					return err
				}

				pt.SubTasks[i], err = t.ToSubTask()
				if err != nil {
					return err
				}
			}

		case config.TaskTestingV2:
			t, err := base.ToTestingTask(subTask)
			if err != nil {
				log.Error(err)
				return err
			}

			if t.Enabled {
				FmtBuilds(t.JobCtx.Builds, log)
				// 获取 pipeline keystores 信息
				envs := make([]*commonmodels.KeyVal, 0)

				// Iterate test jobctx builds, and replace it if params specified from task.
				// 外部触发的pipeline
				if pt.TaskCreator == setting.WebhookTaskCreator || pt.TaskCreator == setting.CronTaskCreator {
					SetTriggerBuilds(t.JobCtx.Builds, pt.TaskArgs.Test.Builds)
				} else {
					setManunalBuilds(t.JobCtx.Builds, pt.TaskArgs.Test.Builds)
				}

				err = SetCandidateRegistry(pt.ConfigPayload, log)
				if err != nil {
					return err
				}

				//use log path
				if pt.ServiceName == "" {
					pt.ServiceName = t.TestName
				}

				// 设置敏感信息
				t.JobCtx.EnvVars = append(t.JobCtx.EnvVars, envs...)

				pt.SubTasks[i], err = t.ToSubTask()
				if err != nil {
					return err
				}
			}
		case config.TaskResetImage:
			t, err := base.ToDeployTask(subTask)
			if err != nil {
				log.Error(err)
				return err
			}

			if t.Enabled {
				t.SetNamespace(pt.TaskArgs.Deploy.Namespace)
				image, err := validateServiceContainer(t.EnvName, t.ProductName, t.ServiceName, t.ContainerName)
				if err != nil {
					log.Error(err)
					return err
				}

				t.SetImage(image)

				pt.SubTasks[i], err = t.ToSubTask()
				if err != nil {
					return err
				}
			}

		case config.TaskDeploy:
			t, err := base.ToDeployTask(subTask)
			if err != nil {
				log.Error(err)
				return err
			}

			if t.Enabled {
				// 从创建任务payload设置容器部署
				t.SetImage(pt.TaskArgs.Deploy.Image)
				t.SetNamespace(pt.TaskArgs.Deploy.Namespace)

				_, err := validateServiceContainer(t.EnvName, t.ProductName, t.ServiceName, t.ContainerName)
				if err != nil {
					log.Error(err)
					return err
				}

				// Task creator can be webhook trigger or cronjob trigger or validated user
				// Validated user includes both that user is granted write permission or user is the owner of this product
				if pt.TaskCreator == setting.WebhookTaskCreator ||
					pt.TaskCreator == setting.CronTaskCreator ||
					IsProductAuthed(pt.TaskCreator, t.Namespace, pt.ProductName, config.ProductWritePermission, log) {
					log.Infof("Validating permission passed. product:%s, owner:%s, task executed by: %s", pt.ProductName, t.Namespace, pt.TaskCreator)
				} else {
					log.Errorf("permission denied. product:%s, owner:%s, task executed by: %s", pt.ProductName, t.Namespace, pt.TaskCreator)
					return errors.New(e.ProductAccessDeniedErrMsg)
				}

				pt.SubTasks[i], err = t.ToSubTask()
				if err != nil {
					return err
				}
			}

		case config.TaskDistributeToS3:
			task, err := base.ToDistributeToS3Task(subTask)
			if err != nil {
				log.Error(err)
				return err
			}
			if task.Enabled {
				task.SetPackageFile(pt.TaskArgs.Deploy.PackageFile)

				if pt.TeamName == "" {
					task.ProductName = pt.ProductName
				} else {
					task.ProductName = pt.TeamName
				}

				task.ServiceName = pt.ServiceName

				if task.DestStorageURL == "" {
					var storage *commonmodels.S3Storage
					// 在pipeline中配置了对象存储
					if task.S3StorageID != "" {
						if storage, err = commonrepo.NewS3StorageColl().Find(task.S3StorageID); err != nil {
							return e.ErrFindS3Storage.AddDesc(fmt.Sprintf("id:%s, %v", task.S3StorageID, err))
						}
					} else if storage, err = GetS3RelStorage(log); err != nil { // adapt for qbox
						return e.ErrFindS3Storage
					}

					defaultS3 := s3.S3{
						S3Storage: storage,
					}

					task.DestStorageURL, err = defaultS3.GetEncryptedURL()
					if err != nil {
						return err
					}
				}
				pt.SubTasks[i], err = task.ToSubTask()
				if err != nil {
					return err
				}
			}

		case config.TaskReleaseImage:
			t, err := base.ToReleaseImageTask(subTask)
			if err != nil {
				log.Error(err)
				return err
			}

			if t.Enabled {
				// 从创建任务payload设置线上镜像分发任务
				t.SetImage(pt.TaskArgs.Deploy.Image)
				// 兼容老的任务配置
				if len(t.Releases) == 0 {
					found := false
					for id, v := range pt.ConfigPayload.RepoConfigs {
						if v.Namespace == t.ImageRepo {
							t.Releases = append(t.Releases, commonmodels.RepoImage{RepoID: id})
							found = true
						}
					}

					if !found {
						return fmt.Errorf("没有找到命名空间是 [%s] 的镜像仓库", t.ImageRepo)
					}
				}

				var repos []commonmodels.RepoImage
				for _, repoImage := range t.Releases {
					if v, ok := pt.ConfigPayload.RepoConfigs[repoImage.RepoID]; ok {
						repoImage.Name = util.ReplaceRepo(pt.TaskArgs.Deploy.Image, v.RegAddr, v.Namespace)
						repoImage.Host = util.GetURLHostName(v.RegAddr)
						repoImage.Namespace = v.Namespace
						repos = append(repos, repoImage)
					}
				}

				t.Releases = repos
				if len(t.Releases) == 0 {
					t.Enabled = false
				} else {
					t.ImageRelease = t.Releases[0].Name
				}

				pt.SubTasks[i], err = t.ToSubTask()
				if err != nil {
					return err
				}
			}

		case config.TaskJira:
			t, err := base.ToJiraTask(subTask)
			if err != nil {
				log.Error(err)
				return err
			}
			FmtBuilds(t.Builds, log)
			if t.Enabled {
				// 外部触发的pipeline
				if pt.TaskCreator == setting.WebhookTaskCreator || pt.TaskCreator == setting.CronTaskCreator {
					SetTriggerBuilds(t.Builds, pt.TaskArgs.Builds)
				} else {
					setManunalBuilds(t.Builds, pt.TaskArgs.Builds)
				}

				pt.SubTasks[i], err = t.ToSubTask()
				if err != nil {
					return err
				}
			}
		case config.TaskSecurity:
			t, err := base.ToSecurityTask(subTask)
			if err != nil {
				log.Error(err)
				return err
			}

			if t.Enabled {
				t.SetImageName(pt.TaskArgs.Deploy.Image)
				pt.SubTasks[i], err = t.ToSubTask()
				if err != nil {
					return err
				}
			}
		case config.TaskDistribute:
			// 为了兼容历史数据类型，目前什么都不用做，避免出错
		default:
			return e.NewErrInvalidTaskType(string(pre.TaskType))
		}
	}

	return nil
}

func AddSubtaskToStage(stages *[]*commonmodels.Stage, subTask map[string]interface{}, target string) {
	subTaskPre, err := base.ToPreview(subTask)
	if err != nil {
		log.Errorf("subtask to preview error: %v", err)
		return
	}
	if subTaskPre.TaskType == "" {
		log.Error("empty subtask task type")
		return
	}
	stageFound := false

	for _, stage := range *stages {
		if stage.TaskType == subTaskPre.TaskType {
			// deploy task 同一个组件可能有多个部署目标
			if subTaskPre.TaskType == config.TaskDeploy || subTaskPre.TaskType == config.TaskResetImage {
				if _, ok := stage.SubTasks[target]; ok {
					stage.SubTasks[target+"_"+nextTargetID(stage.SubTasks, target)] = subTask
				} else {
					stage.SubTasks[target] = subTask
				}
			} else {
				stage.SubTasks[target] = subTask
			}
			stageFound = true
			break
		}
	}

	if !stageFound {
		stage := &commonmodels.Stage{
			TaskType:    subTaskPre.TaskType,
			SubTasks:    map[string]map[string]interface{}{target: subTask},
			RunParallel: true,
		}

		if subTaskPre.TaskType == config.TaskResetImage {
			stage.AfterAll = true
		}
		// 除了测试模块，其他的都可以并行跑
		//if subTaskPre.TaskType == task.TaskTestingV2 {
		//	stage.RunParallel = false
		//}
		*stages = append(*stages, stage)
	}
}

func nextTargetID(subTasks map[string]map[string]interface{}, target string) string {
	count := 0
	for k := range subTasks {
		if regexp.MustCompile(`^\Q` + target + `\E(_\d+)?$`).MatchString(k) {
			count++
		}
	}

	return strconv.Itoa(count)
}
