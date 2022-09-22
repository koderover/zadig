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

	j.job.Spec = j.spec
	return nil
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
		j.spec.ServiceAndImages = []*commonmodels.ServiceAndImage{}
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
			if err := checkServiceExsistsInEnv(productServiceMap, deploy.ServiceName, j.spec.Env); err != nil {
				return resp, err
			}
			jobTaskSpec := &commonmodels.JobTaskDeploySpec{
				Env:                j.spec.Env,
				SkipCheckRunStatus: j.spec.SkipCheckRunStatus,
				ServiceName:        deploy.ServiceName,
				ServiceType:        setting.K8SDeployType,
				ServiceModule:      deploy.ServiceModule,
				ClusterID:          product.ClusterID,
				Image:              deploy.Image,
			}
			jobTask := &commonmodels.JobTask{
				Name:    jobNameFormat(deploy.ServiceName + "-" + deploy.ServiceModule + "-" + j.job.Name),
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
