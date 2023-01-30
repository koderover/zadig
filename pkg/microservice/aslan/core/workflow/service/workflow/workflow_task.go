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
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	gotempl "text/template"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"

	configbase "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/task"
	taskmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/task"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	templaterepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/base"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/jira"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/s3"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/scmnotify"
	templ "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/template"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/client/systemconfig"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/types"
	"github.com/koderover/zadig/pkg/util"
)

const (
	ClusterStorageEP = "nfs-server"
)

type CronjobWorkflowArgs struct {
	Target []*commonmodels.TargetArgs `bson:"targets"                      json:"targets"`
}

type ServiceBuildInfo struct {
	ServiceName   string `json:"service_name"`
	ServiceModule string `json:"service_module"`
	BuildName     string `json:"build_name"`
}

// GetWorkflowArgs 返回工作流详细信息
func GetWorkflowArgs(productName, namespace string, serviceBuildInfos []*ServiceBuildInfo, log *zap.SugaredLogger) (*CronjobWorkflowArgs, error) {
	resp := &CronjobWorkflowArgs{}
	opt := &commonrepo.ProductFindOptions{Name: productName, EnvName: namespace}
	product, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		log.Errorf("Product.Find error: %v", err)
		return resp, e.ErrFindProduct.AddDesc(err.Error())
	}

	allModules, err := ListBuildDetail("", "", log)
	if err != nil {
		log.Errorf("BuildModule.List error: %v", err)
		return resp, e.ErrListBuildModule.AddDesc(err.Error())
	}

	targetMap, _ := commonservice.GetProductTargetMap(product)
	targets := make([]*commonmodels.TargetArgs, 0)

	for _, serviceBuildInfo := range serviceBuildInfos {
		container := strings.Join([]string{productName, serviceBuildInfo.ServiceName, serviceBuildInfo.ServiceModule}, SplitSymbol)
		if _, ok := targetMap[container]; !ok {
			continue
		}
		target := &commonmodels.TargetArgs{Name: serviceBuildInfo.ServiceModule, ServiceName: serviceBuildInfo.ServiceName, Deploy: targetMap[container], Build: &commonmodels.BuildArgs{}, HasBuild: true}
		moBuild, _ := findModuleByServiceBuildInfo(allModules, serviceBuildInfo)
		if moBuild == nil {
			moBuild = &commonmodels.Build{}
			target.HasBuild = false
		}
		err = fillBuildDetail(moBuild, serviceBuildInfo.ServiceName, serviceBuildInfo.ServiceModule)
		if err != nil {
			return resp, e.ErrListBuildModule.AddErr(err)
		}

		target.Build.Repos = moBuild.SafeRepos()

		if moBuild.PreBuild != nil {
			EnsureBuildResp(moBuild)
			target.Envs = moBuild.PreBuild.Envs
		}

		if moBuild.JenkinsBuild != nil {
			jenkinsBuildParams := make([]*types.JenkinsBuildParam, 0)
			for _, jenkinsBuildParam := range moBuild.JenkinsBuild.JenkinsBuildParam {
				jenkinsBuildParams = append(jenkinsBuildParams, &types.JenkinsBuildParam{
					Name:         jenkinsBuildParam.Name,
					Value:        jenkinsBuildParam.Value,
					Type:         jenkinsBuildParam.Type,
					ChoiceOption: jenkinsBuildParam.ChoiceOption,
					AutoGenerate: jenkinsBuildParam.AutoGenerate,
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

func getHideServiceModules(workflow *commonmodels.Workflow) sets.String {
	hideServiceModules := sets.NewString()
	if workflow.BuildStage != nil && workflow.BuildStage.Enabled {
		for _, buildModule := range workflow.BuildStage.Modules {
			if buildModule.HideServiceModule {
				hideServiceModules.Insert(strings.Join([]string{buildModule.Target.ProductName, buildModule.Target.ServiceName, buildModule.Target.ServiceModule}, SplitSymbol))
			}
		}
	}

	if workflow.ArtifactStage != nil && workflow.ArtifactStage.Enabled {
		for _, artifactModule := range workflow.ArtifactStage.Modules {
			if artifactModule.HideServiceModule {
				hideServiceModules.Insert(strings.Join([]string{artifactModule.Target.ProductName, artifactModule.Target.ServiceName, artifactModule.Target.ServiceModule}, SplitSymbol))
			}
		}
	}

	return hideServiceModules
}

func getProjectTargets(productName string) []string {
	var targets []string
	productTmpl, err := template.NewProductColl().Find(productName)
	if err != nil {
		log.Errorf("[%s] ProductTmpl.Find error: %v", productName, err)
		return targets
	}
	services, err := commonrepo.NewServiceColl().ListMaxRevisionsForServices(productTmpl.AllServiceInfos(), "")
	if err != nil {
		log.Errorf("ServiceTmpl.ListMaxRevisions error: %v", err)
		return targets
	}

	for _, serviceTmpl := range services {
		switch serviceTmpl.Type {
		case setting.K8SDeployType, setting.HelmDeployType:
			for _, container := range serviceTmpl.Containers {
				targets = append(targets, strings.Join([]string{serviceTmpl.ProductName, serviceTmpl.ServiceName, container.Name}, SplitSymbol))
			}
		case setting.PMDeployType:
			targets = append(targets, strings.Join([]string{serviceTmpl.ProductName, serviceTmpl.ServiceName, serviceTmpl.ServiceName}, SplitSymbol))
		}
	}

	return targets
}

func findModuleByContainer(productName, serviceModuleTarget string, buildStageModules []*commonmodels.BuildModule, allModules []*commonmodels.Build, log *zap.SugaredLogger) (*commonmodels.Build, *commonmodels.ServiceModuleTarget) {
	// find build name
	var buildName string
	for _, buildStageModule := range buildStageModules {
		if buildStageModule.Target == nil {
			return nil, nil
		}
		targetStr := fmt.Sprintf("%s%s%s%s%s", buildStageModule.Target.ProductName, SplitSymbol, buildStageModule.Target.ServiceName, SplitSymbol, buildStageModule.Target.ServiceModule)
		if serviceModuleTarget == targetStr {
			buildName = buildStageModule.Target.BuildName
			break
		}
	}
	if buildName == "" {
		// Compatible with old data if buildName is empty
		productTmpl, err := template.NewProductColl().Find(productName)
		if err != nil {
			log.Errorf("failed to find project,err:%s", err)
			return nil, nil
		}
		services, err := commonrepo.NewServiceColl().ListMaxRevisionsForServices(productTmpl.AllServiceInfos(), "")
		if err != nil {
			log.Errorf("failed to list services,err:%s", err)
			return nil, nil
		}

		for _, serviceTmpl := range services {
			if serviceTmpl.Type != setting.PMDeployType {
				for _, container := range serviceTmpl.Containers {
					targetStr := fmt.Sprintf("%s%s%s%s%s", serviceTmpl.ProductName, SplitSymbol, serviceTmpl.ServiceName, SplitSymbol, container.Name)
					if serviceModuleTarget == targetStr {
						buildName = findBuildNameByContainerName(container.Name, serviceTmpl)
					}
				}
			} else if serviceTmpl.Type == setting.PMDeployType {
				targetStr := fmt.Sprintf("%s%s%s%s%s", serviceTmpl.ProductName, SplitSymbol, serviceTmpl.ServiceName, SplitSymbol, serviceTmpl.ServiceName)
				if serviceModuleTarget == targetStr {
					buildName = findBuildNameByContainerName(serviceTmpl.ServiceName, serviceTmpl)
				}
			}
		}
	}
	if buildName != "" {
		for _, module := range allModules {
			if module.Name == buildName {
				for _, target := range module.Targets {
					targetStr := fmt.Sprintf("%s%s%s%s%s", target.ProductName, SplitSymbol, target.ServiceName, SplitSymbol, target.ServiceModule)
					if targetStr == serviceModuleTarget {
						return module, target
					}
				}
			}
		}
	}

	return nil, nil
}

func findBuildNameByContainerName(containerName string, serviceTmpl *commonmodels.Service) string {
	opt := &commonrepo.BuildListOption{
		ServiceName: serviceTmpl.ServiceName,
		Targets:     []string{containerName},
	}
	if serviceTmpl.Visibility != setting.PublicService {
		opt.ProductName = serviceTmpl.ProductName
	}

	buildModules, err := commonrepo.NewBuildColl().List(opt)
	if err != nil || len(buildModules) == 0 {
		return ""
	}
	buildName := buildModules[0].Name
	return buildName
}

func findModuleByTargetAndVersion(allModules []*commonmodels.Build, serviceModuleTarget string) (*commonmodels.Build, *commonmodels.ServiceModuleTarget) {
	containerArr := strings.Split(serviceModuleTarget, SplitSymbol)
	if len(containerArr) != 3 {
		return nil, nil
	}

	opt := &commonrepo.ServiceFindOption{
		ServiceName:   containerArr[1],
		ProductName:   containerArr[0],
		ExcludeStatus: setting.ProductStatusDeleting,
	}
	serviceObj, _ := commonrepo.NewServiceColl().Find(opt)
	if serviceObj != nil && serviceObj.Visibility == setting.PublicService {
		containerArr[0] = serviceObj.ProductName
	}
	for _, mo := range allModules {
		for _, target := range mo.Targets {
			targetStr := fmt.Sprintf("%s%s%s%s%s", target.ProductName, SplitSymbol, target.ServiceName, SplitSymbol, target.ServiceModule)
			if targetStr == strings.Join(containerArr, SplitSymbol) {
				return mo, target
			}
		}
	}
	return nil, nil
}

func findModuleByServiceBuildInfo(allModules []*commonmodels.Build, serviceBuildInfo *ServiceBuildInfo) (*commonmodels.Build, *commonmodels.ServiceModuleTarget) {
	for _, mo := range allModules {
		if mo.Name != serviceBuildInfo.BuildName {
			continue
		}
		for _, target := range mo.Targets {
			if target.ServiceName == serviceBuildInfo.ServiceName && target.ServiceModule == serviceBuildInfo.ServiceModule {
				return mo, target
			}
		}
	}
	return nil, nil
}

func EnsureBuildResp(mb *commonmodels.Build) {
	if len(mb.Targets) == 0 {
		mb.Targets = make([]*commonmodels.ServiceModuleTarget, 0)
	}

	mb.Repos = mb.SafeRepos()
	for _, repo := range mb.Repos {
		repo.RepoNamespace = repo.GetRepoNamespace()
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

func ListBuildDetail(name, targets string, log *zap.SugaredLogger) ([]*commonmodels.Build, error) {
	opt := &commonrepo.BuildListOption{
		Name: name,
	}

	if len(strings.TrimSpace(targets)) != 0 {
		opt.Targets = strings.Split(targets, ",")
	}

	resp, err := commonrepo.NewBuildColl().List(opt)
	if err != nil {
		log.Errorf("[Build.List] %s error: %v", name, err)
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
	filtermap := make(map[string][]*types.BranchFilterInfo)
	if workflow.BuildStage.Enabled {
		for _, buildModule := range workflow.BuildStage.Modules {
			buildKey := fmt.Sprintf("%s/%s", buildModule.Target.ServiceModule, buildModule.Target.ServiceName)
			filtermap[buildKey] = buildModule.BranchFilter
		}
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

	allModules, err := ListBuildDetail("", "", log)
	if err != nil {
		log.Errorf("BuildModule.List error: %v", err)
		return resp, e.ErrListBuildModule.AddDesc(err.Error())
	}

	allTestings, err := commonrepo.NewTestingColl().List(&commonrepo.ListTestOption{ProductName: "", TestType: ""})
	if err != nil {
		log.Errorf("TestingModule.List error: %v", err)
		return resp, e.ErrListTestModule.AddDesc(err.Error())
	}

	targetMap, imageNameM := commonservice.GetProductTargetMap(product)
	projectTargets := getProjectTargets(product.ProductName)
	hideServiceModules := getHideServiceModules(workflow)
	targets := make([]*commonmodels.TargetArgs, 0)
	if (workflow.BuildStage != nil && workflow.BuildStage.Enabled) || (workflow.ArtifactStage != nil && workflow.ArtifactStage.Enabled) {
		for _, container := range projectTargets {
			if hideServiceModules.Has(container) {
				continue
			}
			if _, ok := targetMap[container]; !ok {
				continue
			}

			containerArr := strings.Split(container, SplitSymbol)
			if len(containerArr) != 3 {
				continue
			}
			target := &commonmodels.TargetArgs{
				Name:        containerArr[2],
				ServiceName: containerArr[1],
				ProductName: containerArr[0],
				Deploy:      targetMap[container],
				ImageName:   imageNameM[container],
				Build:       &commonmodels.BuildArgs{},
				HasBuild:    true,
			}

			moBuild, targetInfo := findModuleByContainer(workflow.ProductTmplName, container, workflow.BuildStage.Modules, allModules, log)
			if moBuild == nil {
				moBuild = &commonmodels.Build{}
				target.HasBuild = false
			}
			target.BuildName = moBuild.Name
			err = fillBuildDetail(moBuild, containerArr[1], containerArr[2])
			if err != nil {
				return resp, e.ErrListBuildModule.AddErr(err)
			}

			if moBuild.TemplateID != "" {
				target.Build.Repos = targetInfo.Repos
			} else {
				target.Build.Repos = moBuild.SafeReposDeepCopy()
			}

			if moBuild.PreBuild != nil {
				EnsureBuildResp(moBuild)
				target.Envs = moBuild.PreBuild.Envs
			}

			if moBuild.JenkinsBuild != nil {
				jenkinsBuildParams := make([]*types.JenkinsBuildParam, 0)
				for _, jenkinsBuildParam := range moBuild.JenkinsBuild.JenkinsBuildParam {
					jenkinsBuildParams = append(jenkinsBuildParams, &types.JenkinsBuildParam{
						Name:         jenkinsBuildParam.Name,
						Value:        jenkinsBuildParam.Value,
						Type:         jenkinsBuildParam.Type,
						ChoiceOption: jenkinsBuildParam.ChoiceOption,
						AutoGenerate: jenkinsBuildParam.AutoGenerate,
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

	for _, target := range targets {
		key := fmt.Sprintf("%s/%s", target.Name, target.ServiceName)
		for _, repoInfo := range target.Build.Repos {
			if filterList, ok := filtermap[key]; ok {
				for _, filter := range filterList {
					// make sure they are the same repository
					if filter.CodehostID == repoInfo.CodehostID && filter.RepoOwner == repoInfo.RepoOwner && filter.RepoName == repoInfo.RepoName {
						repoInfo.FilterRegexp = filter.FilterRegExp
						if filter.DefaultBranch != "" {
							repoInfo.Branch = filter.DefaultBranch
						}
						break
					}
				}
			}
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
							ChoiceOption: moduleEnv.ChoiceOption,
							Type:         moduleEnv.Type,
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

func CreateWorkflowTask(args *commonmodels.WorkflowTaskArgs, taskCreator string, log *zap.SugaredLogger) (*CreateTaskResp, error) {
	if args == nil {
		return nil, fmt.Errorf("args should not be nil")
	}

	// RequestMode=openAPI means that external clients call the API, and some data needs to be obtained and added to args
	if args.RequestMode == setting.RequestModeOpenAPI {
		log.Info("CreateWorkflowTask from openAPI")
		resp, err := AddDataToArgsOrCreateReleaseImageTask(args, log)
		if err != nil {
			log.Errorf("AddDataToArgs error: %v", err)
			return nil, err
		}
		// create release image task
		if resp != nil {
			return resp, nil
		}
		taskCreator = setting.RequestModeOpenAPI
	}

	workflow, err := commonrepo.NewWorkflowColl().Find(args.WorkflowName)
	if err != nil {
		log.Errorf("Workflow.Find error: %v", err)
		return nil, e.ErrFindWorkflow.AddDesc(err.Error())
	}

	project, err := template.NewProductColl().Find(workflow.ProductTmplName)
	if err != nil {
		log.Errorf("project.Find error: %v", err)
		return nil, e.ErrFindWorkflow.AddDesc(err.Error())
	}
	// developer don't pass args.ProductTmplName
	if args.ProductTmplName == "" {
		args.ProductTmplName = workflow.ProductTmplName
	}
	args.IsParallel = workflow.IsParallel

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

	// get global configPayload
	configPayload := commonservice.GetConfigPayload(args.CodehostID)
	if len(env.RegistryID) == 0 {
		reg, _, err := commonservice.FindDefaultRegistry(false, log)
		if err != nil {
			log.Errorf("get default registry error: %v", err)
			return nil, e.ErrGetCounter.AddDesc(err.Error())
		}
		env.RegistryID = reg.ID.Hex()
	}
	configPayload.RegistryID = env.RegistryID
	err = SetCandidateRegistry(configPayload, log)
	if err != nil {
		log.Errorf("workflow_task setCandidateRegistry configPayload:[%v] err:%v", configPayload, err)
		return nil, err
	}

	nextTaskID, err := generateNextTaskID(args.WorkflowName)
	if err != nil {
		return nil, err
	}
	// modify configPayload
	modifyConfigPayload(configPayload, args.IgnoreCache, args.ResetCache)

	distributeS3StoreURL, defaultS3StoreURL, err := getDefaultAndDestS3StoreURL(workflow, log)
	if err != nil {
		log.Errorf("getDefaultAndDestS3StoreUrl workflow name:[%s] err:%v", workflow.Name, err)
		return nil, e.ErrCreateTask.AddErr(err)
	}

	stages := make([]*commonmodels.Stage, 0)
	serviceInfos := make([]*taskmodels.ServiceInfo, 0)
	for _, target := range args.Target {
		var subTasks []map[string]interface{}
		var err error
		if target.JenkinsBuildArgs == nil {
			buildModuleArgs := &commonmodels.BuildModuleArgs{
				Target:      target.Name,
				ServiceName: target.ServiceName,
				//ProductName: target.ProductName,				// TODO productName may be nil in some situation, need to figure out reason
				ProductName: args.ProductTmplName,
				Variables:   target.Envs,
				Env:         env,
				BuildName:   getBuildName(workflow, target.Name, target.ServiceName),
			}
			subTasks, err = BuildModuleToSubTasks(buildModuleArgs, log)
		} else {
			subTasks, err = JenkinsBuildModuleToSubTasks(&JenkinsBuildOption{
				Target:           target.Name,
				ServiceName:      target.ServiceName,
				ProductName:      args.ProductTmplName,
				JenkinsBuildArgs: target.JenkinsBuildArgs,
				BuildName:        target.BuildName,
			}, log)
		}
		if err != nil {
			log.Errorf("buildModuleToSubTasks target:[%s] err:%v", target.Name, err)
			return nil, e.ErrCreateTask.AddErr(err)
		}

		if env != nil {
			// generate subtasks to deploy
			for _, deployEnv := range target.Deploy {
				if deployEnv.Type == setting.PMDeployType {
					continue
				}
				deployTask, err := deployEnvToSubTasks(deployEnv, env, project.Timeout)
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
						resetImageTask["release_name"] = deployTask["release_name"]
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
				// since more field is introduced into the serviceModuleTarget structure, we only use the field we need to determine if the target match
				if serviceModule.ProductName == distribute.Target.ProductName && serviceModule.ServiceName == distribute.Target.ServiceName && serviceModule.ServiceModule == distribute.Target.ServiceModule {
					distributeTasks, err = formatDistributeSubtasks(
						serviceModule,
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

		_, err = jira.GetJiraInfo()
		if err == nil {
			jiraTask, err := AddJiraSubTask(target.Name, target.Name, target.ServiceName, args.ProductTmplName, getBuildName(workflow, target.Name, target.ServiceName), log)
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
		task := &taskmodels.Task{
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

		if err := ensurePipelineTask(&taskmodels.TaskOpt{
			Task:           task,
			EnvName:        args.Namespace,
			ServiceName:    target.ServiceName,
			ServiceInfos:   &serviceInfos,
			IsWorkflowTask: true,
			ImageName:      target.ImageName,
		}, log); err != nil {
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
			AddSubtaskToStage(&stages, stask, target.Name+"_"+target.ServiceName)
		}
	}
	// add extension to stage
	if workflow.ExtensionStage != nil && workflow.ExtensionStage.Enabled {
		extensionTask, err := addExtensionToSubTasks(workflow.ExtensionStage, serviceInfos)
		if err != nil {
			log.Errorf("add extension task error: %s", err)
			return nil, e.ErrCreateTask.AddErr(err)
		}
		AddSubtaskToStage(&stages, extensionTask, string(config.TaskExtension))
	}

	testTask := &taskmodels.Task{
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

		AddSubtaskToStage(&stages, testSubTask, testTask.TestModuleName)
	}

	sort.Sort(ByStageKind(stages))
	triggerBy := &commonmodels.TriggerBy{
		CodehostID:     args.CodehostID,
		RepoOwner:      args.RepoOwner,
		RepoNamespace:  args.RepoNamespace,
		RepoName:       args.RepoName,
		Source:         args.Source,
		MergeRequestID: args.MergeRequestID,
		CommitID:       args.CommitID,
		Ref:            args.Ref,
		EventType:      args.EventType,
	}
	task := &taskmodels.Task{
		TaskID:              nextTaskID,
		Type:                config.WorkflowType,
		ProductName:         workflow.ProductTmplName,
		PipelineDisplayName: workflow.DisplayName,
		PipelineName:        args.WorkflowName,
		Description:         args.Description,
		TaskCreator:         taskCreator,
		ReqID:               args.ReqID,
		Status:              config.StatusCreated,
		Stages:              stages,
		WorkflowArgs:        args,
		ConfigPayload:       configPayload,
		StorageURI:          defaultS3StoreURL,
		ResetImage:          workflow.ResetImage,
		ResetImagePolicy:    workflow.ResetImagePolicy,
		TriggerBy:           triggerBy,
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
	resp := &CreateTaskResp{
		ProjectName:  args.ProductTmplName,
		PipelineName: args.WorkflowName,
		TaskID:       nextTaskID,
	}
	return resp, nil
}

func generateNextTaskID(workflowName string) (int64, error) {
	nextTaskID, err := commonrepo.NewCounterColl().GetNextSeq(fmt.Sprintf(setting.WorkflowTaskFmt, workflowName))
	if err != nil {
		log.Errorf("Counter.GetNextSeq error: %s", err)
		return 0, e.ErrGetCounter.AddDesc(err.Error())
	}
	return nextTaskID, nil
}

func modifyConfigPayload(configPayload *commonmodels.ConfigPayload, ignoreCache, resetCache bool) {
	repos, err := commonservice.ListRegistryNamespaces("", true, log.SugaredLogger())
	if err == nil {
		configPayload.RepoConfigs = make(map[string]*commonmodels.RegistryNamespace)
		for _, repo := range repos {
			configPayload.RepoConfigs[repo.ID.Hex()] = repo
		}
	}

	configPayload.IgnoreCache = ignoreCache
	configPayload.ResetCache = resetCache

}

// add data to workflow args or create release image task
func AddDataToArgsOrCreateReleaseImageTask(args *commonmodels.WorkflowTaskArgs, log *zap.SugaredLogger) (*CreateTaskResp, error) {
	if len(args.Target) == 0 && len(args.ReleaseImages) == 0 {
		return nil, errors.New("target and release_images cannot be empty at the same time")
	}

	workflow, err := commonrepo.NewWorkflowColl().Find(args.WorkflowName)
	if err != nil {
		log.Errorf("[Workflow.Find] error: %s", err)
		return nil, e.ErrFindWorkflow.AddErr(err)
	}
	if len(args.Target) > 0 {
		builds, err := commonrepo.NewBuildColl().List(&commonrepo.BuildListOption{})
		if err != nil {
			log.Errorf("[Build.List] error: %s", err)
			return nil, e.ErrListBuildModule.AddErr(err)
		}
		args.ProductTmplName = workflow.ProductTmplName

		// Complete target information
		for _, target := range args.Target {
			target.HasBuild = true
			// OpenAPI mode, the name passed in is the service name
			serviceType, err := getValidServiceType(target.ServiceType)
			if err != nil {
				return nil, err
			}
			opt := &commonrepo.ServiceFindOption{
				ServiceName:   target.Name,
				ProductName:   workflow.ProductTmplName,
				Type:          serviceType,
				ExcludeStatus: setting.ProductStatusDeleting}
			serviceTmpl, err := commonrepo.NewServiceColl().Find(opt)
			if err != nil {
				log.Errorf("[ServiceTmpl.Find] error: %s", err)
				return nil, e.ErrGetService.AddErr(err)
			}

			// Complete the deploy information in the target
			deploys := make([]commonmodels.DeployEnv, 0)
			for _, container := range serviceTmpl.Containers {
				// If the service component in the service is built, put the service component into deploy
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

			// Complete the build information in the target
			for _, build := range builds {
				if len(build.Targets) == 0 {
					continue
				}
				// If the service component owned by the build contains any service component of the service, the match is considered successful
				match := false
				repos := build.Repos
				for _, container := range serviceTmpl.Containers {
					serviceModuleTarget := &commonmodels.ServiceModuleTarget{
						ProductName:   serviceTmpl.ProductName,
						ServiceName:   serviceTmpl.ServiceName,
						ServiceModule: container.Name,
					}
					for _, buildTarget := range build.Targets {
						if reflect.DeepEqual(buildTarget, serviceModuleTarget) {
							target.Name = container.Name
							target.ServiceName = serviceTmpl.ServiceName
							match = true
							repos = buildTarget.Repos
							break
						}
					}
				}
				if !match {
					continue
				}
				// If the service component is successfully matched, match the warehouse name from the built warehouse list and add the repo information
				if target.Build == nil {
					continue
				}

				for _, buildRepo := range repos {
					for _, targetRepo := range target.Build.Repos {
						if targetRepo.RepoName == buildRepo.RepoName {
							// openAPI only pass repoName, branch, pr
							targetRepo.Source = buildRepo.Source
							targetRepo.RepoOwner = buildRepo.RepoOwner
							targetRepo.RepoNamespace = buildRepo.RepoNamespace
							targetRepo.RemoteName = buildRepo.RemoteName
							targetRepo.CodehostID = buildRepo.CodehostID
							targetRepo.CheckoutPath = buildRepo.CheckoutPath
						}
					}
				}
			}
		}
		// Complete test information
		if workflow.TestStage != nil && workflow.TestStage.Enabled {
			tests := make([]*commonmodels.TestArgs, 0)
			for _, testName := range workflow.TestStage.TestNames {
				moduleTest, err := commonrepo.NewTestingColl().Find(testName, "")
				if err != nil {
					log.Errorf("[Testing.Find] TestModuleName:%s, error:%s", testName, err)
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
					log.Errorf("[Testing.Find] TestModuleName:%s, error:%s", testEntity.Name, err)
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

		return nil, nil
	}

	return createReleaseImageTask(workflow, args, log)
}

func buildRegistryMap() (map[string]*commonmodels.RegistryNamespace, error) {
	registries, err := commonservice.ListRegistryNamespaces("", true, log.SugaredLogger())
	if err != nil {
		return nil, fmt.Errorf("failed to query registries")
	}
	ret := make(map[string]*commonmodels.RegistryNamespace)
	for _, singleRegistry := range registries {
		fullUrl := fmt.Sprintf("%s/%s", singleRegistry.RegAddr, singleRegistry.Namespace)
		fullUrl = strings.TrimSuffix(fullUrl, "/")
		u, _ := url.Parse(fullUrl)
		if len(u.Scheme) > 0 {
			fullUrl = strings.TrimPrefix(fullUrl, fmt.Sprintf("%s://", u.Scheme))
		}
		ret[fullUrl] = singleRegistry
	}
	return ret, nil
}

func createReleaseImageTask(workflow *commonmodels.Workflow, args *commonmodels.WorkflowTaskArgs, log *zap.SugaredLogger) (*CreateTaskResp, error) {
	// Get global configPayload
	configPayload := commonservice.GetConfigPayload(0)
	args.ProductTmplName = workflow.ProductTmplName
	nextTaskID, err := generateNextTaskID(workflow.Name)
	if err != nil {
		return nil, err
	}
	// modify configPayload
	modifyConfigPayload(configPayload, false, false)

	registryMap, err := buildRegistryMap()
	if err != nil {
		log.Errorf("failed to build registry map, err: %s", err)
		// use default registry
		reg, _, err := commonservice.FindDefaultRegistry(true, log)
		if err != nil {
			log.Errorf("can't find default candidate registry, err: %s", err)
			return nil, e.ErrFindRegistry.AddDesc(err.Error())
		}
		configPayload.Registry.Addr = reg.RegAddr
		configPayload.Registry.AccessKey = reg.AccessKey
		configPayload.Registry.SecretKey = reg.SecretKey
		configPayload.Registry.Namespace = reg.Namespace
	} else {
		// extract registry from image
		for _, releaseImage := range args.ReleaseImages {
			registryUrl, err := commonservice.ExtractImageRegistry(releaseImage.Image)
			if err != nil {
				log.Errorf("failed to extract image registry, image:%s  err: %s", releaseImage.Image, err)
				continue
			}
			registryUrl = strings.TrimSuffix(registryUrl, "/")
			if reg, ok := registryMap[registryUrl]; ok {
				configPayload.Registry.Addr = reg.RegAddr
				configPayload.Registry.AccessKey = reg.AccessKey
				configPayload.Registry.SecretKey = reg.SecretKey
				configPayload.Registry.Namespace = reg.Namespace
				break
			}
		}
	}

	distributeS3StoreURL, defaultS3StoreURL, err := getDefaultAndDestS3StoreURL(workflow, log)
	if err != nil {
		log.Errorf("getDefaultAndDestS3StoreUrl workflow name:[%s] err:%s", workflow.Name, err)
		return nil, e.ErrCreateTask.AddErr(err)
	}

	stages := make([]*commonmodels.Stage, 0)
	// Generate distributed subtask
	if workflow.DistributeStage != nil && workflow.DistributeStage.Enabled {
		var (
			distributeTasks []map[string]interface{}
			err             error
			subTasks        = make([]map[string]interface{}, 0)
		)

		for _, imageInfo := range args.ReleaseImages {
			for _, distribute := range workflow.DistributeStage.Distributes {
				if distribute.Target == nil {
					continue
				}
				if distribute.Target.ServiceModule == imageInfo.ServiceModule && distribute.Target.ServiceName == imageInfo.ServiceName {
					distributeTasks, err = formatDistributeSubtasks(
						&commonmodels.ServiceModuleTarget{
							ProductName:   args.ProductTmplName,
							ServiceName:   imageInfo.ServiceName,
							ServiceModule: imageInfo.ServiceModule,
						},
						workflow.DistributeStage.Releases,
						workflow.DistributeStage.ImageRepo,
						workflow.DistributeStage.JumpBoxHost,
						distributeS3StoreURL,
						distribute,
					)
					if err != nil {
						log.Errorf("distrbiute stages to subtasks error: %s", err)
						return nil, e.ErrCreateTask.AddErr(err)
					}
				}
			}
			subTasks = append(subTasks, distributeTasks...)

			// Fill in the associated content between subtasks
			task := &taskmodels.Task{
				TaskID:        nextTaskID,
				PipelineName:  workflow.Name,
				TaskCreator:   setting.RequestModeOpenAPI,
				ReqID:         args.ReqID,
				SubTasks:      subTasks,
				ServiceName:   imageInfo.ServiceModule,
				ConfigPayload: configPayload,
				TaskArgs:      &commonmodels.TaskArgs{PipelineName: workflow.Name, TaskCreator: setting.RequestModeOpenAPI, Deploy: commonmodels.DeployArgs{Image: imageInfo.Image}},
				ProductName:   workflow.ProductTmplName,
			}
			sort.Sort(ByTaskKind(task.SubTasks))

			if err := ensurePipelineTask(&taskmodels.TaskOpt{
				Task: task,
			}, log); err != nil {
				log.Errorf("workflow_task ensurePipelineTask taskID:[%d] pipelineName:[%s] err:%s", task.ID, task.PipelineName, err)
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
				AddSubtaskToStage(&stages, stask, imageInfo.ServiceModule)
			}
		}
	}

	sort.Sort(ByStageKind(stages))
	task := &taskmodels.Task{
		WorkflowArgs:        args,
		TaskID:              nextTaskID,
		Type:                config.WorkflowType,
		ProductName:         workflow.ProductTmplName,
		PipelineName:        workflow.Name,
		PipelineDisplayName: workflow.DisplayName,
		TaskCreator:         setting.RequestModeOpenAPI,
		ReqID:               args.ReqID,
		Status:              config.StatusCreated,
		Stages:              stages,
		ConfigPayload:       configPayload,
		StorageURI:          defaultS3StoreURL,
	}

	if len(task.Stages) <= 0 {
		errMessage := fmt.Sprintf("%s or %s", e.PipelineSubTaskNotFoundErrMsg, "Invalid service module")
		return nil, e.ErrCreateTask.AddDesc(errMessage)
	}

	endpoint := fmt.Sprintf("%s-%s:9000", config.Namespace(), ClusterStorageEP)
	task.StorageEndpoint = endpoint

	if err := CreateTask(task); err != nil {
		log.Errorf("workflow Create task:[%v] err:%s", task, err)
		return nil, e.ErrCreateTask
	}

	resp := &CreateTaskResp{
		ProjectName:  workflow.ProductTmplName,
		PipelineName: workflow.Name,
		TaskID:       nextTaskID,
	}

	return resp, nil
}

// Only supports k8s and helm two service types currently
func getValidServiceType(serviceType string) (string, error) {
	switch serviceType {
	// Compatible when the service_type is equal to empty
	case setting.K8SDeployType, "":
		return setting.K8SDeployType, nil
	case setting.HelmDeployType:
		return setting.HelmDeployType, nil
	default:
		return "", fmt.Errorf("Unsupported service type")
	}
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
		deployTask = taskmodels.Deploy{
			TaskType:    config.TaskDeploy,
			Enabled:     true,
			Namespace:   prodEnv.Namespace,
			ProductName: prodEnv.ProductName,
			EnvName:     prodEnv.EnvName,
			Timeout:     timeout,
			ClusterID:   prodEnv.ClusterID,
		}
	)

	envList := strings.Split(env.Env, "/")
	if len(envList) != 2 {
		err := fmt.Errorf("[%s]split target env error", env.Env)
		log.Error(err)
		return nil, err
	}
	deployTask.ServiceName = envList[0]
	deployTask.ContainerName = envList[1] + "_" + envList[0]

	switch env.Type {
	case setting.K8SDeployType:
		deployTask.ServiceType = setting.K8SDeployType
		return deployTask.ToSubTask()
	case setting.HelmDeployType:
		deployTask.ServiceType = setting.HelmDeployType
		if pSvc, ok := prodEnv.GetServiceMap()[deployTask.ServiceName]; ok {
			deployTask.ServiceRevision = pSvc.Revision
		}
		revisionSvc, err := commonrepo.NewServiceColl().Find(&commonrepo.ServiceFindOption{
			ServiceName: deployTask.ServiceName,
			Revision:    deployTask.ServiceRevision,
			ProductName: prodEnv.ProductName,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to find service: %s with revision: %d, err: %s", deployTask.ServiceName, deployTask.ServiceRevision, err)
		}
		deployTask.ReleaseName = util.GeneReleaseName(revisionSvc.GetReleaseNaming(), prodEnv.ProductName, prodEnv.Namespace, prodEnv.EnvName, deployTask.ServiceName)
		return deployTask.ToSubTask()
	}
	return resp, fmt.Errorf("env type not match")
}

func resetImageTaskToSubTask(env commonmodels.DeployEnv, prodEnv *commonmodels.Product) (map[string]interface{}, error) {
	switch env.Type {
	case setting.K8SDeployType:
		deployTask := taskmodels.Deploy{TaskType: config.TaskResetImage, Enabled: true}
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
	case setting.HelmDeployType:
		deployTask := taskmodels.Deploy{TaskType: config.TaskResetImage, Enabled: true}
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
		deployTask.ServiceType = setting.HelmDeployType
		return deployTask.ToSubTask()
	default:
		return nil, nil
	}
}

func artifactToSubTasks(name, image string) (map[string]interface{}, error) {
	artifactTask := taskmodels.Artifact{TaskType: config.TaskArtifact, Enabled: true}
	artifactTask.Name = name
	artifactTask.Image = image
	return artifactTask.ToSubTask()
}

func formatDistributeSubtasks(serviceModule *commonmodels.ServiceModuleTarget, releaseImages []commonmodels.RepoImage, imageRepo, jumpboxHost, destStorageURL string, distribute *commonmodels.ProductDistribute) ([]map[string]interface{}, error) {
	var resp []map[string]interface{}
	productName := serviceModule.ProductName
	if distribute.ImageDistribute {
		t := taskmodels.ReleaseImage{
			TaskType:    config.TaskReleaseImage,
			Enabled:     true,
			ProductName: productName,
		}
		// get product Info for further use
		productInfo, err := template.NewProductColl().Find(productName)
		if err != nil {
			return nil, err
		}
		distributeInfo := make([]*taskmodels.DistributeInfo, 0)
		// now we add distributeInfo in
		for _, repoInfo := range releaseImages {
			envInfo, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
				EnvName: repoInfo.DeployEnv,
				Name:    productName,
			})
			if err != nil {
				return nil, err
			}
			releaseName := ""
			if repoInfo.DeployEnabled {
				if productInfo.ProductFeature != nil && productInfo.ProductFeature.DeployType == setting.HelmDeployType {
					svcMap := envInfo.GetServiceMap()
					pSvc, ok := svcMap[serviceModule.ServiceName]
					if !ok {
						return nil, fmt.Errorf("can't find service: %s in product: %s:%s", serviceModule.ServiceName, productName, repoInfo.DeployEnv)
					}
					templateSvc, err := commonrepo.NewServiceColl().Find(&commonrepo.ServiceFindOption{
						ServiceName: serviceModule.ServiceName,
						Revision:    pSvc.Revision,
						Type:        setting.HelmDeployType,
						ProductName: productName,
					})
					if err != nil {
						return nil, err
					}
					releaseName = util.GeneReleaseName(templateSvc.GetReleaseNaming(), productName, envInfo.Namespace, repoInfo.DeployEnv, serviceModule.ServiceName)
				}
			}
			distributeInfo = append(distributeInfo, &taskmodels.DistributeInfo{
				DeployEnabled:     repoInfo.DeployEnabled,
				DeployEnv:         repoInfo.DeployEnv,
				DeployServiceType: productInfo.ProductFeature.DeployType,
				DeployClusterID:   envInfo.ClusterID,
				DeployNamespace:   envInfo.Namespace,
				ReleaseName:       releaseName,
				RepoID:            repoInfo.RepoID,
			})
		}
		t.DistributeInfo = distributeInfo
		t.Releases = releaseImages

		// convert to subtask
		subtask, err := t.ToSubTask()
		if err != nil {
			return resp, err
		}
		resp = append(resp, subtask)
	}
	if distribute.QstackDistribute && destStorageURL != "" {
		task := taskmodels.DistributeToS3{
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

func AddJiraSubTask(moduleName, target, serviceName, productName, buildName string, log *zap.SugaredLogger) (map[string]interface{}, error) {
	repos := make([]*types.Repository, 0)

	jira := &taskmodels.Jira{
		TaskType: config.TaskJira,
		Enabled:  true,
	}

	module, err := getBuildModule(buildName, serviceName, target, productName)
	if err != nil {
		return nil, e.ErrConvertSubTasks.AddErr(err)
	}
	if module.TemplateID == "" {
		repos = append(repos, module.Repos...)
	} else {
		for _, repo := range module.Targets {
			if repo.ServiceName == serviceName && repo.ServiceModule == moduleName {
				repos = append(repos, repo.Repos...)
				break
			}
		}
	}

	jira.Builds = repos
	return jira.ToSubTask()
}

func addSecurityToSubTasks() (map[string]interface{}, error) {
	securityTask := taskmodels.Security{TaskType: config.TaskSecurity, Enabled: true}
	return securityTask.ToSubTask()
}

func addExtensionToSubTasks(stage *commonmodels.ExtensionStage, serviceInfos []*taskmodels.ServiceInfo) (map[string]interface{}, error) {
	extensionTask := taskmodels.Extension{
		TaskType:     config.TaskExtension,
		Enabled:      true,
		URL:          stage.URL,
		Path:         stage.Path,
		Headers:      stage.Headers,
		IsCallback:   stage.IsCallback,
		Timeout:      stage.Timeout,
		ServiceInfos: serviceInfos,
	}
	return extensionTask.ToSubTask()
}

func workFlowArgsToTaskArgs(target string, workflowArgs *commonmodels.WorkflowTaskArgs) *commonmodels.TaskArgs {
	resp := &commonmodels.TaskArgs{PipelineName: workflowArgs.WorkflowName, TaskCreator: workflowArgs.WorkflowTaskCreator}
	for _, build := range workflowArgs.Target {
		if build.Name == target {
			if build.Build != nil {
				resp.Builds = build.Build.Repos
			}
		}
	}
	for _, build := range resp.Builds {
		if workflowArgs.MergeRequestID == "" {
			continue
		}
		mrID, _ := strconv.Atoi(workflowArgs.MergeRequestID)
		if build.Source == workflowArgs.Source && build.RepoName == workflowArgs.RepoName && build.RepoOwner == workflowArgs.RepoOwner {
			build.PR = mrID
			build.PRs = []int{mrID}
		}
	}
	return resp
}

// TODO 和validation中转化testsubtask合并为一个方法
func testArgsToSubtask(args *commonmodels.WorkflowTaskArgs, pt *taskmodels.Task, log *zap.SugaredLogger) ([]*taskmodels.Testing, error) {
	var resp []*taskmodels.Testing
	var servicesArray []string
	var services string

	// 创建任务的测试参数为脱敏数据，需要转换为实际数据
	for _, test := range args.Tests {
		existed, err := commonrepo.NewTestingColl().Find(test.TestModuleName, args.ProductTmplName)
		if err == nil && existed.PreTest != nil {
			commonservice.EnsureSecretEnvs(existed.PreTest.Envs, test.Envs)
		}
	}

	for _, service := range args.Target {
		servicesArray = append(servicesArray, service.ServiceName)
	}
	services = strings.Join(servicesArray, ",")

	testArgs := args.Tests
	testCreator := args.WorkflowTaskCreator

	registries, err := commonservice.ListRegistryNamespaces("", true, log)
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
		for _, repo := range testModule.Repos {
			repoInfo, err := systemconfig.New().GetCodeHost(repo.CodehostID)
			if err != nil {
				log.Errorf("Failed to get proxy settings for codehost ID: %d, the error is: %s", repo.CodehostID, err)
				return nil, err
			}
			repo.EnableProxy = repoInfo.EnableProxy
			repo.RepoNamespace = repo.GetRepoNamespace()
		}

		testTask := &taskmodels.Testing{
			TaskType: config.TaskTestingV2,
			Enabled:  true,
			TestName: "test",
			Timeout:  testModule.Timeout,
		}
		testTask.TestModuleName = testModule.Name
		testTask.JobCtx.TestType = testModule.TestType
		testTask.JobCtx.Builds = testModule.Repos
		testTask.JobCtx.BuildSteps = append(testTask.JobCtx.BuildSteps, &taskmodels.BuildStep{BuildType: "shell", Scripts: testModule.Scripts})

		testTask.JobCtx.ArtifactPaths = testModule.ArtifactPaths
		testTask.JobCtx.TestThreshold = testModule.Threshold
		testTask.JobCtx.Caches = testModule.Caches
		testTask.JobCtx.TestResultPath = testModule.TestResultPath
		testTask.JobCtx.TestReportPath = testModule.TestReportPath

		clusterInfo, err := commonrepo.NewK8SClusterColl().Get(testModule.PreTest.ClusterID)
		if err != nil {
			return nil, err
		}
		testTask.Cache = clusterInfo.Cache

		// If the cluster is not configured with a cache medium, the cache cannot be used, so don't enable cache explicitly.
		if testTask.Cache.MediumType == "" {
			testTask.CacheEnable = false
		} else {
			testTask.CacheEnable = testModule.CacheEnable
			testTask.CacheDirType = testModule.CacheDirType
			testTask.CacheUserDir = testModule.CacheUserDir
		}

		if testTask.Registries == nil {
			testTask.Registries = registries
		}

		if testModule.PreTest != nil {
			testTask.InstallItems = testModule.PreTest.Installs
			testTask.JobCtx.CleanWorkspace = testModule.PreTest.CleanWorkspace
			testTask.JobCtx.EnableProxy = testModule.PreTest.EnableProxy
			testTask.Namespace = testModule.PreTest.Namespace
			testTask.ClusterID = testModule.PreTest.ClusterID

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
			envs = append(envs, &commonmodels.KeyVal{Key: "SERVICES", Value: services})
			envs = append(envs, &commonmodels.KeyVal{Key: "WORKSPACE", Value: "/workspace"})

			testTask.JobCtx.EnvVars = envs
			testTask.ImageID = testModule.PreTest.ImageID
			testTask.BuildOS = testModule.PreTest.BuildOS
			testTask.ImageFrom = testModule.PreTest.ImageFrom
			testTask.ClusterID = testModule.PreTest.ClusterID
			testTask.Namespace = testModule.PreTest.Namespace
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
			testTask.ResReqSpec = testModule.PreTest.ResReqSpec
		}
		// 设置 build 安装脚本
		testTask.InstallCtx, err = BuildInstallCtx(testTask.InstallItems)
		if err != nil {
			log.Errorf("buildInstallCtx error: %v", err)
			return resp, err
		}

		// Iterate test jobctx builds, and replace it if params specified from taskmodels.
		// 外部触发的pipeline
		if testCreator == setting.WebhookTaskCreator || testCreator == setting.CronTaskCreator {
			_ = SetTriggerBuilds(testTask.JobCtx.Builds, testArg.Builds, log)
		} else {
			_ = setManunalBuilds(testTask.JobCtx.Builds, testArg.Builds, log)
		}

		resp = append(resp, testTask)
	}

	return resp, nil
}

func CreateArtifactWorkflowTask(args *commonmodels.WorkflowTaskArgs, taskCreator string, log *zap.SugaredLogger) (*CreateTaskResp, error) {
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
	repos, err := commonservice.ListRegistryNamespaces("", true, log)
	if err == nil {
		configPayload.RepoConfigs = make(map[string]*commonmodels.RegistryNamespace)
		for _, repo := range repos {
			configPayload.RepoConfigs[repo.ID.Hex()] = repo
		}
	}

	err = SetCandidateRegistry(configPayload, log)
	if err != nil {
		log.Errorf("workflow_task setCandidateRegistry configPayload:[%v] err:%v", configPayload, err)
		return nil, err
	}

	configPayload.IgnoreCache = args.IgnoreCache
	configPayload.ResetCache = args.ResetCache

	distributeS3StoreURL, defaultS3StoreURL, err := getDefaultAndDestS3StoreURL(workflow, log)
	if err != nil {
		log.Errorf("getDefaultAndDestS3StoreUrl workflow name:[%s] err:%v", workflow.Name, err)
		return nil, err
	}

	stages := make([]*commonmodels.Stage, 0)
	serviceInfos := make([]*taskmodels.ServiceInfo, 0)
	for _, artifact := range args.Artifact {
		subTasks := make([]map[string]interface{}, 0)
		// image artifact deploy
		if artifact.Image != "" {
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
							resetImageTask["release_name"] = deployTask["release_name"]
							subTasks = append(subTasks, resetImageTask)
						}
					}
					subTasks = append(subTasks, deployTask)
				}
			}
		} else if artifact.FileName != "" {
			buildModuleArgs := &commonmodels.BuildModuleArgs{
				Target:       artifact.Name,
				ServiceName:  artifact.ServiceName,
				ProductName:  args.ProductTmplName,
				Env:          env,
				URL:          artifact.URL,
				WorkflowName: artifact.WorkflowName,
				TaskID:       artifact.TaskID,
				FileName:     artifact.FileName,
				TaskType:     string(config.TaskArtifactDeploy),
			}
			buildSubtasks, err := BuildModuleToSubTasks(buildModuleArgs, log)
			if err != nil {
				log.Errorf("buildModuleToSubTasks target:[%s] err:%s", artifact.Name, err)
				return nil, e.ErrCreateTask.AddErr(err)
			}
			subTasks = append(subTasks, buildSubtasks...)
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
						serviceModule,
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
		task := &taskmodels.Task{
			ProductName:   args.ProductTmplName,
			PipelineName:  args.WorkflowName,
			TaskID:        nextTaskID,
			TaskCreator:   taskCreator,
			ReqID:         args.ReqID,
			SubTasks:      subTasks,
			ServiceName:   artifact.Name,
			ConfigPayload: configPayload,
			TaskArgs:      workFlowArgsToTaskArgs(artifact.Name, args),
			WorkflowArgs:  args,
		}
		sort.Sort(ByTaskKind(task.SubTasks))

		if err := ensurePipelineTask(&taskmodels.TaskOpt{
			Task:           task,
			EnvName:        args.Namespace,
			IsWorkflowTask: true,
			ServiceInfos:   &serviceInfos,
		}, log); err != nil {
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
			AddSubtaskToStage(&stages, stask, artifact.Name+"_"+artifact.ServiceName)
		}
	}

	// add extension to stage
	if workflow.ExtensionStage != nil && workflow.ExtensionStage.Enabled {
		extensionTask, err := addExtensionToSubTasks(workflow.ExtensionStage, serviceInfos)
		if err != nil {
			log.Errorf("add extension task error: %s", err)
			return nil, e.ErrCreateTask.AddErr(err)
		}
		AddSubtaskToStage(&stages, extensionTask, string(config.TaskExtension))
	}

	testTask := &taskmodels.Task{
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
		AddSubtaskToStage(&stages, testSubTask, testTask.TestModuleName)
	}

	sort.Sort(ByStageKind(stages))
	triggerBy := &commonmodels.TriggerBy{
		Source:         args.Source,
		MergeRequestID: args.MergeRequestID,
		CommitID:       args.CommitID,
	}
	task := &taskmodels.Task{
		TaskID:              nextTaskID,
		Type:                config.WorkflowType,
		ProductName:         workflow.ProductTmplName,
		PipelineName:        args.WorkflowName,
		PipelineDisplayName: workflow.DisplayName,
		Description:         args.Description,
		TaskCreator:         taskCreator,
		ReqID:               args.ReqID,
		Status:              config.StatusCreated,
		Stages:              stages,
		WorkflowArgs:        args,
		ConfigPayload:       configPayload,
		StorageURI:          defaultS3StoreURL,
		ResetImage:          workflow.ResetImage,
		ResetImagePolicy:    workflow.ResetImagePolicy,
		TriggerBy:           triggerBy,
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
	resp := &CreateTaskResp{
		ProjectName:  workflow.ProductTmplName,
		PipelineName: args.WorkflowName,
		TaskID:       nextTaskID,
	}
	return resp, nil
}

// fillBuildDetail fill the contents for builds created from build templates
func fillBuildDetail(moduleBuild *commonmodels.Build, serviceName, serviceModule string) error {
	if moduleBuild.TemplateID == "" {
		return nil
	}
	buildTemplate, err := commonrepo.NewBuildTemplateColl().Find(&commonrepo.BuildTemplateQueryOption{
		ID: moduleBuild.TemplateID,
	})
	if err != nil {
		return fmt.Errorf("failed to find build template with id: %s, err: %s", moduleBuild.TemplateID, err)
	}

	moduleBuild.Timeout = buildTemplate.Timeout
	moduleBuild.PreBuild = buildTemplate.PreBuild
	moduleBuild.JenkinsBuild = buildTemplate.JenkinsBuild
	moduleBuild.Scripts = buildTemplate.Scripts
	moduleBuild.PostBuild = buildTemplate.PostBuild
	moduleBuild.SSHs = buildTemplate.SSHs
	moduleBuild.PMDeployScripts = buildTemplate.PMDeployScripts
	moduleBuild.CacheEnable = buildTemplate.CacheEnable
	moduleBuild.CacheDirType = buildTemplate.CacheDirType
	moduleBuild.CacheUserDir = buildTemplate.CacheUserDir
	moduleBuild.AdvancedSettingsModified = buildTemplate.AdvancedSettingsModified

	// repos are configured by service modules
	for _, serviceConfig := range moduleBuild.Targets {
		if serviceConfig.ServiceName == serviceName && serviceConfig.ServiceModule == serviceModule {
			moduleBuild.Repos = serviceConfig.Repos
			if moduleBuild.PreBuild == nil {
				moduleBuild.PreBuild = &commonmodels.PreBuild{}
			}
			moduleBuild.PreBuild.Envs = commonservice.MergeBuildEnvs(moduleBuild.PreBuild.Envs, serviceConfig.Envs)
			break
		}
	}
	return nil
}

func BuildModuleToSubTasks(args *commonmodels.BuildModuleArgs, log *zap.SugaredLogger) ([]map[string]interface{}, error) {
	var (
		subTasks    = make([]map[string]interface{}, 0)
		serviceTmpl *commonmodels.Service
	)

	if args.Env != nil {
		serviceTmpl, _ = commonservice.GetServiceTemplate(
			args.Target, setting.PMDeployType, args.ProductName, setting.ProductStatusDeleting, 0, log,
		)
	}

	registries, err := commonservice.ListRegistryNamespaces("", true, log)
	if err != nil {
		return nil, e.ErrConvertSubTasks.AddErr(err)
	}

	module, err := getBuildModule(args.BuildName, args.ServiceName, args.Target, args.ProductName)
	if err != nil {
		return nil, e.ErrConvertSubTasks.AddErr(err)
	}

	err = fillBuildDetail(module, args.ServiceName, args.Target)
	if err != nil {
		return nil, e.ErrConvertSubTasks.AddErr(err)
	}
	build := &taskmodels.Build{
		TaskType:            config.TaskBuild,
		Enabled:             true,
		InstallItems:        module.PreBuild.Installs,
		ServiceName:         args.Target,
		Service:             args.ServiceName,
		JobCtx:              taskmodels.JobCtx{},
		ImageID:             module.PreBuild.ImageID,
		BuildOS:             module.PreBuild.BuildOS,
		ImageFrom:           module.PreBuild.ImageFrom,
		ResReq:              module.PreBuild.ResReq,
		ResReqSpec:          module.PreBuild.ResReqSpec,
		Timeout:             module.Timeout,
		Registries:          registries,
		ProductName:         args.ProductName,
		Namespace:           module.PreBuild.Namespace,
		ClusterID:           module.PreBuild.ClusterID,
		UseHostDockerDaemon: module.PreBuild.UseHostDockerDaemon,
	}

	// In some old build configurations, the `pre_build.cluster_id` field is empty indicating that's a local cluster.
	// We do a protection here to avoid query failure.
	// Resaving the build configuration after v1.8.0 will automatically populate this field.
	if module.PreBuild.ClusterID == "" {
		module.PreBuild.ClusterID = setting.LocalClusterID
	}

	clusterInfo, err := commonrepo.NewK8SClusterColl().Get(module.PreBuild.ClusterID)
	if err != nil {
		return nil, e.ErrConvertSubTasks.AddErr(fmt.Errorf("failed to get cluster: %s, err: %s", module.PreBuild.ClusterID, err))
	}
	build.Cache = clusterInfo.Cache

	// If the cluster is not configured with a cache medium, the cache cannot be used, so don't enable cache explicitly.
	if build.Cache.MediumType == "" {
		build.CacheEnable = false
	} else {
		build.CacheEnable = module.CacheEnable
		build.CacheDirType = module.CacheDirType
		build.CacheUserDir = module.CacheUserDir
	}

	if args.TaskType != "" {
		build.TaskType = config.TaskArtifactDeploy
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

	envHostInfos := make(map[string]*commonmodels.PrivateKey)
	if serviceTmpl != nil {
		build.ServiceType = setting.PMDeployType
		envHost := make(map[string][]string)
		envHostNames := make(map[string][]string)

		for _, envConfig := range serviceTmpl.EnvConfigs {
			privateKeys, err := commonrepo.NewPrivateKeyColl().ListHostIPByArgs(&commonrepo.ListHostIPArgs{IDs: envConfig.HostIDs})
			if err != nil {
				log.Errorf("ListNameByArgs ids err:%s", err)
				continue
			}
			ips := sets.NewString()
			names := sets.NewString()
			ips, names = extractHost(privateKeys, ips, names, envHostInfos)
			privateKeys, err = commonrepo.NewPrivateKeyColl().ListHostIPByArgs(&commonrepo.ListHostIPArgs{Labels: envConfig.Labels})
			if err != nil {
				log.Errorf("ListNameByArgs labels err:%s", err)
				continue
			}
			ips, names = extractHost(privateKeys, ips, names, envHostInfos)
			envHost[envConfig.EnvName] = ips.List()
			envHostNames[envConfig.EnvName] = names.List()
		}
		build.EnvHostInfo = envHost
		build.EnvHostNames = envHostNames
	}

	if args.Env != nil {
		build.EnvName = args.Env.EnvName
	}

	if build.InstallItems == nil {
		build.InstallItems = make([]*commonmodels.Item, 0)
	}

	build.JobCtx.Builds = module.SafeRepos()
	for _, repo := range build.JobCtx.Builds {
		repoInfo, err := systemconfig.New().GetCodeHost(repo.CodehostID)
		if err != nil {
			log.Errorf("Failed to get proxy settings for codehost ID: %d, the error is: %s", repo.CodehostID, err)
			return nil, err
		}
		repo.EnableProxy = repoInfo.EnableProxy
		repo.RepoNamespace = repo.GetRepoNamespace()
	}
	if len(build.JobCtx.Builds) == 0 {
		build.JobCtx.Builds = make([]*types.Repository, 0)
	}

	build.JobCtx.BuildSteps = []*taskmodels.BuildStep{}
	if module.Scripts != "" {
		build.JobCtx.BuildSteps = append(build.JobCtx.BuildSteps, &taskmodels.BuildStep{BuildType: "shell", Scripts: module.Scripts})
	}

	if module.PMDeployScripts != "" && build.ServiceType == setting.PMDeployType {
		build.JobCtx.PMDeployScripts = module.PMDeployScripts
	}

	if len(module.SSHs) > 0 && build.ServiceType == setting.PMDeployType {
		privateKeys := make([]*taskmodels.SSH, 0)
		for _, sshID := range module.SSHs {
			//私钥信息可能被更新，而构建中存储的信息是旧的，需要根据id获取最新的私钥信息
			latestKeyInfo, err := commonrepo.NewPrivateKeyColl().Find(commonrepo.FindPrivateKeyOption{ID: sshID})
			if err != nil || latestKeyInfo == nil {
				log.Errorf("PrivateKey.Find failed, id:%s, err:%s", sshID, err)
				continue
			}
			ssh := new(taskmodels.SSH)
			ssh.Name = latestKeyInfo.Name
			ssh.UserName = latestKeyInfo.UserName
			ssh.IP = latestKeyInfo.IP
			ssh.Port = latestKeyInfo.Port
			if ssh.Port == 0 {
				ssh.Port = setting.PMHostDefaultPort
			}
			ssh.PrivateKey = latestKeyInfo.PrivateKey

			privateKeys = append(privateKeys, ssh)
			delete(envHostInfos, sshID)
		}
		build.JobCtx.SSHs = privateKeys
	}
	// Sync from the configuration in the environment
	for _, privateKey := range envHostInfos {
		build.JobCtx.SSHs = append(build.JobCtx.SSHs, &taskmodels.SSH{
			Name:       privateKey.Name,
			UserName:   privateKey.UserName,
			IP:         privateKey.IP,
			Port:       privateKey.Port,
			PrivateKey: privateKey.PrivateKey,
		})
	}

	build.JobCtx.EnvVars = module.PreBuild.Envs

	if len(build.JobCtx.EnvVars) == 0 {
		build.JobCtx.EnvVars = make([]*commonmodels.KeyVal, 0)
	}

	if len(args.Variables) > 0 {
		for _, envVar := range build.JobCtx.EnvVars {
			for _, overwrite := range args.Variables {
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
		dockerTemplateContent := ""
		if module.PostBuild.DockerBuild.TemplateID != "" {
			if dockerfileDetail, err := templ.GetDockerfileTemplateDetail(module.PostBuild.DockerBuild.TemplateID, log); err == nil {
				dockerTemplateContent = dockerfileDetail.Content
			}
		}
		build.JobCtx.DockerBuildCtx = &taskmodels.DockerBuildCtx{
			Source:                module.PostBuild.DockerBuild.Source,
			WorkDir:               module.PostBuild.DockerBuild.WorkDir,
			DockerFile:            module.PostBuild.DockerBuild.DockerFile,
			BuildArgs:             module.PostBuild.DockerBuild.BuildArgs,
			DockerTemplateContent: dockerTemplateContent,
		}
	}

	if module.PostBuild != nil && module.PostBuild.FileArchive != nil {
		build.JobCtx.FileArchiveCtx = &taskmodels.FileArchiveCtx{
			FileLocation: module.PostBuild.FileArchive.FileLocation,
		}
	}

	if module.PostBuild != nil && module.PostBuild.Scripts != "" {
		build.JobCtx.PostScripts = module.PostBuild.Scripts
	}

	if module.PostBuild != nil && module.PostBuild.ObjectStorageUpload != nil {
		build.JobCtx.UploadEnabled = module.PostBuild.ObjectStorageUpload.Enabled
		if module.PostBuild.ObjectStorageUpload.Enabled {
			storageInfo, err := commonrepo.NewS3StorageColl().Find(module.PostBuild.ObjectStorageUpload.ObjectStorageID)
			if err != nil {
				log.Errorf("Failed to get basic storage info for uploading, the error is %s", err)
				return nil, err
			}
			build.JobCtx.UploadStorageInfo = &types.ObjectStorageInfo{
				Endpoint: storageInfo.Endpoint,
				AK:       storageInfo.Ak,
				SK:       storageInfo.Sk,
				Bucket:   storageInfo.Bucket,
				Insecure: storageInfo.Insecure,
				Provider: storageInfo.Provider,
			}
			build.JobCtx.UploadInfo = module.PostBuild.ObjectStorageUpload.UploadDetail
		}
	}

	build.JobCtx.Caches = module.Caches

	if args.FileName != "" {
		build.ArtifactInfo = &taskmodels.ArtifactInfo{
			URL:          args.URL,
			WorkflowName: args.WorkflowName,
			TaskID:       args.TaskID,
			FileName:     args.FileName,
		}
	}

	bst, err := build.ToSubTask()
	if err != nil {
		return subTasks, e.ErrConvertSubTasks.AddErr(err)
	}
	subTasks = append(subTasks, bst)

	return subTasks, nil
}

func extractHost(privateKeys []*commonmodels.PrivateKey, ips, names sets.String, envHostInfos map[string]*commonmodels.PrivateKey) (sets.String, sets.String) {
	for _, privateKey := range privateKeys {
		ips.Insert(privateKey.IP)
		names.Insert(privateKey.Name)
		envHostInfos[privateKey.ID.Hex()] = privateKey
	}
	return ips, names
}

func getServiceNaming(projectName, envName, serviceName string) (string, error) {
	productInfo, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    projectName,
		EnvName: envName,
	})
	if err != nil {
		return "", err
	}
	if productInfo.Source != setting.SourceFromHelm {
		return "", nil
	}
	svcMap := productInfo.GetServiceMap()
	pSvc, ok := svcMap[serviceName]
	if !ok {
		return "", fmt.Errorf("can't find service: %s in product: %s:%s", serviceName, projectName, envName)
	}
	templateSvc, err := commonrepo.NewServiceColl().Find(&commonrepo.ServiceFindOption{
		ServiceName: serviceName,
		Revision:    pSvc.Revision,
		Type:        setting.HelmDeployType,
		ProductName: projectName,
	})
	if err != nil {
		return "", nil
	}
	return util.GeneReleaseName(templateSvc.GetReleaseNaming(), projectName, productInfo.Namespace, envName, serviceName), nil
}

func getLatestWorkflowTask(workflowName string) (*task.Task, error) {
	resp, err := commonrepo.NewTaskColl().FindLatestTask(&commonrepo.FindTaskOption{
		PipelineName: workflowName,
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func ensurePipelineTask(taskOpt *taskmodels.TaskOpt, log *zap.SugaredLogger) error {
	var (
		buildEnvs []*commonmodels.KeyVal
	)

	// 验证 Subtask payload
	err := validateSubTaskSetting(taskOpt.Task.PipelineName, taskOpt.Task.SubTasks)
	if err != nil {
		log.Errorf("Validate subtask setting failed: %+v", err)
		return err
	}

	//设置执行任务时参数
	for i, subTask := range taskOpt.Task.SubTasks {

		pre, err := base.ToPreview(subTask)
		if err != nil {
			return errors.New(e.InterfaceToTaskErrMsg)
		}

		switch pre.TaskType {

		case config.TaskBuild, config.TaskArtifactDeploy, config.TaskBuildV3:
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
					taskOpt.Task.ServiceName = t.ServiceName
				}

				// 设置 build 安装脚本
				t.InstallCtx, err = BuildInstallCtx(t.InstallItems)
				if err != nil {
					log.Error(err)
					return err
				}

				// 外部触发的pipeline
				if taskOpt.Task.TaskCreator == setting.WebhookTaskCreator || taskOpt.Task.TaskCreator == setting.CronTaskCreator {
					SetTriggerBuilds(t.JobCtx.Builds, taskOpt.Task.TaskArgs.Builds, log)
				} else {
					setManunalBuilds(t.JobCtx.Builds, taskOpt.Task.TaskArgs.Builds, log)
				}

				opt := &commonrepo.ProductFindOptions{EnvName: taskOpt.EnvName, Name: taskOpt.Task.ProductName}
				exitedProd, err := commonrepo.NewProductColl().Find(opt)
				if err != nil {
					log.Errorf("can't find product by envName:%s error msg: %s", taskOpt.EnvName, err)
					return e.ErrFindRegistry.AddDesc(err.Error())
				}

				// 生成默认镜像tag后缀
				//pt.TaskArgs.Deploy.Tag = releaseCandidate(t, pt.TaskID, pt.ProductName, pt.EnvName, "image")

				// 设置镜像名称
				// 编译任务使用 t.JobCtx.Image
				// 注意: 其他任务从 pt.TaskArgs.Deploy.Image 获取, 必须要有编译任务
				var reg *commonmodels.RegistryNamespace
				if len(exitedProd.RegistryID) > 0 {
					reg, _, err = commonservice.FindRegistryById(exitedProd.RegistryID, true, log)
					if err != nil {
						log.Errorf("service.EnsureRegistrySecret: failed to find registry: %s error msg:%s",
							exitedProd.RegistryID, err)
						return e.ErrFindRegistry.AddDesc(err.Error())
					}
				} else {
					reg, _, err = commonservice.FindDefaultRegistry(true, log)
					if err != nil {
						log.Errorf("can't find default candidate registry: %s", err)
						return e.ErrFindRegistry.AddDesc(err.Error())
					}
				}
				t.JobCtx.Image = GetImage(reg, commonservice.ReleaseCandidate(t.JobCtx.Builds, taskOpt.Task.TaskID, taskOpt.Task.ProductName, t.ServiceName, taskOpt.EnvName, taskOpt.ImageName, "image"))
				taskOpt.Task.TaskArgs.Deploy.Image = t.JobCtx.Image

				if taskOpt.ServiceName != "" {
					*taskOpt.ServiceInfos = append(*taskOpt.ServiceInfos, &taskmodels.ServiceInfo{
						ServiceName:   taskOpt.ServiceName,
						ServiceModule: t.ServiceName,
						Image:         t.JobCtx.Image,
					})
				}

				if taskOpt.Task.ConfigPayload != nil {
					taskOpt.Task.ConfigPayload.Registry.Addr = reg.RegAddr
					taskOpt.Task.ConfigPayload.Registry.AccessKey = reg.AccessKey
					taskOpt.Task.ConfigPayload.Registry.SecretKey = reg.SecretKey
					taskOpt.Task.ConfigPayload.Registry.Namespace = reg.Namespace
				}

				// 二进制文件名称
				// 编译任务使用 t.JobCtx.PackageFile
				// 注意: 其他任务从 pt.TaskArgs.Deploy.PackageFile 获取, 必须要有编译任务
				t.JobCtx.PackageFile = GetPackageFile(commonservice.ReleaseCandidate(t.JobCtx.Builds, taskOpt.Task.TaskID, taskOpt.Task.ProductName, t.ServiceName, taskOpt.EnvName, taskOpt.ImageName, "tar"))
				taskOpt.Task.TaskArgs.Deploy.PackageFile = t.JobCtx.PackageFile

				// 注入编译模块中用户定义环境变量
				// 注意: 需要在pt.TaskArgs.Deploy设置完之后再设置环境变量
				t.JobCtx.EnvVars = append(t.JobCtx.EnvVars, prepareTaskEnvs(taskOpt.Task, log)...)
				// 如果其他模块需要使用编译模块的环境变量进行渲染，需要设置buildEnvs
				buildEnvs = t.JobCtx.EnvVars

				if t.JobCtx.DockerBuildCtx != nil {
					t.JobCtx.DockerBuildCtx.ImageName = t.JobCtx.Image
				}

				if t.JobCtx.FileArchiveCtx != nil {
					//t.JobCtx.FileArchiveCtx.FileName = t.ServiceName
					t.JobCtx.FileArchiveCtx.FileName = t.JobCtx.PackageFile
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
				registryRepo := reg.RegAddr + "/" + reg.Namespace
				if reg.RegProvider == config.RegistryTypeAWS {
					registryRepo = reg.RegAddr
				}

				t.DockerBuildStatus = &taskmodels.DockerBuildStatus{
					ImageName:    t.JobCtx.Image,
					RegistryRepo: registryRepo,
				}

				t.UTStatus = &task.UTStatus{}
				t.StaticCheckStatus = &task.StaticCheckStatus{}
				t.BuildStatus = &task.BuildStatus{}
				if taskOpt.IsWorkflowTask {
					t.ServiceName = t.ServiceName + "_" + t.Service
				}
				taskOpt.Task.SubTasks[i], err = t.ToSubTask()

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
				for i, jenkinsBuildParams := range t.JenkinsBuildArgs.JenkinsBuildParams {
					if jenkinsBuildParams.Name != "IMAGE" {
						continue
					}

					if jenkinsBuildParams.AutoGenerate {
						opt := &commonrepo.ProductFindOptions{EnvName: taskOpt.EnvName, Name: taskOpt.Task.ProductName}
						exitedProd, err := commonrepo.NewProductColl().Find(opt)
						if err != nil {
							log.Errorf("can't find product by envName:%s error msg: %s", taskOpt.EnvName, err)
							return e.ErrFindRegistry.AddDesc(err.Error())
						}

						var reg *commonmodels.RegistryNamespace
						if len(exitedProd.RegistryID) > 0 {
							reg, _, err = commonservice.FindRegistryById(exitedProd.RegistryID, true, log)
							if err != nil {
								log.Errorf("service.EnsureRegistrySecret: failed to find registry: %s error msg:%s",
									exitedProd.RegistryID, err)
								return e.ErrFindRegistry.AddDesc(err.Error())
							}
						} else {
							reg, _, err = commonservice.FindDefaultRegistry(true, log)
							if err != nil {
								log.Errorf("can't find default candidate registry: %s", err)
								return e.ErrFindRegistry.AddDesc(err.Error())
							}
						}
						project, err := templaterepo.NewProductColl().Find(taskOpt.Task.ProductName)
						if err != nil {
							log.Errorf("find project err:%s", err)
							return err
						}

						if project.CustomImageRule != nil && project.CustomImageRule.JenkinsRule != "" {
							va := commonservice.Variable{
								SERVICE:    t.ServiceName,
								TIMESTAMP:  time.Now().Format("20060102150405"),
								IMAGE_NAME: taskOpt.ImageName,
								PROJECT:    taskOpt.Task.ProductName,
								ENV_NAME:   taskOpt.EnvName,
								TASK_ID:    strconv.FormatInt(taskOpt.Task.TaskID, 10),
							}
							tm, err := gotempl.New("jenkins").Parse(project.CustomImageRule.JenkinsRule)
							if err != nil {
								log.Errorf("Parse template err:%s", err)
								return err
							}
							var replaceRuleVariable = gotempl.Must(tm, err)
							payload := bytes.NewBufferString("")
							err = replaceRuleVariable.Execute(payload, va)
							if err != nil {
								log.Errorf("Execute template err:%s", err)
								return err
							}
							image = GetImage(reg, payload.String())
						} else {
							image = GetImage(reg, fmt.Sprintf("%s:%s", taskOpt.ImageName, time.Now().Format("20060102150405")))
						}

						t.JenkinsBuildArgs.JenkinsBuildParams[i].Value = image
						break
					} else if value, ok := jenkinsBuildParams.Value.(string); ok {
						image = value
						break
					}
				}
				if image == "" || !strings.Contains(image, ":") {
					return &ImageIllegal{}
				}

				taskOpt.Task.TaskArgs.Deploy.Image = image
				t.Image = image
				if taskOpt.IsWorkflowTask {
					t.ServiceName = t.ServiceName + "_" + t.Service
				}
				taskOpt.Task.SubTasks[i], err = t.ToSubTask()
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
				if taskOpt.Task.TaskArgs == nil {
					taskOpt.Task.TaskArgs = &commonmodels.TaskArgs{PipelineName: taskOpt.Task.WorkflowArgs.WorkflowName, TaskCreator: taskOpt.Task.WorkflowArgs.WorkflowTaskCreator}
				}
				registry := taskOpt.Task.ConfigPayload.RepoConfigs[taskOpt.Task.WorkflowArgs.RegistryID]
				if registry.RegProvider == config.RegistryTypeAWS {
					t.Image = fmt.Sprintf("%s/%s", util.TrimURLScheme(registry.RegAddr), t.Image)
				} else {
					t.Image = fmt.Sprintf("%s/%s/%s", util.TrimURLScheme(registry.RegAddr), registry.Namespace, t.Image)
				}
				taskOpt.Task.TaskArgs.Deploy.Image = t.Image
				t.RegistryID = taskOpt.Task.WorkflowArgs.RegistryID

				taskOpt.Task.SubTasks[i], err = t.ToSubTask()
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
				t.Image = taskOpt.Task.TaskArgs.Deploy.Image

				// 使用环境变量KEY渲染 docker build的各个参数
				for _, env := range buildEnvs {
					if !env.IsCredential {
						t.DockerFile = strings.Replace(t.DockerFile, fmt.Sprintf("$%s", env.Key), env.Value, -1)
						t.WorkDir = strings.Replace(t.WorkDir, fmt.Sprintf("$%s", env.Key), env.Value, -1)
						t.BuildArgs = strings.Replace(t.BuildArgs, fmt.Sprintf("$%s", env.Key), env.Value, -1)
					}
				}

				taskOpt.Task.SubTasks[i], err = t.ToSubTask()
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
				if taskOpt.Task.TaskCreator == setting.WebhookTaskCreator || taskOpt.Task.TaskCreator == setting.CronTaskCreator {
					SetTriggerBuilds(t.JobCtx.Builds, taskOpt.Task.TaskArgs.Test.Builds, log)
				} else {
					setManunalBuilds(t.JobCtx.Builds, taskOpt.Task.TaskArgs.Test.Builds, log)
				}

				//use log path
				if taskOpt.Task.ServiceName == "" {
					taskOpt.Task.ServiceName = t.TestName
				}

				// 设置敏感信息
				t.JobCtx.EnvVars = append(t.JobCtx.EnvVars, envs...)

				taskOpt.Task.SubTasks[i], err = t.ToSubTask()
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
				t.SetNamespace(taskOpt.Task.TaskArgs.Deploy.Namespace)
				image, err := getImageInfoFromWorkload(t.EnvName, t.ProductName, t.ServiceName, t.ContainerName)
				if err != nil {
					log.Error(err)
					return err
				}

				t.SetImage(image)

				taskOpt.Task.SubTasks[i], err = t.ToSubTask()
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
				t.SetImage(taskOpt.Task.TaskArgs.Deploy.Image)
				t.SetNamespace(taskOpt.Task.TaskArgs.Deploy.Namespace)

				containerName := t.ContainerName
				if taskOpt.IsWorkflowTask {
					containerName = strings.TrimSuffix(containerName, "_"+t.ServiceName)
				}

				// Task creator can be webhook trigger or cronjob trigger or validated user
				// Validated user includes both that user is granted write permission or user is the owner of this product
				if taskOpt.Task.TaskCreator == setting.WebhookTaskCreator ||
					taskOpt.Task.TaskCreator == setting.CronTaskCreator ||
					IsProductAuthed(taskOpt.Task.TaskCreator, t.Namespace, taskOpt.Task.ProductName, config.ProductWritePermission, log) {
					log.Infof("Validating permission passed. product:%s, owner:%s, task executed by: %s", taskOpt.Task.ProductName, t.Namespace, taskOpt.Task.TaskCreator)
				} else {
					log.Errorf("permission denied. product:%s, owner:%s, task executed by: %s", taskOpt.Task.ProductName, t.Namespace, taskOpt.Task.TaskCreator)
					return errors.New(e.ProductAccessDeniedErrMsg)
				}

				taskOpt.Task.SubTasks[i], err = t.ToSubTask()
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
				task.SetPackageFile(taskOpt.Task.TaskArgs.Deploy.PackageFile)

				if taskOpt.Task.TeamName == "" {
					task.ProductName = taskOpt.Task.ProductName
				} else {
					task.ProductName = taskOpt.Task.TeamName
				}

				task.ServiceName = taskOpt.Task.ServiceName

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
				taskOpt.Task.SubTasks[i], err = task.ToSubTask()
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
				t.SetImage(taskOpt.Task.TaskArgs.Deploy.Image)
				// 兼容老的任务配置
				if len(t.Releases) == 0 {
					found := false
					for id, v := range taskOpt.Task.ConfigPayload.RepoConfigs {
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
					if v, ok := taskOpt.Task.ConfigPayload.RepoConfigs[repoImage.RepoID]; ok {
						repoImage.Name = util.ReplaceRepo(taskOpt.Task.TaskArgs.Deploy.Image, v.RegAddr, v.Namespace)
						repoImage.Host = util.TrimURLScheme(v.RegAddr)
						repoImage.Namespace = v.Namespace
						repos = append(repos, repoImage)
					}
				}

				var distributes []*taskmodels.DistributeInfo
				for _, distribute := range t.DistributeInfo {
					d := &taskmodels.DistributeInfo{
						DeployEnabled:       distribute.DeployEnabled,
						DeployEnv:           distribute.DeployEnv,
						DeployNamespace:     distribute.DeployNamespace,
						DeployContainerName: taskOpt.Task.ServiceName, // This is a match for target.Name
						DeployServiceName:   taskOpt.ServiceName,      // this is a match for target.ServiceName
						DeployServiceType:   distribute.DeployServiceType,
						DeployClusterID:     distribute.DeployClusterID,
						RepoID:              distribute.RepoID,
					}
					if distribute.DeployEnabled {
						releaseNaming, err := getServiceNaming(taskOpt.Task.ProductName, distribute.DeployEnv, taskOpt.ServiceName)
						if err != nil {
							return fmt.Errorf("failed to find release naming for service: %s, err: %s", taskOpt.ServiceName, err)
						}
						d.ReleaseName = releaseNaming
					}
					if v, ok := taskOpt.Task.ConfigPayload.RepoConfigs[distribute.RepoID]; ok {
						d.Image = util.ReplaceRepo(taskOpt.Task.TaskArgs.Deploy.Image, v.RegAddr, v.Namespace)
					}
					distributes = append(distributes, d)
				}

				t.Releases = repos
				t.DistributeInfo = distributes
				if len(t.Releases) == 0 {
					t.Enabled = false
				} else {
					t.ImageRelease = t.Releases[0].Name
				}

				if len(t.DistributeInfo) == 0 {
					t.Enabled = false
				}

				taskOpt.Task.SubTasks[i], err = t.ToSubTask()
				if err != nil {
					log.Errorf("release task to subtask error: %s", err)
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
				if taskOpt.Task.TaskCreator == setting.WebhookTaskCreator || taskOpt.Task.TaskCreator == setting.CronTaskCreator {
					SetTriggerBuilds(t.Builds, taskOpt.Task.TaskArgs.Builds, log)
				} else {
					setManunalBuilds(t.Builds, taskOpt.Task.TaskArgs.Builds, log)
				}

				taskOpt.Task.SubTasks[i], err = t.ToSubTask()
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
				t.SetImageName(taskOpt.Task.TaskArgs.Deploy.Image)
				taskOpt.Task.SubTasks[i], err = t.ToSubTask()
				if err != nil {
					return err
				}
			}
		case config.TaskDistribute, config.TaskArtifactPackage:
		// do nothing
		case config.TaskTrigger:
			t, err := base.ToTriggerTask(subTask)
			if err != nil {
				log.Error(err)
				return err
			}

			if t.Enabled {
				taskOpt.Task.SubTasks[i], err = t.ToSubTask()
				if err != nil {
					return err
				}
			}
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

func getBuildName(workflow *commonmodels.Workflow, targetName, serviceName string) string {
	if workflow == nil {
		return ""
	}
	if workflow.BuildStage == nil {
		return ""
	}
	if !workflow.BuildStage.Enabled {
		return ""
	}
	for _, module := range workflow.BuildStage.Modules {
		if module.Target == nil {
			continue
		}
		if module.Target.ServiceName == serviceName && module.Target.ServiceModule == targetName {
			return module.Target.BuildName
		}
	}
	return ""
}

func getBuildModule(buildName, serviceName, serviceModule, productName string) (*commonmodels.Build, error) {
	opt := &commonrepo.BuildListOption{
		ServiceName: serviceName,
		ProductName: productName,
		Targets:     []string{serviceModule},
	}

	if len(serviceModule) > 0 {
		opt.Targets = []string{serviceModule}
	}

	modules, err := commonrepo.NewBuildColl().List(opt)
	if err != nil {
		return nil, err
	}
	// The service may be a shared service
	if len(modules) == 0 {
		opt.ProductName = ""
		modules, err = commonrepo.NewBuildColl().List(opt)
		if err != nil {
			return nil, err
		}
	}
	if len(modules) == 0 {
		return nil, fmt.Errorf("no build module found for %s/%s/%s", productName, serviceName, serviceModule)
	}

	for _, module := range modules {
		if module.Name == buildName {
			return module, nil
		}
	}
	// if default build not set but there is only one build, we use it as the default build.
	if len(modules) == 1 {
		return modules[0], nil
	}
	return nil, fmt.Errorf("multiple build module found for %s/%s/%s, please select one in workflow configuration", productName, serviceName, serviceModule)
}
