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

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/util"
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

func (j *DeployJob) getOriginReferedJobTargets(jobName string) ([]*commonmodels.ServiceAndImage, error) {
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
				}
				return serviceAndImages, nil
			}
			if job.JobType == config.JobZadigDistributeImage {
				distributeSpec := &commonmodels.ZadigDistributeImageJobSpec{}
				if err := commonmodels.IToi(job.Spec, distributeSpec); err != nil {
					return serviceAndImages, err
				}
				for _, distribute := range distributeSpec.Tatgets {
					serviceAndImages = append(serviceAndImages, &commonmodels.ServiceAndImage{
						ServiceName:   distribute.ServiceName,
						ServiceModule: distribute.ServiceModule,
						Image:         distribute.TargetImage,
					})
				}
				return serviceAndImages, nil
			}
		}
	}
	return nil, fmt.Errorf("build job %s not found", jobName)
}

func (j *DeployJob) SetPreset() error {
	j.spec = &commonmodels.ZadigDeployJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return err
	}
	j.setDefaultDeployContent()
	j.job.Spec = j.spec
	var err error
	project, err := templaterepo.NewProductColl().Find(j.workflow.Project)
	if err != nil {
		return fmt.Errorf("failed to find project %s, err: %v", j.workflow.Project, err)
	}
	if project.ProductFeature != nil {
		j.spec.DeployType = project.ProductFeature.DeployType
		if project.ProductFeature.DeployType == "helm" {
			return nil
		}
		if project.ProductFeature.CreateEnvType == "external" {
			return nil
		}
	}
	targets := j.spec.ServiceAndImages
	if j.spec.Source == config.SourceFromJob {
		// clear service and image list to prevent old data from remaining
		targets, err = j.getOriginReferedJobTargets(j.spec.JobName)
		if err != nil {
			return fmt.Errorf("get origin refered job: %s targets failed, err: %v", j.spec.JobName, err)
		}
		j.spec.Services = []*commonmodels.DeployService{}
	}
	serviceNameMap := map[string]bool{}
	for _, target := range targets {
		serviceNameMap[target.ServiceName] = true
	}
	deployServiceMap := map[string]*commonmodels.DeployService{}
	for _, service := range j.spec.Services {
		deployServiceMap[service.ServiceName] = service
	}
	newServices := []*commonmodels.DeployService{}
	for serviceName := range serviceNameMap {
		deployService, err := j.filterServiceVars(serviceName, deployServiceMap[serviceName])
		if err != nil {
			return err
		}
		newServices = append(newServices, deployService)
	}
	j.spec.Services = newServices
	return nil
}

// filterServiceVars filter service variables user defined.
func (j *DeployJob) filterServiceVars(serviceName string, service *commonmodels.DeployService) (*commonmodels.DeployService, error) {
	svcVars, err := kube.FetchCurrentServiceVariable(&kube.GeneSvcYamlOption{ProductName: j.workflow.Project, EnvName: j.spec.Env, ServiceName: serviceName})
	if err != nil {
		return service, fmt.Errorf("failed to find project %s, err: %v", j.workflow.Project, err)
	}
	if service == nil {
		service = &commonmodels.DeployService{
			ServiceName: serviceName,
		}
		for _, svcVar := range svcVars {
			service.KeyVals = append(service.KeyVals, &commonmodels.ServiceKeyVal{
				Key:   svcVar.Key,
				Value: svcVar.Value,
				Type:  commonmodels.StringType,
			})
		}
		return service, nil
	}
	service.ServiceName = serviceName
	newVars := []*commonmodels.ServiceKeyVal{}
	for _, svcVar := range service.KeyVals {
		for _, varItem := range svcVars {
			if svcVar.Key == varItem.Key {
				svcVar.Value = varItem.Value
				newVars = append(newVars, svcVar)
				break
			}
		}
	}
	service.KeyVals = newVars
	return service, nil
}

func (j *DeployJob) mergeServiceVars(service *commonmodels.DeployService) ([]*commonmodels.ServiceKeyVal, error) {
	svcVars, err := kube.FetchCurrentServiceVariable(&kube.GeneSvcYamlOption{ProductName: j.workflow.Project, EnvName: j.spec.Env, ServiceName: service.ServiceName})
	if err != nil {
		return nil, fmt.Errorf("failed to find project %s, err: %v", j.workflow.Project, err)
	}
	if service == nil {
		service = &commonmodels.DeployService{
			ServiceName: service.ServiceName,
		}
		for _, svcVar := range svcVars {
			service.KeyVals = append(service.KeyVals, &commonmodels.ServiceKeyVal{
				Key:   svcVar.Key,
				Value: svcVar.Value,
				Type:  commonmodels.StringType,
			})
		}
		return service.KeyVals, nil
	}
	newVars := []*commonmodels.ServiceKeyVal{}
	for _, varItem := range svcVars {
		newVar := &commonmodels.ServiceKeyVal{Key: varItem.Key, Value: varItem.Value}
		for _, svcVar := range service.KeyVals {
			if newVar.Key == svcVar.Key {
				newVar.Value = svcVar.Value
				break
			}
		}
		newVars = append(newVars, newVar)
	}
	return newVars, nil
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
			j.spec.ServiceAndImages = argsSpec.ServiceAndImages
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

	product, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{Name: j.workflow.Project, EnvName: j.spec.Env})
	if err != nil {
		return resp, fmt.Errorf("env %s not exists", j.spec.Env)
	}

	project, err := templaterepo.NewProductColl().Find(j.workflow.Project)
	if err != nil {
		return resp, fmt.Errorf("failed to find project %s, err: %v", j.workflow.Project, err)
	}

	productServiceMap := product.GetServiceMap()

	if project.ProductFeature != nil && project.ProductFeature.CreateEnvType == setting.SourceFromExternal {
		productServices, err := commonrepo.NewServiceColl().ListExternalWorkloadsBy(j.workflow.Project, j.spec.Env)
		if err != nil {
			return resp, fmt.Errorf("failed to list external workload, err: %v", err)
		}
		for _, service := range productServices {
			productServiceMap[service.ServiceName] = &commonmodels.ProductService{
				ServiceName: service.ServiceName,
				Containers:  service.Containers,
			}
		}
		servicesInExternalEnv, _ := commonrepo.NewServicesInExternalEnvColl().List(&commonrepo.ServicesInExternalEnvArgs{
			ProductName: j.workflow.Project,
			EnvName:     j.spec.Env,
		})
		for _, service := range servicesInExternalEnv {
			productServiceMap[service.ServiceName] = &commonmodels.ProductService{
				ServiceName: service.ServiceName,
			}
		}
	}

	// get deploy info from previous build job
	if j.spec.Source == config.SourceFromJob {
		// clear service and image list to prevent old data from remaining
		targets, err := j.getOriginReferedJobTargets(j.spec.JobName)
		if err != nil {
			return resp, fmt.Errorf("get origin refered job: %s targets failed, err: %v", j.spec.JobName, err)
		}
		j.spec.ServiceAndImages = targets
	}

	serviceMap := map[string]*commonmodels.DeployService{}
	for _, service := range j.spec.Services {
		serviceMap[service.ServiceName] = service
	}

	if j.spec.DeployType == setting.K8SDeployType {
		deployServiceMap := map[string][]*commonmodels.ServiceAndImage{}
		for _, deploy := range j.spec.ServiceAndImages {
			deployServiceMap[deploy.ServiceName] = append(deployServiceMap[deploy.ServiceName], deploy)
		}
		for serviceName, deploys := range deployServiceMap {
			jobTaskSpec := &commonmodels.JobTaskDeploySpec{
				Env:                j.spec.Env,
				SkipCheckRunStatus: j.spec.SkipCheckRunStatus,
				ServiceName:        serviceName,
				ServiceType:        setting.K8SDeployType,
				CreateEnvType:      project.ProductFeature.CreateEnvType,
				ClusterID:          product.ClusterID,
				Production:         j.spec.Production,
				DeployContents:     j.spec.DeployContents,
			}
			for _, deploy := range deploys {
				if err := checkServiceExsistsInEnv(productServiceMap, serviceName, j.spec.Env); err != nil {
					return resp, err
				}
				jobTaskSpec.ServiceAndImages = append(jobTaskSpec.ServiceAndImages, &commonmodels.DeployServiceModule{
					Image:         deploy.Image,
					ServiceModule: deploy.ServiceModule,
				})
			}
			if project.ProductFeature.CreateEnvType != "external" {
				jobTaskSpec.DeployContents = j.spec.DeployContents
				jobTaskSpec.Production = j.spec.Production
				service := serviceMap[serviceName]
				if service != nil {
					jobTaskSpec.UpdateConfig = service.UpdateConfig
					jobTaskSpec.KeyVals, err = j.mergeServiceVars(service)
					if err != nil {
						return resp, err
					}
				}
			}
			jobTask := &commonmodels.JobTask{
				Name:    jobNameFormat(serviceName + "-" + j.job.Name),
				Key:     strings.Join([]string{j.job.Name, serviceName}, "."),
				JobType: string(config.JobZadigDeploy),
				Spec:    jobTaskSpec,
			}
			resp = append(resp, jobTask)
		}
	}
	if j.spec.DeployType == setting.HelmDeployType {
		deployServiceMap := map[string][]*commonmodels.ServiceAndImage{}
		for _, deploy := range j.spec.ServiceAndImages {
			deployServiceMap[deploy.ServiceName] = append(deployServiceMap[deploy.ServiceName], deploy)
		}
		for serviceName, deploys := range deployServiceMap {
			var serviceRevision int64
			if pSvc, ok := productServiceMap[serviceName]; ok {
				serviceRevision = pSvc.Revision
			}

			revisionSvc, err := commonrepo.NewServiceColl().Find(&commonrepo.ServiceFindOption{
				ServiceName: serviceName,
				Revision:    serviceRevision,
				ProductName: product.ProductName,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to find service: %s with revision: %d, err: %s", serviceName, serviceRevision, err)
			}
			releaseName := util.GeneReleaseName(revisionSvc.GetReleaseNaming(), product.ProductName, product.Namespace, product.EnvName, serviceName)

			jobTaskSpec := &commonmodels.JobTaskHelmDeploySpec{
				Env:                j.spec.Env,
				ServiceName:        serviceName,
				SkipCheckRunStatus: j.spec.SkipCheckRunStatus,
				ServiceType:        setting.HelmDeployType,
				ClusterID:          product.ClusterID,
				ReleaseName:        releaseName,
			}
			for _, deploy := range deploys {
				if err := checkServiceExsistsInEnv(productServiceMap, serviceName, j.spec.Env); err != nil {
					return resp, err
				}
				jobTaskSpec.ImageAndModules = append(jobTaskSpec.ImageAndModules, &commonmodels.ImageAndServiceModule{
					ServiceModule: deploy.ServiceModule,
					Image:         deploy.Image,
				})
			}
			jobTask := &commonmodels.JobTask{
				Name:    jobNameFormat(serviceName + "-" + j.job.Name),
				Key:     strings.Join([]string{j.job.Name, serviceName}, "."),
				JobType: string(config.JobZadigHelmDeploy),
				Spec:    jobTaskSpec,
			}
			resp = append(resp, jobTask)
		}
	}

	j.job.Spec = j.spec
	return resp, nil
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
