/*
Copyright 2025 The KodeRover Authors.

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

package job

import (
	"fmt"

	"golang.org/x/exp/slices"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/repository"
	commontypes "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/types"
	aslanUtil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/setting"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	helmtool "github.com/koderover/zadig/v2/pkg/tool/helmclient"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/types/job"
	"github.com/koderover/zadig/v2/pkg/util"
)

type DeployJobController struct {
	*BasicInfo

	jobSpec *commonmodels.ZadigDeployJobSpec
}

func CreateDeployJobController(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) (Job, error) {
	spec := new(commonmodels.ZadigDeployJobSpec)
	if err := commonmodels.IToi(job.Spec, spec); err != nil {
		return nil, fmt.Errorf("failed to create build job controller, error: %s", err)
	}

	basicInfo := &BasicInfo{
		name:        job.Name,
		jobType:     job.JobType,
		errorPolicy: job.ErrorPolicy,
		workflow:    workflow,
	}

	return DeployJobController{
		BasicInfo: basicInfo,
		jobSpec:   spec,
	}, nil
}

func (j DeployJobController) SetWorkflow(wf *commonmodels.WorkflowV4) {
	j.workflow = wf
}

func (j DeployJobController) GetSpec() interface{} {
	return j.jobSpec
}

func (j DeployJobController) Validate(isExecution bool) error {
	if err := aslanUtil.CheckZadigProfessionalLicense(); err != nil {
		if j.jobSpec.Production {
			return e.ErrLicenseInvalid.AddDesc("生产环境功能需要专业版才能使用")
		}

		for _, item := range j.jobSpec.DeployContents {
			if item == config.DeployVars || item == config.DeployConfig {
				return e.ErrLicenseInvalid.AddDesc("基础版仅能部署镜像")
			}
		}
	}

	if j.jobSpec.Source != config.SourceFromJob {
		return nil
	}
	jobRankMap := getJobRankMap(j.workflow.Stages)
	buildJobRank, ok := jobRankMap[j.jobSpec.JobName]
	if !ok || buildJobRank >= jobRankMap[j.name] {
		return fmt.Errorf("can not quote job %s in job %s", j.jobSpec.JobName, j.name)
	}

	return nil
}

func (j DeployJobController) Update(useUserInput bool, ticket *commonmodels.ApprovalTicket) error {
	latestJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}

	latestSpec := new(commonmodels.ZadigDeployJobSpec)
	if err := commonmodels.IToi(latestJob.Spec, latestSpec); err != nil {
		return fmt.Errorf("failed to decode apollo job spec, error: %s", err)
	}

	j.jobSpec.Production = latestSpec.Production
	project, err := templaterepo.NewProductColl().Find(j.workflow.Project)
	if err != nil {
		return fmt.Errorf("failed to find project %s, err: %v", j.workflow.Project, err)
	}
	if project.ProductFeature != nil {
		j.jobSpec.DeployType = project.ProductFeature.DeployType
	}
	j.jobSpec.SkipCheckRunStatus = latestSpec.SkipCheckRunStatus
	j.jobSpec.SkipCheckHelmWorkloadStatus = latestSpec.SkipCheckHelmWorkloadStatus
	j.jobSpec.DeployContents = latestSpec.DeployContents
	j.jobSpec.ServiceVariableConfig = latestSpec.ServiceVariableConfig
	j.jobSpec.DefaultServices = latestSpec.DefaultServices

	// source is a bit tricky: if the saved args has a source of fromjob, but it has been change to runtime in the config
	// we need to not only update its source but also set services to empty slice.
	if j.jobSpec.Source == config.SourceFromJob && latestSpec.Source == config.SourceRuntime {
		j.jobSpec.Services = make([]*commonmodels.DeployServiceInfo, 0)
	}
	j.jobSpec.Source = latestSpec.Source

	if j.jobSpec.Source == config.SourceFromJob {
		j.jobSpec.OriginJobName = latestSpec.JobName
	}

	if !useUserInput {
		j.jobSpec.Env = latestSpec.Env
		if !ticket.IsAllowedEnv(j.workflow.Project, j.jobSpec.Env) {
			j.jobSpec.Env = ""
		}
		products, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
			Name:       j.workflow.Project,
			Production: util.GetBoolPointer(j.jobSpec.Production),
		})
		if err != nil {
			log.Errorf("can't list envs in project %s, error: %w", j.workflow.Project, err)
			return err
		}

		currentEnvMap := make(map[string]*commonmodels.Product)
		for _, env := range products {
			currentEnvMap[env.EnvName] = env
		}

		if _, ok := currentEnvMap[j.jobSpec.Env]; !ok {
			j.jobSpec.Env = ""
		}

		// if unselected for some reason, we skip calculating default service
		if j.jobSpec.Env == "" {
			j.jobSpec.Services = make([]*commonmodels.DeployServiceInfo, 0)
			return nil
		}

		envDeployInfo, err := generateDeployInfoForEnv(j.jobSpec.Env, j.workflow.Project, j.jobSpec.Production, j.jobSpec.ServiceVariableConfig, ticket)
		if err != nil {
			log.Errorf("failed to generate service deployment info for env: %s, error: %s", j.jobSpec.Env, err)
			return err
		}

		userConfiguredService := make(map[string]*commonmodels.ServiceAndImage)
		for _, service := range j.jobSpec.DefaultServices {
			userConfiguredService[service.ServiceName] = service
		}

		availableSvcOption := make([]*commonmodels.ServiceAndImage, 0)
		for _, service := range envDeployInfo.Services {
			if userSvcOption, ok := userConfiguredService[service.ServiceName]; ok {
				availableSvcOption = append(availableSvcOption, userSvcOption)
			}
		}

		j.jobSpec.DefaultServices = availableSvcOption
		return nil
	}

	if !ticket.IsAllowedEnv(j.workflow.Project, j.jobSpec.Env) {
		return fmt.Errorf("update workflow spec is denied: approvalTicket: [%s] does not allow deploying to env [%s]", ticket.ApprovalID, j.jobSpec.Env)
	}

	if j.jobSpec.Env != latestSpec.Env && latestSpec.EnvSource == config.ParamSourceFixed {
		return fmt.Errorf("update workflow spec is denied: configuration in job is [%s] and fixed, it does not allow deploying to env [%s]", latestSpec.Env, j.jobSpec.EnvSource)
	}

	envDeployInfo, err := generateDeployInfoForEnv(j.jobSpec.Env, j.workflow.Project, j.jobSpec.Production, j.jobSpec.ServiceVariableConfig, ticket)
	if err != nil {
		log.Errorf("failed to generate service deployment info for env: %s, error: %s", j.jobSpec.Env, err)
		return err
	}

	userConfiguredService := make(map[string]*commonmodels.DeployServiceInfo)
	for _, service := range j.jobSpec.Services {
		userConfiguredService[service.ServiceName] = service
	}

	mergedService := make([]*commonmodels.DeployServiceInfo, 0)

	for _, service := range envDeployInfo.Services {
		if userSvc, ok := userConfiguredService[service.ServiceName]; ok {
			// if the user wants to update config/variables do the merge variables logic, otherwise do nothing just add it to the user's selection
			if !(slices.Contains(j.jobSpec.DeployContents, config.DeployImage) && len(j.jobSpec.DeployContents) == 1) {
				// merge the kv based on user's selection weather config should be updated
				userKVMap := make(map[string]*commontypes.RenderVariableKV)
				for _, userKV := range userSvc.VariableKVs {
					userKVMap[userKV.Key] = userKV
				}

				newUserKV := make([]*commontypes.RenderVariableKV, 0)

				var variableInfo *commonmodels.DeployVariableInfo
				if userSvc.UpdateConfig {
					variableInfo = service.ServiceVariable
				} else {
					variableInfo = service.EnvVariable
				}

				for _, kv := range variableInfo.VariableKVs {
					updatedKV := &commontypes.RenderVariableKV{
						ServiceVariableKV: kv.ServiceVariableKV,
						UseGlobalVariable: kv.UseGlobalVariable,
					}
					if userKV, ok := userKVMap[kv.Key]; ok {
						if !kv.UseGlobalVariable {
							updatedKV.Value = userKV.Value
						}
					}
					newUserKV = append(newUserKV, updatedKV)
				}

				mergedValues, err := helmtool.MergeOverrideValues("", variableInfo.VariableYaml, userSvc.VariableYaml, "", make([]*helmtool.KV, 0))
				if err != nil {
					return fmt.Errorf("failed to merge helm values, error: %s", err)
				}

				userSvc.VariableYaml = mergedValues
				userSvc.VariableKVs = newUserKV
			}
			mergedService = append(mergedService, userSvc)
		}
	}
	j.jobSpec.Services = mergedService
	return nil
}

func (j DeployJobController) SetOptions(ticket *commonmodels.ApprovalTicket) error {
	envOptions := make([]*commonmodels.ZadigDeployEnvInformation, 0)

	if j.jobSpec.EnvSource == config.ParamSourceFixed {
		if ticket.IsAllowedEnv(j.workflow.Project, j.jobSpec.Env) {
			envInfo, err := generateDeployInfoForEnv(j.jobSpec.Env, j.workflow.Project, j.jobSpec.Production, j.jobSpec.ServiceVariableConfig, ticket)
			if err != nil {
				log.Errorf("failed to generate service deployment info for env: %s, error: %s", j.jobSpec.Env, err)
				return err
			}

			envOptions = append(envOptions, envInfo)
		}
	} else {
		products, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
			Name:       j.workflow.Project,
			Production: util.GetBoolPointer(j.jobSpec.Production),
		})
		if err != nil {
			log.Errorf("can't list envs in project %s, error: %w", j.workflow.Project, err)
			return err
		}

		for _, env := range products {
			// skip the sleeping envs
			if env.IsSleeping() {
				continue
			}

			if ticket.IsAllowedEnv(j.workflow.Project, env.EnvName) {
				continue
			}

			envInfo, err := generateDeployInfoForEnv(j.jobSpec.Env, j.workflow.Project, j.jobSpec.Production, j.jobSpec.ServiceVariableConfig, ticket)
			if err != nil {
				log.Errorf("failed to generate service deployment info for env: %s, error: %s", j.jobSpec.Env, err)
				return err
			}

			envOptions = append(envOptions, envInfo)
		}
	}

	j.jobSpec.EnvOptions = envOptions
	return nil
}

func (j DeployJobController) ClearOptions() {
	j.jobSpec.EnvOptions = nil
}

func (j DeployJobController) ClearSelection() {
	j.jobSpec.Services = make([]*commonmodels.DeployServiceInfo, 0)
}

func (j DeployJobController) ToTask(taskID int64) ([]*commonmodels.JobTask, error) {
	resp := make([]*commonmodels.JobTask, 0)

	product, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{Name: j.workflow.Project, EnvName: j.jobSpec.Env})
	if err != nil {
		return nil, fmt.Errorf("env %s not exists", j.jobSpec.Env)
	}

	project, err := templaterepo.NewProductColl().Find(j.workflow.Project)
	if err != nil {
		return nil, fmt.Errorf("failed to find project %s, err: %v", j.workflow.Project, err)
	}

	productServiceMap := product.GetServiceMap()

	// get deploy info from previous build job
	if j.jobSpec.Source == config.SourceFromJob {
		// adapt to the front end, use the direct quoted job name
		if j.jobSpec.OriginJobName != "" {
			j.jobSpec.JobName = j.jobSpec.OriginJobName
		}

		serviceReferredJob := getOriginJobName(j.workflow, j.jobSpec.JobName)
		deployOrder, err := j.getReferredJobOrder(serviceReferredJob, j.jobSpec.JobName)
		if err != nil {
			return resp, fmt.Errorf("get origin refered job: %s targets failed, err: %v", serviceReferredJob, err)
		}

		configurationServiceMap := make(map[string]*commonmodels.DeployServiceInfo)
		for _, svc := range j.jobSpec.Services {
			configurationServiceMap[svc.ServiceName] = svc
		}

		deployServiceMap := make(map[string]*commonmodels.ServiceWithModuleAndImage)
		deployModuleMap := make(map[string]int)
		for _, svc := range deployOrder {
			deployServiceMap[svc.ServiceName] = svc
			for _, module := range svc.ServiceModules {
				key := fmt.Sprintf("%s++%s", svc.ServiceName, module.ServiceModule)
				deployModuleMap[key] = 1
			}
		}

		for _, service := range j.jobSpec.Services {
			moduleList := make([]*commonmodels.DeployModuleInfo, 0)

			deployService, ok := deployServiceMap[service.ServiceName]
			if !ok {
				continue
			}

			for _, module := range deployService.ServiceModules {
				key := fmt.Sprintf("%s++%s", service.ServiceName, module.ServiceModule)
				if _, ok := deployModuleMap[key]; ok {
					moduleList = append(moduleList, module)
				}
			}
			service.Modules = moduleList
		}
	}

	serviceMap := map[string]*commonmodels.DeployServiceInfo{}
	for _, service := range j.jobSpec.Services {
		serviceMap[service.ServiceName] = service
	}

	templateProduct, err := templaterepo.NewProductColl().Find(j.workflow.Project)
	if err != nil {
		return nil, fmt.Errorf("cannot find product %s: %w", j.workflow.Project, err)
	}
	timeout := templateProduct.Timeout * 60

	if j.jobSpec.DeployType == setting.K8SDeployType {
		for jobSubTaskID, svc := range j.jobSpec.Services {
			serviceName := svc.ServiceName
			jobTaskSpec := &commonmodels.JobTaskDeploySpec{
				Env:                j.jobSpec.Env,
				SkipCheckRunStatus: j.jobSpec.SkipCheckRunStatus,
				ServiceName:        serviceName,
				ServiceType:        setting.K8SDeployType,
				CreateEnvType:      project.ProductFeature.CreateEnvType,
				ClusterID:          product.ClusterID,
				Production:         j.jobSpec.Production,
				DeployContents:     j.jobSpec.DeployContents,
				Timeout:            timeout,
			}

			for _, module := range svc.Modules {
				// if external env, check service exists
				if project.IsHostProduct() {
					if err := checkServiceExistsInEnv(productServiceMap, serviceName, j.jobSpec.Env); err != nil {
						return nil, err
					}
				}
				jobTaskSpec.ServiceAndImages = append(jobTaskSpec.ServiceAndImages, &commonmodels.DeployServiceModule{
					Image:         module.Image,
					ImageName:     module.ImageName,
					ServiceModule: module.ServiceModule,
				})
			}
			if !project.IsHostProduct() {
				jobTaskSpec.DeployContents = j.jobSpec.DeployContents
				jobTaskSpec.Production = j.jobSpec.Production
				service := serviceMap[serviceName]
				if service != nil {
					jobTaskSpec.UpdateConfig = service.UpdateConfig
					jobTaskSpec.VariableKVs = service.VariableKVs

					serviceRender := product.GetSvcRender(serviceName)
					svcRenderVarMap := map[string]*commontypes.RenderVariableKV{}
					for _, varKV := range serviceRender.OverrideYaml.RenderVariableKVs {
						svcRenderVarMap[varKV.Key] = varKV
					}

					// filter variables that used global variable
					filteredKV := []*commontypes.RenderVariableKV{}
					for _, jobKV := range jobTaskSpec.VariableKVs {
						svcKV, ok := svcRenderVarMap[jobKV.Key]
						if !ok {
							// deploy new variable
							filteredKV = append(filteredKV, jobKV)
							continue
						}
						// deploy existed variable
						if svcKV.UseGlobalVariable {
							continue
						}
						filteredKV = append(filteredKV, jobKV)
					}
					jobTaskSpec.VariableKVs = filteredKV
				}
				// if only deploy images, clear keyvals
				if slices.Contains(j.jobSpec.DeployContents, config.DeployImage) && len(j.jobSpec.DeployContents) == 1 {
					jobTaskSpec.VariableKVs = []*commontypes.RenderVariableKV{}
				}
			}
			jobTask := &commonmodels.JobTask{
				Key:         genJobKey(j.name, serviceName),
				Name:        GenJobName(j.workflow, j.name, jobSubTaskID),
				DisplayName: genJobDisplayName(j.name, serviceName),
				OriginName:  j.name,
				JobInfo: map[string]string{
					JobNameKey:     j.name,
					"service_name": serviceName,
				},
				JobType:     string(config.JobZadigDeploy),
				Spec:        jobTaskSpec,
				ErrorPolicy: j.errorPolicy,
			}
			if jobTaskSpec.CreateEnvType == "system" {
				var updateRevision bool
				if slices.Contains(jobTaskSpec.DeployContents, config.DeployConfig) && jobTaskSpec.UpdateConfig {
					updateRevision = true
				}

				varsYaml := ""
				varKVs := []*commontypes.RenderVariableKV{}
				if slices.Contains(jobTaskSpec.DeployContents, config.DeployVars) {
					varsYaml, err = commontypes.RenderVariableKVToYaml(jobTaskSpec.VariableKVs, true)
					if err != nil {
						return nil, fmt.Errorf("generate vars yaml error: %v", err)
					}
					varKVs = jobTaskSpec.VariableKVs
				}
				containers := []*commonmodels.Container{}
				if slices.Contains(jobTaskSpec.DeployContents, config.DeployImage) {
					if j.jobSpec.Source == config.SourceFromJob {
						for _, serviceImage := range jobTaskSpec.ServiceAndImages {
							containers = append(containers, &commonmodels.Container{
								Name:      serviceImage.ServiceModule,
								Image:     "{{ NOT BE RENDERED }}",
								ImageName: "{{ NOT BE RENDERED }}",
							})
						}
					} else {
						for _, serviceImage := range jobTaskSpec.ServiceAndImages {
							containers = append(containers, &commonmodels.Container{
								Name:      serviceImage.ServiceModule,
								Image:     serviceImage.Image,
								ImageName: util.ExtractImageName(serviceImage.Image),
							})
						}
					}
				}

				option := &kube.GeneSvcYamlOption{
					ProductName:           j.workflow.Project,
					EnvName:               jobTaskSpec.Env,
					ServiceName:           jobTaskSpec.ServiceName,
					UpdateServiceRevision: updateRevision,
					VariableYaml:          varsYaml,
					VariableKVs:           varKVs,
					Containers:            containers,
				}
				updatedYaml, _, _, err := kube.GenerateRenderedYaml(option)
				if err != nil {
					return nil, fmt.Errorf("generate service yaml error: %v", err)
				}
				jobTaskSpec.YamlContent = updatedYaml
			}

			for _, image := range jobTaskSpec.ServiceAndImages {
				log.Infof("DeployJob ToJobs %d: workflow %s service %s, module %s, image %s",
					taskID, j.workflow.Name, serviceName, image.ServiceModule, image.Image)
			}
			resp = append(resp, jobTask)
		}
	}

	if j.jobSpec.DeployType == setting.HelmDeployType {
		for jobSubTaskID, svc := range j.jobSpec.Services {
			var serviceRevision int64
			if pSvc, ok := productServiceMap[svc.ServiceName]; ok {
				serviceRevision = pSvc.Revision
			}

			revisionSvc, err := repository.QueryTemplateService(&commonrepo.ServiceFindOption{
				ServiceName: svc.ServiceName,
				Revision:    serviceRevision,
				ProductName: product.ProductName,
			}, product.Production)
			if err != nil {
				return nil, fmt.Errorf("failed to find service: %s with revision: %d, err: %s", svc.ServiceName, serviceRevision, err)
			}
			releaseName := util.GeneReleaseName(revisionSvc.GetReleaseNaming(), product.ProductName, product.Namespace, product.EnvName, svc.ServiceName)

			jobTaskSpec := &commonmodels.JobTaskHelmDeploySpec{
				Env:                          j.jobSpec.Env,
				Source:                       j.jobSpec.Source,
				ServiceName:                  svc.ServiceName,
				DeployContents:               j.jobSpec.DeployContents,
				SkipCheckRunStatus:           j.jobSpec.SkipCheckRunStatus,
				SkipCheckHelmWorkfloadStatus: j.jobSpec.SkipCheckHelmWorkloadStatus,
				ServiceType:                  setting.HelmDeployType,
				ClusterID:                    product.ClusterID,
				ReleaseName:                  releaseName,
				Timeout:                      timeout,
				IsProduction:                 j.jobSpec.Production,
			}

			for _, module := range svc.Modules {
				service := serviceMap[svc.ServiceName]
				if service != nil {
					jobTaskSpec.UpdateConfig = service.UpdateConfig
					jobTaskSpec.VariableYaml = service.VariableYaml
					jobTaskSpec.UserSuppliedValue = jobTaskSpec.VariableYaml
				}

				jobTaskSpec.ImageAndModules = append(jobTaskSpec.ImageAndModules, &commonmodels.ImageAndServiceModule{
					ServiceModule: module.ServiceModule,
					Image:         module.Image,
				})
			}
			jobTask := &commonmodels.JobTask{
				Key:         genJobKey(j.name, svc.ServiceName),
				Name:        GenJobName(j.workflow, j.name, jobSubTaskID),
				DisplayName: genJobDisplayName(j.name, svc.ServiceName),
				OriginName:  j.name,
				JobInfo: map[string]string{
					JobNameKey:     j.name,
					"service_name": svc.ServiceName,
				},
				JobType:     string(config.JobZadigHelmDeploy),
				Spec:        jobTaskSpec,
				ErrorPolicy: j.errorPolicy,
			}
			resp = append(resp, jobTask)
		}
	}

	return resp, nil
}

func (j DeployJobController) SetRepo(repo *types.Repository) error {
	return nil
}

func (j DeployJobController) SetRepoCommitInfo() error {
	return nil
}

func (j DeployJobController) getReferredJobOrder(serviceReferredJob, imageReferredJob string) ([]*commonmodels.ServiceWithModuleAndImage, error) {
	resp := make([]*commonmodels.ServiceWithModuleAndImage, 0)
	found := false

	// first determine the services with order
ServiceOrderLoop:
	for _, stage := range j.workflow.Stages {
		for _, job := range stage.Jobs {
			if job.Name != serviceReferredJob {
				continue
			}
			if job.JobType == config.JobZadigBuild {
				buildSpec := &commonmodels.ZadigBuildJobSpec{}
				if err := commonmodels.IToi(job.Spec, buildSpec); err != nil {
					return nil, err
				}

				order := make([]string, 0)
				buildModuleMap := make(map[string][]*commonmodels.DeployModuleInfo)
				for _, build := range buildSpec.ServiceAndBuilds {
					if _, ok := buildModuleMap[build.ServiceName]; !ok {
						buildModuleMap[build.ServiceName] = make([]*commonmodels.DeployModuleInfo, 0)
						order = append(order, build.ServiceName)
					}

					buildModuleMap[build.ServiceName] = append(buildModuleMap[build.ServiceName], &commonmodels.DeployModuleInfo{
						ServiceModule: build.ServiceModule,
						//Image:         build.Image,
						//ImageName:     util.ExtractImageName(build.ImageName),
					})
				}

				for _, serviceName := range order {
					resp = append(resp, &commonmodels.ServiceWithModuleAndImage{
						ServiceName:    serviceName,
						ServiceModules: buildModuleMap[serviceName],
					})
				}
				found = true
				break ServiceOrderLoop
			}

			if job.JobType == config.JobZadigDistributeImage {
				distributeSpec := &commonmodels.ZadigDistributeImageJobSpec{}
				if err := commonmodels.IToi(job.Spec, distributeSpec); err != nil {
					return nil, err
				}
				order := make([]string, 0)
				distributeModuleMap := make(map[string][]*commonmodels.DeployModuleInfo)
				for _, distribute := range distributeSpec.Targets {
					if _, ok := distributeModuleMap[distribute.ServiceName]; !ok {
						distributeModuleMap[distribute.ServiceName] = make([]*commonmodels.DeployModuleInfo, 0)
						order = append(order, distribute.ServiceName)
					}

					distributeModuleMap[distribute.ServiceName] = append(distributeModuleMap[distribute.ServiceName], &commonmodels.DeployModuleInfo{
						ServiceModule: distribute.ServiceModule,
						//Image:         distribute.TargetImage,
						//ImageName:     util.ExtractImageName(distribute.ImageName),
					})
				}

				for _, serviceName := range order {
					resp = append(resp, &commonmodels.ServiceWithModuleAndImage{
						ServiceName:    serviceName,
						ServiceModules: distributeModuleMap[serviceName],
					})
				}
				found = true
				break ServiceOrderLoop
			}

			if job.JobType == config.JobZadigDeploy {
				deploySpec := &commonmodels.ZadigDeployJobSpec{}
				if err := commonmodels.IToi(job.Spec, deploySpec); err != nil {
					return nil, err
				}
				for _, service := range deploySpec.Services {
					resp = append(resp, &commonmodels.ServiceWithModuleAndImage{
						ServiceName:    service.ServiceName,
						ServiceModules: service.Modules,
					})
				}
				found = true
				break ServiceOrderLoop
			}
		}
	}

	if !found {
		return nil, fmt.Errorf("qutoed service referrece of job %s not found", serviceReferredJob)
	}

	// then we determine the image for the selected job, use the output for each module is enough
	for _, svc := range resp {
		for _, module := range svc.ServiceModules {
			// generate real job keys
			key := job.GetJobOutputKey(fmt.Sprintf("%s.%s.%s", imageReferredJob, svc.ServiceName, module.ServiceModule), IMAGEKEY)

			module.Image = key
		}
	}

	return resp, nil
}

// generateDeployInfoForEnv generates the whole environment deployment info for the given env, it contains:
// 1. basic environment information
// 2. ALL service information that is either in the environment or the service definition
// 3. 2 versions of variable list (filtered by the user's variable configuration) showing the service variable definition and the env variables (if applicable)
func generateDeployInfoForEnv(env, project string, production bool, configuredServiceVariableList commonmodels.DeployServiceVariableConfigList, approvalTicket *commonmodels.ApprovalTicket) (*commonmodels.ZadigDeployEnvInformation, error) {
	serviceOption := make([]*commonmodels.DeployOptionInfo, 0)

	envInfo, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:       project,
		EnvName:    env,
		Production: util.GetBoolPointer(production),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to find env: %s in environments, error: %s", env, err)
	}

	if envInfo.RegistryID == "" {
		registry, err := commonrepo.NewRegistryNamespaceColl().Find(&commonrepo.FindRegOps{
			IsDefault: true,
		})

		if err != nil {
			return nil, fmt.Errorf("failed to find default registry for env: %s, error: %s", env, err)
		}
		envInfo.RegistryID = registry.ID.Hex()
	}

	projectInfo, err := templaterepo.NewProductColl().Find(project)
	if err != nil {
		return nil, fmt.Errorf("failed to find project %s, err: %v", project, err)
	}

	envServiceMap := envInfo.GetServiceMap()

	if projectInfo.IsHostProduct() {
		for _, service := range envServiceMap {
			modules := make([]*commonmodels.DeployModuleInfo, 0)
			for _, module := range service.Containers {
				if approvalTicket.IsAllowedService(project, service.ServiceName, module.Name) {
					modules = append(modules, &commonmodels.DeployModuleInfo{
						ServiceModule: module.Name,
						Image:         module.Image,
						ImageName:     util.ExtractImageName(module.Image),
					})
				}
			}

			svcBasicInfo := commonmodels.DeployBasicInfo{
				ServiceName:  service.ServiceName,
				Modules:      modules,
				Deployed:     true,
				AutoSync:     false,
				UpdateConfig: false,
				Updatable:    false,
			}

			// for hosting project, config and variable is forbidden to use, so we give no variable info
			serviceOption = append(serviceOption, &commonmodels.DeployOptionInfo{
				DeployBasicInfo: svcBasicInfo,

				EnvVariable:     nil,
				ServiceVariable: nil,
			})
		}
		return &commonmodels.ZadigDeployEnvInformation{
			Env:        envInfo.EnvName,
			EnvAlias:   envInfo.Alias,
			Production: production,
			RegistryID: envInfo.RegistryID,
			Services:   serviceOption,
		}, nil
	}

	serviceDefinitionMap := make(map[string]*commonmodels.Service)
	var serviceDefinitions []*commonmodels.Service
	if production {
		serviceDefinitions, err = commonrepo.NewProductionServiceColl().ListMaxRevisions(&commonrepo.ServiceListOption{
			ProductName: project,
		})
	} else {
		serviceDefinitions, err = commonrepo.NewServiceColl().ListMaxRevisions(&commonrepo.ServiceListOption{
			ProductName: project,
		})
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list services, error: %s", err)
	}

	for _, service := range serviceDefinitions {
		serviceDefinitionMap[service.ServiceName] = service
	}

	serviceList, err := repository.ListMaxRevisionsServices(project, production)
	if err != nil {
		return nil, fmt.Errorf("get service definition list error: %v", err)
	}

	serviceGeneralInfo, err := commonservice.BuildServiceInfoInEnv(envInfo, serviceList, nil, log.SugaredLogger())
	if err != nil {
		return nil, fmt.Errorf("failed to generate service info, error: %s", err)
	}

	serviceGeneralInfoMap := make(map[string]*commonservice.EnvService)
	for _, service := range serviceGeneralInfo.Services {
		serviceGeneralInfoMap[service.ServiceName] = service
	}

	/*
	   1. Throw everything in the envs into the response
	   2. Comparing the service list in the envs with the service list in service definition to find extra (if not found just do nothing)
	   3. Do a scan for the services that is newly created in the service list

	   Additional logics:
	   1. If a new service is about to be added into the env, it bypasses the VariableConfig settings. Users should always see it.
	*/
	for _, service := range envServiceMap {
		modules := make([]*commonmodels.DeployModuleInfo, 0)
		modulesMap := make(map[string]string)
		for _, module := range service.Containers {
			modulesMap[module.Name] = module.Image
			if approvalTicket.IsAllowedService(project, service.ServiceName, module.Name) {
				modules = append(modules, &commonmodels.DeployModuleInfo{
					ServiceModule: module.Name,
					Image:         module.Image,
					ImageName:     util.ExtractImageName(module.Image),
				})
			}
		}

		if serviceDef, ok := serviceDefinitionMap[service.ServiceName]; ok {
			for _, module := range serviceDef.Containers {
				// if a container is newly created in the service, add it to the module list
				if _, ok := modulesMap[module.Name]; !ok {
					modules = append(modules, &commonmodels.DeployModuleInfo{
						ServiceModule: module.Name,
						Image:         module.Image,
						ImageName:     util.ExtractImageName(module.Image),
					})
				}
			}
		}

		serviceVariableConfigMap := make(map[string][]*commonmodels.DeployVariableConfig)
		for _, svc := range configuredServiceVariableList {
			serviceVariableConfigMap[svc.ServiceName] = svc.VariableConfigs
		}

		svcBasicInfo := commonmodels.DeployBasicInfo{
			ServiceName: service.ServiceName,
			Modules:     modules,
			Deployed:    true,
			AutoSync:    service.GetServiceRender().GetAutoSync(),
			Updatable:   serviceGeneralInfoMap[service.ServiceName].Updatable,
		}

		serviceVariableInfo := &commonmodels.DeployVariableInfo{
			VariableKVs:  filterKVsByConfig(service.ServiceName, serviceGeneralInfoMap[service.ServiceName].LatestVariableKVs, configuredServiceVariableList),
			OverrideKVs:  serviceGeneralInfoMap[service.ServiceName].OverrideKVs,
			VariableYaml: serviceGeneralInfoMap[service.ServiceName].LatestVariableYaml,
		}

		envVariableInfo := &commonmodels.DeployVariableInfo{
			VariableKVs:  filterKVsByConfig(service.ServiceName, serviceGeneralInfoMap[service.ServiceName].LatestVariableKVs, configuredServiceVariableList),
			OverrideKVs:  serviceGeneralInfoMap[service.ServiceName].OverrideKVs,
			VariableYaml: serviceGeneralInfoMap[service.ServiceName].VariableYaml,
		}

		serviceOption = append(serviceOption, &commonmodels.DeployOptionInfo{
			DeployBasicInfo: svcBasicInfo,

			EnvVariable:     envVariableInfo,
			ServiceVariable: serviceVariableInfo,
		})
	}

	for serviceName, service := range serviceDefinitionMap {
		if _, ok := envServiceMap[serviceName]; ok {
			continue
		}

		modules := make([]*commonmodels.DeployModuleInfo, 0)
		for _, module := range service.Containers {
			if approvalTicket.IsAllowedService(project, service.ServiceName, module.Name) {
				modules = append(modules, &commonmodels.DeployModuleInfo{
					ServiceModule: module.Name,
					Image:         module.Image,
					ImageName:     util.ExtractImageName(module.Image),
				})
			}
		}

		svcBasicInfo := commonmodels.DeployBasicInfo{
			ServiceName: service.ServiceName,
			Modules:     modules,
			Deployed:    true,
			AutoSync:    false,
			Updatable:   serviceGeneralInfoMap[service.ServiceName].Updatable,
		}

		kvs := make([]*commontypes.RenderVariableKV, 0)
		for _, kv := range service.ServiceVariableKVs {
			kvs = append(kvs, &commontypes.RenderVariableKV{
				ServiceVariableKV: *kv,
				UseGlobalVariable: false,
			})
		}

		serviceVariableInfo := &commonmodels.DeployVariableInfo{
			VariableKVs:  filterKVsByConfig(service.ServiceName, kvs, configuredServiceVariableList),
			OverrideKVs:  serviceGeneralInfoMap[service.ServiceName].OverrideKVs,
			VariableYaml: serviceGeneralInfoMap[service.ServiceName].LatestVariableYaml,
		}

		serviceOption = append(serviceOption, &commonmodels.DeployOptionInfo{
			DeployBasicInfo: svcBasicInfo,

			EnvVariable:     nil,
			ServiceVariable: serviceVariableInfo,
		})
	}

	return &commonmodels.ZadigDeployEnvInformation{
		Env:        envInfo.EnvName,
		EnvAlias:   envInfo.Alias,
		Production: production,
		RegistryID: envInfo.RegistryID,
		Services:   serviceOption,
	}, nil
}

// filterKVsByConfig Filters the given originKVs by certain logic:
// 1. if the service is not in the kvConfig list, skip filtering
// 2. if the length of the variable config is 0, which still means there is not limit, skip filtering
// 3. otherwise filter the originKVs list by the kvConfig
func filterKVsByConfig(serviceName string, originKVs []*commontypes.RenderVariableKV, kvConfig commonmodels.DeployServiceVariableConfigList) []*commontypes.RenderVariableKV {
	serviceVariableConfigMap := make(map[string][]*commonmodels.DeployVariableConfig)
	for _, svc := range kvConfig {
		serviceVariableConfigMap[svc.ServiceName] = svc.VariableConfigs
	}

	cfg, ok := serviceVariableConfigMap[serviceName]
	if !ok {
		return originKVs
	}
	if len(cfg) == 0 {
		return originKVs
	}

	kvCfgMap := make(map[string]*commonmodels.DeployVariableConfig)
	for _, kv := range cfg {
		kvCfgMap[kv.VariableKey] = kv
	}

	resp := make([]*commontypes.RenderVariableKV, 0)
	for _, originKV := range originKVs {
		if kvCfg, ok := kvCfgMap[originKV.Key]; ok {
			newKV := &commontypes.RenderVariableKV{
				ServiceVariableKV: originKV.ServiceVariableKV,
				UseGlobalVariable: false,
			}

			if kvCfg.Source == "other" {
				newKV.Value = kvCfg.Value
				newKV.UseGlobalVariable = true
			}

			resp = append(resp, newKV)
		}
	}

	return resp
}

func checkServiceExistsInEnv(serviceMap map[string]*commonmodels.ProductService, serviceName, env string) error {
	if _, ok := serviceMap[serviceName]; !ok {
		return fmt.Errorf("service %s not exists in env %s", serviceName, env)
	}
	return nil
}
