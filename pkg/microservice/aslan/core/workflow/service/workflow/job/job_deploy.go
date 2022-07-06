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

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/types/step"
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
	j.job.Spec = j.spec
	return nil
}

func (j *DeployJob) SetPreset() error {
	j.spec = &commonmodels.ZadigDeployJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return err
	}
	j.job.Spec = j.spec

	project, err := templaterepo.NewProductColl().Find(j.workflow.Project)
	if err != nil {
		return err
	}
	if project.ProductFeature != nil {
		j.spec.DeployType = project.ProductFeature.DeployType
	}

	services, err := commonrepo.NewServiceColl().ListMaxRevisionsByProduct(j.workflow.Project)
	if err != nil {
		return err
	}

	if j.spec.Source != config.SourceRuntime {
		return nil
	}

	for _, service := range services {
		for _, container := range service.Containers {
			j.spec.ServiceAndImages = append(j.spec.ServiceAndImages, &commonmodels.ServiceAndImage{ServiceName: service.ServiceName, ServiceModule: container.Name})
		}
	}
	j.job.Spec = j.spec
	return nil
}

func (j *DeployJob) ToJobs(taskID int64) ([]*commonmodels.JobTask, error) {
	resp := []*commonmodels.JobTask{}

	j.spec = &commonmodels.ZadigDeployJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return resp, err
	}
	j.job.Spec = j.spec

	product, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{Name: j.workflow.Project, EnvName: j.spec.Env})
	if err != nil {
		return resp, fmt.Errorf("env %s not exists", j.spec.Env)
	}

	project, err := templaterepo.NewProductColl().Find(j.workflow.Project)
	if err != nil {
		return resp, err
	}

	productServiceMap := product.GetServiceMap()

	if project.ProductFeature != nil && project.ProductFeature.CreateEnvType == setting.SourceFromExternal {
		productServices, err := commonrepo.NewServiceColl().ListExternalWorkloadsBy(j.workflow.Project, j.spec.Env)
		if err != nil {
			return resp, err
		}
		for _, service := range productServices {
			productServiceMap[service.ServiceName] = &commonmodels.ProductService{
				ServiceName: service.ServiceName,
				Containers:  service.Containers,
			}
		}
	}

	// get deploy info from previous build job
	if j.spec.Source == config.SourceFromJob {
		for _, stage := range j.workflow.Stages {
			for _, job := range stage.Jobs {
				if job.JobType != config.JobZadigBuild || job.Name != j.spec.JobName {
					continue
				}
				buildSpec := &commonmodels.ZadigBuildJobSpec{}
				if err := commonmodels.IToi(job.Spec, buildSpec); err != nil {
					return resp, err
				}
				for _, build := range buildSpec.ServiceAndBuilds {
					j.spec.ServiceAndImages = append(j.spec.ServiceAndImages, &commonmodels.ServiceAndImage{
						ServiceName:   build.ServiceName,
						ServiceModule: build.ServiceModule,
						Image:         build.Image,
					})
				}
			}
		}
	}
	if j.spec.DeployType == setting.K8SDeployType {
		for _, deploy := range j.spec.ServiceAndImages {
			if err := checkServiceAndContainerExsistsInEnv(productServiceMap, deploy.ServiceName, deploy.ServiceModule, j.spec.Env); err != nil {
				return resp, err
			}
			jobTask := &commonmodels.JobTask{
				Name:    jobNameFormat(deploy.ServiceName + "-" + deploy.ServiceModule + "-" + j.job.Name),
				JobType: string(config.JobZadigDeploy),
			}
			deployStep := &commonmodels.StepTask{
				Name:     deploy.ServiceName + "-deploy",
				JobName:  jobTask.Name,
				StepType: config.StepDeploy,
				Spec: step.StepDeploySpec{
					Env:           j.spec.Env,
					ServiceName:   deploy.ServiceName,
					ServiceType:   setting.K8SDeployType,
					ServiceModule: deploy.ServiceModule,
					ClusterID:     product.ClusterID,
					Image:         deploy.Image,
				},
			}
			jobTask.Steps = append(jobTask.Steps, deployStep)
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

			jobTask := &commonmodels.JobTask{
				Name:    jobNameFormat(serviceName + "-" + j.job.Name),
				JobType: string(config.JobZadigDeploy),
			}
			deployStep := &commonmodels.StepTask{
				Name:     serviceName + "-deploy",
				JobName:  jobTask.Name,
				StepType: config.StepHelmDeploy,
			}
			helmDeploySpec := step.StepHelmDeploySpec{
				Env:         j.spec.Env,
				ServiceName: serviceName,
				ServiceType: setting.HelmDeployType,
				ClusterID:   product.ClusterID,
				ReleaseName: releaseName,
			}
			for _, deploy := range deploys {
				if err := checkServiceAndContainerExsistsInEnv(productServiceMap, serviceName, deploy.ServiceModule, j.spec.Env); err != nil {
					return resp, err
				}
				helmDeploySpec.ImageAndModules = append(helmDeploySpec.ImageAndModules, &step.ImageAndServiceModule{
					ServiceModule: deploy.ServiceModule,
					Image:         deploy.Image,
				})
			}
			deployStep.Spec = helmDeploySpec

			jobTask.Steps = append(jobTask.Steps, deployStep)
			resp = append(resp, jobTask)
		}
	}

	j.job.Spec = j.spec
	return resp, nil
}

func checkServiceAndContainerExsistsInEnv(serviceMap map[string]*commonmodels.ProductService, serviceName, serviceModule, env string) error {
	if service, ok := serviceMap[serviceName]; !ok {
		return fmt.Errorf("service %s not exists in env %s", serviceName, env)
	} else {
		for _, container := range service.Containers {
			if container.Name == serviceModule {
				return nil
			}
		}
	}
	return fmt.Errorf("service %s module %s not exists in env %s", serviceName, serviceModule, env)
}
