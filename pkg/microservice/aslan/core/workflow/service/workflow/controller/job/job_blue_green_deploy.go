/*
 * Copyright 2025 The KodeRover Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package job

import (
	"bytes"
	"fmt"

	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/releaseutil"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/printers"

	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/helper/log"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/repository"
	aslanUtil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/setting"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	serializer2 "github.com/koderover/zadig/v2/pkg/tool/kube/serializer"
	"github.com/koderover/zadig/v2/pkg/util"
)

type BlueGreenDeployJobController struct {
	*BasicInfo

	jobSpec *commonmodels.BlueGreenDeployV2JobSpec
}

func CreateBlueGreenDeployJobController(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) (Job, error) {
	//spec := new(commonmodels.BlueGreenDeployV2JobSpec)
	//if err := commonmodels.IToi(job.Spec, spec); err != nil {
	//	return nil, fmt.Errorf("failed to create apollo job controller, error: %s", err)
	//}
	//
	//basicInfo := &BasicInfo{
	//	name:        job.Name,
	//	jobType:     job.JobType,
	//	errorPolicy: job.ErrorPolicy,
	//	workflow:    workflow,
	//}

	return nil, nil

	//return BlueGreenDeployJobController{
	//	BasicInfo: basicInfo,
	//	jobSpec:   spec,
	//}, nil
}

func (j BlueGreenDeployJobController) SetWorkflow(wf *commonmodels.WorkflowV4) {
	j.workflow = wf
}

func (j BlueGreenDeployJobController) GetSpec() interface{} {
	return j.jobSpec
}

func (j BlueGreenDeployJobController) Validate(isExecution bool) error {
	if err := aslanUtil.CheckZadigProfessionalLicense(); err != nil {
		return e.ErrLicenseInvalid.AddDesc("")
	}

	currJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}

	currJobSpec := new(commonmodels.ApolloJobSpec)
	if err := commonmodels.IToi(currJob.Spec, currJobSpec); err != nil {
		return fmt.Errorf("failed to decode apollo job spec, error: %s", err)
	}

	if j.jobSpec.Version == "" {
		return fmt.Errorf("job %s version is too old and not supported, please remove it and create a new one", j.name)
	}

	quoteJobs := make([]*commonmodels.Job, 0)
	for _, stage := range j.workflow.Stages {
		for _, job := range stage.Jobs {
			if job.JobType != config.JobK8sBlueGreenRelease {
				continue
			}
			releaseJobSpec := &commonmodels.BlueGreenReleaseV2JobSpec{}
			if err := commonmodels.IToiYaml(job.Spec, releaseJobSpec); err != nil {
				return err
			}
			if releaseJobSpec.FromJob == j.name {
				quoteJobs = append(quoteJobs, job)
			}
		}
	}
	if len(quoteJobs) == 0 {
		return fmt.Errorf("no blue-green release job quote blue-green deploy job %s", j.name)
	}
	if len(quoteJobs) > 1 {
		return fmt.Errorf("more than one blue-green release job quote blue-green deploy job %s", j.name)
	}
	jobRankMap := GetJobRankMap(j.workflow.Stages)
	if jobRankMap[j.name] >= jobRankMap[quoteJobs[0].Name] {
		return fmt.Errorf("blue-green release job %s should run before blue-green deploy job %s", quoteJobs[0].Name, j.name)
	}
	return nil
}

func (j BlueGreenDeployJobController) Update(useUserInput bool, ticket *commonmodels.ApprovalTicket) error {
	currJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}

	currJobSpec := new(commonmodels.BlueGreenDeployV2JobSpec)
	if err := commonmodels.IToi(currJob.Spec, currJobSpec); err != nil {
		return fmt.Errorf("failed to decode apollo job spec, error: %s", err)
	}

	if useUserInput {
		if j.jobSpec.Env != currJobSpec.Env {
			return fmt.Errorf("given env: %s does not match job definition: %s", j.jobSpec.Env, currJobSpec.Env)
		}
	}
	j.jobSpec.Env = currJobSpec.Env
	j.jobSpec.Production = currJobSpec.Production

	// Determine service list and its corresponding kvs
	deployableService, _, err := generateBlueGreenEnvDeployServiceInfo(j.jobSpec.Env, j.jobSpec.Production, j.workflow.Project, currJobSpec.Services)
	if err != nil {
		log.Errorf("failed to generate deployable service from latest workflow spec, err: %s", err)
		return err
	}

	mergedService := make([]*commonmodels.BlueGreenDeployV2Service, 0)
	userConfiguredService := make(map[string]*commonmodels.BlueGreenDeployV2Service)

	for _, service := range j.jobSpec.Services {
		userConfiguredService[service.ServiceName] = service
	}

	for _, service := range deployableService {
		if userSvc, ok := userConfiguredService[service.ServiceName]; ok {
			mergedService = append(mergedService, &commonmodels.BlueGreenDeployV2Service{
				ServiceName:         service.ServiceName,
				BlueServiceYaml:     userSvc.BlueServiceYaml,
				BlueServiceName:     service.BlueServiceYaml,
				BlueDeploymentYaml:  service.BlueDeploymentYaml,
				BlueDeploymentName:  service.BlueDeploymentName,
				GreenDeploymentName: service.GreenDeploymentName,
				GreenServiceName:    service.GreenServiceName,
				ServiceAndImage:     userSvc.ServiceAndImage,
			})
		} else {
			continue
		}
	}

	j.jobSpec.Services = mergedService
	return nil
}

func generateBlueGreenEnvDeployServiceInfo(env string, production bool, project string, services []*commonmodels.BlueGreenDeployV2Service) ([]*commonmodels.BlueGreenDeployV2Service, string, error) {
	targetEnv, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		EnvName:    env,
		Name:       project,
		Production: util.GetBoolPointer(production),
	})

	configuredServiceMap := make(map[string]*commonmodels.BlueGreenDeployV2Service)
	for _, svc := range services {
		configuredServiceMap[svc.ServiceName] = svc
	}

	if err != nil {
		return nil, "", fmt.Errorf("failed to find product %s, env %s, err: %s", project, env, err)
	}

	latestSvcList, err := repository.ListMaxRevisionsServices(project, production)
	if err != nil {
		return nil, "", fmt.Errorf("failed to list services with max revisions in project: %s, error: %s")
	}

	serviceInfo, err := commonservice.BuildServiceInfoInEnv(targetEnv, latestSvcList, nil, log.GetSimpleLogger())
	if err != nil {
		return nil, "", fmt.Errorf("failed to build service info in env: %s, error: %s", env, err)
	}

	resp := make([]*commonmodels.BlueGreenDeployV2Service, 0)

	for _, envService := range serviceInfo.Services {
		if !envService.Deployed {
			continue
		}

		appendService := &commonmodels.BlueGreenDeployV2Service{
			ServiceName: envService.ServiceName,
		}
		modules := make([]*commonmodels.BlueGreenDeployV2ServiceModuleAndImage, 0)

		for _, module := range envService.ServiceModules {
			modules = append(modules, &commonmodels.BlueGreenDeployV2ServiceModuleAndImage{
				ServiceModule: module.Name,
				Image:         module.Image,
				ImageName:     util.ExtractImageName(module.Image),
				Name:          module.Name,
				ServiceName:   envService.ServiceName,
			})
		}
		appendService.ServiceAndImage = modules

		// if a yaml is pre-configured, use it. Otherwise, just get it from the environment and do some render.
		if configuredService, ok := configuredServiceMap[envService.ServiceName]; ok && len(configuredService.BlueServiceYaml) != 0 {
			appendService.BlueServiceYaml = configuredService.BlueServiceYaml
		} else {
			yamlContent, _, err := kube.FetchCurrentAppliedYaml(&kube.GeneSvcYamlOption{
				ProductName: project,
				EnvName:     env,
				ServiceName: envService.ServiceName,
			})
			if err != nil {
				return nil, "", fmt.Errorf("failed to fetch %s current applied yaml, err: %s", envService.ServiceName, err)
			}

			resources := make([]*unstructured.Unstructured, 0)
			manifests := releaseutil.SplitManifests(yamlContent)
			for _, item := range manifests {
				u, err := serializer2.NewDecoder().YamlToUnstructured([]byte(item))
				if err != nil {
					return nil, "", fmt.Errorf("failed to decode service %s yaml to unstructured: %v", envService.ServiceName, err)
				}
				resources = append(resources, u)
			}

			serviceNum := 0
			for _, resource := range resources {
				switch resource.GetKind() {
				case setting.Service:
					if serviceNum > 0 {
						return nil, "", fmt.Errorf("service %s has more than one service", envService.ServiceName)
					}
					serviceNum++
					service := &corev1.Service{}
					err := runtime.DefaultUnstructuredConverter.FromUnstructured(resource.Object, service)
					if err != nil {
						return nil, "", fmt.Errorf("failed to convert service %s service to service object: %v", envService.ServiceName, err)
					}
					service.Name = service.Name + "-blue"
					if service.Spec.Selector == nil {
						service.Spec.Selector = make(map[string]string)
					}
					service.Spec.Selector[config.BlueGreenVersionLabelName] = config.BlueVersion
					appendService.BlueServiceYaml, err = toYaml(service)
					if err != nil {
						return nil, "", fmt.Errorf("failed to marshal service %s service object: %v", envService.ServiceName, err)
					}
				}
			}
			if serviceNum == 0 {
				return nil, "", fmt.Errorf("service %s has no service", envService.ServiceName)
			}
		}

		resp = append(resp, appendService)
	}

	registryID := targetEnv.RegistryID
	if registryID == "" {
		registry, err := commonrepo.NewRegistryNamespaceColl().Find(&commonrepo.FindRegOps{
			IsDefault: true,
		})

		if err != nil {
			return nil, "", fmt.Errorf("failed to find default registry for env: %s, error: %s", env, err)
		}
		registryID = registry.ID.Hex()
	}
	return resp, registryID, nil
}

func toYaml(obj runtime.Object) (string, error) {
	y := printers.YAMLPrinter{}
	writer := bytes.NewBuffer(nil)
	err := y.PrintObj(obj, writer)
	if err != nil {
		return "", errors.Wrapf(err, "failed to marshal object to yaml")
	}
	return writer.String(), nil
}
