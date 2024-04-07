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

package job

import (
	"fmt"
	"strings"

	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/repository"
	commontypes "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/types"
	aslanUtil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/util"
)

const (
	ENVNAMEKEY = "envName"
)

type DeployJob struct {
	job      *commonmodels.Job
	workflow *commonmodels.WorkflowV4
	spec     *commonmodels.ZadigDeployJobSpec
}

func (j *DeployJob) Instantiate() error {
	j.spec = &commonmodels.ZadigDeployJobSpec{}
	if err := commonmodels.IToiYaml(j.job.Spec, j.spec); err != nil {
		return err
	}
	j.setDefaultDeployContent()
	j.job.Spec = j.spec
	return nil
}

func (j *DeployJob) setDefaultDeployContent() {
	if j.spec.DeployContents == nil || len(j.spec.DeployContents) <= 0 {
		j.spec.DeployContents = []config.DeployContent{config.DeployImage}
	}
}

func (j *DeployJob) getOriginReferredJobTargets(jobName string) ([]*commonmodels.ServiceAndImage, error) {
	serviceAndImages := []*commonmodels.ServiceAndImage{}
	for _, stage := range j.workflow.Stages {
		for _, job := range stage.Jobs {
			if job.Name != j.spec.JobName {
				continue
			}
			if job.JobType == config.JobZadigBuild {
				buildSpec := &commonmodels.ZadigBuildJobSpec{}
				if err := commonmodels.IToi(job.Spec, buildSpec); err != nil {
					return serviceAndImages, err
				}
				for _, build := range buildSpec.ServiceAndBuilds {
					serviceAndImages = append(serviceAndImages, &commonmodels.ServiceAndImage{
						ServiceName:   build.ServiceName,
						ServiceModule: build.ServiceModule,
						Image:         build.Image,
					})
					log.Infof("DeployJob ToJobs getOriginReferredJobTargets: workflow %s service %s, module %s, image %s",
						j.workflow.Name, build.ServiceName, build.ServiceModule, build.Image)
				}
				return serviceAndImages, nil
			}
			if job.JobType == config.JobZadigDistributeImage {
				distributeSpec := &commonmodels.ZadigDistributeImageJobSpec{}
				if err := commonmodels.IToi(job.Spec, distributeSpec); err != nil {
					return serviceAndImages, err
				}
				for _, distribute := range distributeSpec.Targets {
					serviceAndImages = append(serviceAndImages, &commonmodels.ServiceAndImage{
						ServiceName:   distribute.ServiceName,
						ServiceModule: distribute.ServiceModule,
						Image:         distribute.TargetImage,
					})
				}
				return serviceAndImages, nil
			}
			if job.JobType == config.JobZadigDeploy {
				deploySpec := &commonmodels.ZadigDeployJobSpec{}
				if err := commonmodels.IToi(job.Spec, deploySpec); err != nil {
					return serviceAndImages, err
				}
				for _, service := range deploySpec.Services {
					for _, module := range service.Modules {
						serviceAndImages = append(serviceAndImages, &commonmodels.ServiceAndImage{
							ServiceName:   service.ServiceName,
							ServiceModule: module.ServiceModule,
							Image:         module.Image,
						})
					}
				}
				return serviceAndImages, nil
			}
		}
	}
	return nil, fmt.Errorf("qutoed build/deploy job %s not found", jobName)
}

// SetPreset sets all info for the user-config env service.
func (j *DeployJob) SetPreset() error {
	j.spec = &commonmodels.ZadigDeployJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return err
	}
	j.setDefaultDeployContent()
	var err error
	project, err := templaterepo.NewProductColl().Find(j.workflow.Project)
	if err != nil {
		return fmt.Errorf("failed to find project %s, err: %v", j.workflow.Project, err)
	}
	if project.ProductFeature != nil {
		j.spec.DeployType = project.ProductFeature.DeployType
	}
	// if quoted job quote another job, then use the service and image of the quoted job
	if j.spec.Source == config.SourceFromJob {
		j.spec.OriginJobName = j.spec.JobName
		j.spec.JobName = getOriginJobName(j.workflow, j.spec.JobName)
	} else if j.spec.Source == config.SourceRuntime {
		envName := strings.ReplaceAll(j.spec.Env, setting.FixedValueMark, "")

		serviceDeployOption, err := generateEnvDeployServiceInfo(envName, j.workflow.Project, j.spec.Production, j.spec)
		if err != nil {
			log.Errorf("failed to generate service deployment info for env: %s, error: %s", envName, err)
			return err
		}

		configuredModulesMap := make(map[string]sets.String)
		for _, module := range j.spec.ServiceAndImages {
			if _, ok := configuredModulesMap[module.ServiceName]; !ok {
				configuredModulesMap[module.ServiceName] = sets.NewString()
			}

			configuredModulesMap[module.ServiceName].Insert(module.ServiceModule)
		}

		svcResp := make([]*commonmodels.DeployServiceInfo, 0)

		for _, svc := range serviceDeployOption {
			if modulesList, ok := configuredModulesMap[svc.ServiceName]; !ok {
				continue
			} else {
				// if configured, delete all the unnecessary modules
				selectedModules := make([]*commonmodels.DeployModuleInfo, 0)
				for _, module := range svc.Modules {
					if modulesList.Has(module.ServiceModule) {
						selectedModules = append(selectedModules, module)
					}
				}

				svcResp = append(svcResp, &commonmodels.DeployServiceInfo{
					ServiceName:       svc.ServiceName,
					VariableConfigs:   svc.VariableConfigs,
					VariableKVs:       svc.VariableKVs,
					LatestVariableKVs: svc.LatestVariableKVs,
					VariableYaml:      svc.VariableYaml,
					UpdateConfig:      svc.UpdateConfig,
					Updatable:         svc.Updatable,
					Deployed:          svc.Deployed,
					Modules:           selectedModules,
					KeyVals:           svc.KeyVals,
					LatestKeyVals:     svc.LatestKeyVals,
				})
			}
		}

		j.spec.Services = svcResp
	}

	j.job.Spec = j.spec
	return nil
}

// SetOptions get the service deployment info from ALL envs and set these information into the EnvOptions Field
func (j *DeployJob) SetOptions() error {
	j.spec = &commonmodels.ZadigDeployJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return err
	}

	envOptions := make([]*commonmodels.ZadigDeployEnvInformation, 0)

	if strings.HasPrefix(j.spec.Env, setting.FixedValueMark) {
		// if the env is fixed, we put the env in the option
		envName := strings.ReplaceAll(j.spec.Env, setting.FixedValueMark, "")

		serviceInfo, err := generateEnvDeployServiceInfo(envName, j.workflow.Project, j.spec.Production, j.spec)
		if err != nil {
			log.Errorf("failed to generate service deployment info for env: %s, error: %s", envName, err)
			return err
		}

		envOptions = append(envOptions, &commonmodels.ZadigDeployEnvInformation{
			Env:      envName,
			Services: serviceInfo,
		})
	} else {
		// otherwise list all the envs in this project
		products, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
			Name:       j.workflow.Project,
			Production: util.GetBoolPointer(j.spec.Production),
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

			serviceDeployOption, err := generateEnvDeployServiceInfo(env.EnvName, j.workflow.Project, j.spec.Production, j.spec)
			if err != nil {
				log.Errorf("failed to generate service deployment info for env: %s, error: %s", env.EnvName, err)
				return err
			}

			envOptions = append(envOptions, &commonmodels.ZadigDeployEnvInformation{
				Env:      env.EnvName,
				Services: serviceDeployOption,
			})
		}
	}

	j.spec.EnvOptions = envOptions
	j.job.Spec = j.spec
	return nil
}

func generateEnvDeployServiceInfo(env, project string, production bool, spec *commonmodels.ZadigDeployJobSpec) ([]*commonmodels.DeployServiceInfo, error) {
	resp := make([]*commonmodels.DeployServiceInfo, 0)
	envInfo, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:       project,
		EnvName:    env,
		Production: util.GetBoolPointer(production),
	})

	if err != nil {
		return nil, fmt.Errorf("failed to find fixed env: %s in environments, error: %s", env, err)
	}

	envServiceMap := envInfo.GetServiceMap()
	serviceDefinitionMap := make(map[string]*commonmodels.Service)
	serviceKVSettingMap := make(map[string][]*commonmodels.DeployVariableConfig)

	updateConfig := false
	for _, contents := range spec.DeployContents {
		if contents == config.DeployVars {
			updateConfig = true
		}
	}

	for _, svc := range spec.Services {
		serviceKVSettingMap[svc.ServiceName] = svc.VariableConfigs
	}

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

	/*
	   1. Throw everything in the envs into the response
	   2. Do a scan for the services that is newly created in the service list

	   Additional logics:
	   1. VariableConfig is the field user used to limit the range of kvs workflow user can see, it should not be returned.
	   2. If a new service is about to be added into the env, it bypasses the VariableConfig settings. Users should always see it.
	*/

	for _, service := range envServiceMap {
		modules := make([]*commonmodels.DeployModuleInfo, 0)
		for _, module := range service.Containers {
			modules = append(modules, &commonmodels.DeployModuleInfo{
				ServiceModule: module.Name,
				Image:         module.Image,
				ImageName:     commonutil.ExtractImageName(module.Image),
			})
		}

		kvs := make([]*commontypes.RenderVariableKV, 0)

		for _, kv := range service.VariableKVs {
			for _, configKV := range serviceKVSettingMap[service.ServiceName] {
				if kv.Key == configKV.VariableKey {
					kvs = append(kvs, kv)
				}
			}
		}

		resp = append(resp, &commonmodels.DeployServiceInfo{
			ServiceName:       service.ServiceName,
			VariableKVs:       kvs,
			LatestVariableKVs: service.VariableKVs,
			VariableYaml:      service.VariableYaml,
			UpdateConfig:      updateConfig,
			Updatable:         service.Updatable,
			Deployed:          true,
			Modules:           modules,
		})
	}

	for serviceName, service := range serviceDefinitionMap {
		if _, ok := envServiceMap[serviceName]; ok {
			continue
		}

		modules := make([]*commonmodels.DeployModuleInfo, 0)
		for _, module := range service.Containers {
			modules = append(modules, &commonmodels.DeployModuleInfo{
				ServiceModule: module.Name,
				Image:         module.Image,
				ImageName:     commonutil.ExtractImageName(module.Image),
			})
		}

		kvs := make([]*commontypes.RenderVariableKV, 0)
		for _, kv := range service.ServiceVariableKVs {
			kvs = append(kvs, &commontypes.RenderVariableKV{
				ServiceVariableKV: *kv,
				UseGlobalVariable: false,
			})
		}

		resp = append(resp, &commonmodels.DeployServiceInfo{
			ServiceName:  service.ServiceName,
			VariableKVs:  kvs,
			VariableYaml: service.VariableYaml,
			UpdateConfig: updateConfig,
			Updatable:    true,
			Deployed:     false,
			Modules:      modules,
		})
	}

	return resp, nil
}

func (j *DeployJob) MergeArgs(args *commonmodels.Job) error {
	if j.job.Name == args.Name && j.job.JobType == args.JobType {
		j.spec = &commonmodels.ZadigDeployJobSpec{}
		if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
			return err
		}
		j.job.Spec = j.spec
		argsSpec := &commonmodels.ZadigDeployJobSpec{}
		if err := commonmodels.IToi(args.Spec, argsSpec); err != nil {
			return err
		}
		j.spec.Env = argsSpec.Env
		j.spec.Services = argsSpec.Services
		if j.spec.Source == config.SourceRuntime {
			j.spec.Services = argsSpec.Services
		}

		j.job.Spec = j.spec
	}
	return nil
}

func (j *DeployJob) ToJobs(taskID int64) ([]*commonmodels.JobTask, error) {
	resp := []*commonmodels.JobTask{}
	j.spec = &commonmodels.ZadigDeployJobSpec{}

	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return resp, err
	}
	j.setDefaultDeployContent()
	j.job.Spec = j.spec

	envName := strings.ReplaceAll(j.spec.Env, setting.FixedValueMark, "")
	product, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{Name: j.workflow.Project, EnvName: envName})
	if err != nil {
		return resp, fmt.Errorf("env %s not exists", envName)
	}

	project, err := templaterepo.NewProductColl().Find(j.workflow.Project)
	if err != nil {
		return resp, fmt.Errorf("failed to find project %s, err: %v", j.workflow.Project, err)
	}

	productServiceMap := product.GetServiceMap()

	// get deploy info from previous build job
	if j.spec.Source == config.SourceFromJob {
		// adapt to the front end, use the direct quoted job name
		if j.spec.OriginJobName != "" {
			j.spec.JobName = j.spec.OriginJobName
		}
		j.spec.JobName = getOriginJobName(j.workflow, j.spec.JobName)
		targets, err := j.getOriginReferredJobTargets(j.spec.JobName)
		if err != nil {
			return resp, fmt.Errorf("get origin refered job: %s targets failed, err: %v", j.spec.JobName, err)
		}
		// clear service and image list to prevent old data from remaining
		targetsMap := make(map[string]struct {
			Image     string
			ImageName string
		})
		for _, target := range targets {
			key := fmt.Sprintf("%s++%s", target.ServiceName, target.ServiceModule)
			targetsMap[key] = struct {
				Image     string
				ImageName string
			}{
				Image:     target.Image,
				ImageName: target.ImageName,
			}
		}

		services := make([]*commonmodels.DeployServiceInfo, 0)
		for _, svc := range j.spec.Services {
			hasModule := false
			modules := make([]*commonmodels.DeployModuleInfo, 0)
			for _, module := range svc.Modules {
				key := fmt.Sprintf("%s++%s", svc.ServiceName, module.ServiceModule)
				if imageInfo, ok := targetsMap[key]; ok {
					hasModule = true
					modules = append(modules, &commonmodels.DeployModuleInfo{
						ServiceModule: module.ServiceModule,
						Image:         imageInfo.Image,
						ImageName:     imageInfo.ImageName,
					})
				}
			}

			svc.Modules = modules
			if hasModule {
				services = append(services, svc)
			}
		}

		j.spec.Services = services
	}

	serviceMap := map[string]*commonmodels.DeployServiceInfo{}
	for _, service := range j.spec.Services {
		serviceMap[service.ServiceName] = service
	}

	templateProduct, err := templaterepo.NewProductColl().Find(j.workflow.Project)
	if err != nil {
		return resp, fmt.Errorf("cannot find product %s: %w", j.workflow.Project, err)
	}
	timeout := templateProduct.Timeout * 60

	if j.spec.DeployType == setting.K8SDeployType {
		for _, svc := range j.spec.Services {
			serviceName := svc.ServiceName
			jobTaskSpec := &commonmodels.JobTaskDeploySpec{
				Env:                envName,
				SkipCheckRunStatus: j.spec.SkipCheckRunStatus,
				ServiceName:        serviceName,
				ServiceType:        setting.K8SDeployType,
				CreateEnvType:      project.ProductFeature.CreateEnvType,
				ClusterID:          product.ClusterID,
				Production:         j.spec.Production,
				DeployContents:     j.spec.DeployContents,
				Timeout:            timeout,
			}

			for _, module := range svc.Modules {
				// if external env, check service exists
				if project.IsHostProduct() {
					if err := checkServiceExsistsInEnv(productServiceMap, serviceName, envName); err != nil {
						return resp, err
					}
				}
				jobTaskSpec.ServiceAndImages = append(jobTaskSpec.ServiceAndImages, &commonmodels.DeployServiceModule{
					Image:         module.Image,
					ImageName:     module.ImageName,
					ServiceModule: module.ServiceModule,
				})
			}
			if !project.IsHostProduct() {
				jobTaskSpec.DeployContents = j.spec.DeployContents
				jobTaskSpec.Production = j.spec.Production
				service := serviceMap[serviceName]
				if service != nil {
					jobTaskSpec.UpdateConfig = service.UpdateConfig
					jobTaskSpec.VariableConfigs = service.VariableConfigs
					if service.UpdateConfig {
						jobTaskSpec.VariableKVs = service.LatestVariableKVs
					} else {
						jobTaskSpec.VariableKVs = service.VariableKVs
					}

					serviceRender := product.GetSvcRender(serviceName)
					svcRenderVarMap := map[string]*commontypes.RenderVariableKV{}
					for _, varKV := range serviceRender.OverrideYaml.RenderVariableKVs {
						svcRenderVarMap[varKV.Key] = varKV
					}

					// filter variables that used global variable
					filterdKV := []*commontypes.RenderVariableKV{}
					for _, jobKV := range jobTaskSpec.VariableKVs {
						svcKV, ok := svcRenderVarMap[jobKV.Key]
						if !ok {
							// deploy new variable
							filterdKV = append(filterdKV, jobKV)
							continue
						}
						// deploy existed variable
						if svcKV.UseGlobalVariable {
							continue
						}
						filterdKV = append(filterdKV, jobKV)
					}
					jobTaskSpec.VariableKVs = filterdKV
				}
				// if only deploy images, clear keyvals
				if onlyDeployImage(j.spec.DeployContents) {
					jobTaskSpec.VariableConfigs = []*commonmodels.DeployVariableConfig{}
					jobTaskSpec.VariableKVs = []*commontypes.RenderVariableKV{}
				}
			}
			jobTask := &commonmodels.JobTask{
				Name: jobNameFormat(serviceName + "-" + j.job.Name),
				Key:  strings.Join([]string{j.job.Name, serviceName}, "."),
				JobInfo: map[string]string{
					JobNameKey:     j.job.Name,
					"service_name": serviceName,
				},
				JobType: string(config.JobZadigDeploy),
				Spec:    jobTaskSpec,
			}
			if jobTaskSpec.CreateEnvType == "system" {
				var updateRevision bool
				if slices.Contains(jobTaskSpec.DeployContents, config.DeployConfig) && jobTaskSpec.UpdateConfig {
					updateRevision = true
				}

				varsYaml := ""
				varKVs := []*commontypes.RenderVariableKV{}
				if slices.Contains(jobTaskSpec.DeployContents, config.DeployVars) {
					varsYaml, err = commontypes.RenderVariableKVToYaml(jobTaskSpec.VariableKVs)
					if err != nil {
						return nil, errors.Errorf("generate vars yaml error: %v", err)
					}
					varKVs = jobTaskSpec.VariableKVs
				}
				containers := []*commonmodels.Container{}
				if slices.Contains(jobTaskSpec.DeployContents, config.DeployImage) {
					if j.spec.Source == config.SourceFromJob {
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
					return nil, errors.Errorf("generate service yaml error: %v", err)
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

	if j.spec.DeployType == setting.HelmDeployType {
		for _, svc := range j.spec.Services {
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
				Env:                envName,
				ServiceName:        svc.ServiceName,
				DeployContents:     j.spec.DeployContents,
				SkipCheckRunStatus: j.spec.SkipCheckRunStatus,
				ServiceType:        setting.HelmDeployType,
				ClusterID:          product.ClusterID,
				ReleaseName:        releaseName,
				Timeout:            timeout,
				IsProduction:       j.spec.Production,
			}

			for _, module := range svc.Modules {
				service := serviceMap[svc.ServiceName]
				if service != nil {
					jobTaskSpec.UpdateConfig = service.UpdateConfig
					jobTaskSpec.KeyVals = service.KeyVals
					jobTaskSpec.VariableYaml = service.VariableYaml
					jobTaskSpec.UserSuppliedValue = jobTaskSpec.VariableYaml
				}

				jobTaskSpec.ImageAndModules = append(jobTaskSpec.ImageAndModules, &commonmodels.ImageAndServiceModule{
					ServiceModule: module.ServiceModule,
					Image:         module.Image,
				})
			}
			jobTask := &commonmodels.JobTask{
				Name: jobNameFormat(svc.ServiceName + "-" + j.job.Name),
				Key:  strings.Join([]string{j.job.Name, svc.ServiceName}, "."),
				JobInfo: map[string]string{
					JobNameKey:     j.job.Name,
					"service_name": svc.ServiceName,
				},
				JobType: string(config.JobZadigHelmDeploy),
				Spec:    jobTaskSpec,
			}
			resp = append(resp, jobTask)
		}
	}

	j.job.Spec = j.spec
	return resp, nil
}

func onlyDeployImage(deployContents []config.DeployContent) bool {
	return slices.Contains(deployContents, config.DeployImage) && len(deployContents) == 1
}

func checkServiceExsistsInEnv(serviceMap map[string]*commonmodels.ProductService, serviceName, env string) error {
	if _, ok := serviceMap[serviceName]; !ok {
		return fmt.Errorf("service %s not exists in env %s", serviceName, env)
	}
	return nil
}

func (j *DeployJob) LintJob() error {
	j.spec = &commonmodels.ZadigDeployJobSpec{}
	if err := commonmodels.IToiYaml(j.job.Spec, j.spec); err != nil {
		return err
	}
	if err := aslanUtil.CheckZadigProfessionalLicense(); err != nil {
		if j.spec.Production {
			return e.ErrLicenseInvalid.AddDesc("生产环境功能需要专业版才能使用")
		}

		for _, item := range j.spec.DeployContents {
			if item == config.DeployVars || item == config.DeployConfig {
				return e.ErrLicenseInvalid.AddDesc("基础版仅能部署镜像")
			}
		}
	}
	if j.spec.Source != config.SourceFromJob {
		return nil
	}
	jobRankMap := getJobRankMap(j.workflow.Stages)
	buildJobRank, ok := jobRankMap[j.spec.JobName]
	if !ok || buildJobRank >= jobRankMap[j.job.Name] {
		return fmt.Errorf("can not quote job %s in job %s", j.spec.JobName, j.job.Name)
	}
	return nil
}

func (j *DeployJob) GetOutPuts(log *zap.SugaredLogger) []string {
	return getOutputKey(j.job.Name, ensureDeployInOutputs())
}

func ensureDeployInOutputs() []*commonmodels.Output {
	return []*commonmodels.Output{{Name: ENVNAMEKEY}}
}
