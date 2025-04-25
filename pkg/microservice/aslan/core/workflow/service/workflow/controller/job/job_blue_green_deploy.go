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
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/helper/log"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/repository"
	"github.com/koderover/zadig/v2/pkg/setting"
	serializer2 "github.com/koderover/zadig/v2/pkg/tool/kube/serializer"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/util"
)

// TODO: services => service_options

type BlueGreenDeployJobController struct {
	*BasicInfo

	jobSpec *commonmodels.BlueGreenDeployV2JobSpec
}

func CreateBlueGreenDeployJobController(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) (Job, error) {
	spec := new(commonmodels.BlueGreenDeployV2JobSpec)
	if err := commonmodels.IToi(job.Spec, spec); err != nil {
		return nil, fmt.Errorf("failed to create apollo job controller, error: %s", err)
	}

	basicInfo := &BasicInfo{
		name:        job.Name,
		jobType:     job.JobType,
		errorPolicy: job.ErrorPolicy,
		workflow:    workflow,
	}

	return BlueGreenDeployJobController{
		BasicInfo: basicInfo,
		jobSpec:   spec,
	}, nil
}

func (j BlueGreenDeployJobController) SetWorkflow(wf *commonmodels.WorkflowV4) {
	j.workflow = wf
}

func (j BlueGreenDeployJobController) GetSpec() interface{} {
	return j.jobSpec
}

func (j BlueGreenDeployJobController) Validate(isExecution bool) error {
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

	j.jobSpec.Env = currJobSpec.Env
	j.jobSpec.Production = currJobSpec.Production
	j.jobSpec.ServiceOptions = currJobSpec.ServiceOptions

	// Determine service list and its corresponding kvs
	deployableService, _, err := generateBlueGreenEnvDeployServiceInfo(j.jobSpec.Env, j.jobSpec.Production, j.workflow.Project, currJobSpec.ServiceOptions)
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
				BlueServiceName:     service.BlueServiceName,
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

func (j BlueGreenDeployJobController) SetOptions(ticket *commonmodels.ApprovalTicket) error {
	envOptions := make([]*commonmodels.ZadigBlueGreenDeployEnvInformation, 0)

	if j.jobSpec.Source == "fixed" {
		if ticket.IsAllowedEnv(j.workflow.Project, j.jobSpec.Env) {
			serviceInfo, registryID, err := generateBlueGreenEnvDeployServiceInfo(j.jobSpec.Env, j.jobSpec.Production, j.workflow.Project, j.jobSpec.ServiceOptions)
			if err != nil {
				log.Errorf("failed to generate blue-green deploy info for env: %s, error: %s", j.jobSpec.Env, err)
				return err
			}

			envOptions = append(envOptions, &commonmodels.ZadigBlueGreenDeployEnvInformation{
				Env:        j.jobSpec.Env,
				RegistryID: registryID,
				Services:   serviceInfo,
			})
		}
	} else {
		// otherwise list all the envs in this project
		products, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
			Name:       j.workflow.Project,
			Production: util.GetBoolPointer(j.jobSpec.Production),
		})
		if err != nil {
			return fmt.Errorf("can't list envs in project %s, error: %w", j.workflow.Project, err)
		}
		for _, env := range products {
			if !ticket.IsAllowedEnv(j.workflow.Project, env.EnvName) {
				continue
			}

			serviceInfo, registryID, err := generateBlueGreenEnvDeployServiceInfo(env.EnvName, j.jobSpec.Production, j.workflow.Project, j.jobSpec.ServiceOptions)
			if err != nil {
				log.Errorf("failed to generate blue-green deploy info for env: %s, error: %s", env.EnvName, err)
				continue
			}

			envOptions = append(envOptions, &commonmodels.ZadigBlueGreenDeployEnvInformation{
				Env:        env.EnvName,
				RegistryID: registryID,
				Services:   serviceInfo,
			})
		}
	}

	j.jobSpec.EnvOptions = envOptions
	return nil
}

func (j BlueGreenDeployJobController) ClearOptions() {
	j.jobSpec.EnvOptions = nil
}

func (j BlueGreenDeployJobController) ClearSelection() {
	j.jobSpec.Services = make([]*commonmodels.BlueGreenDeployV2Service, 0)
}

func (j BlueGreenDeployJobController) ToTask(taskID int64) ([]*commonmodels.JobTask, error) {
	if err := j.Validate(true); err != nil {
		return nil, err
	}

	resp := make([]*commonmodels.JobTask, 0)

	templateProduct, err := templaterepo.NewProductColl().Find(j.workflow.Project)
	if err != nil {
		return resp, fmt.Errorf("cannot find product %s: %w", j.workflow.Project, err)
	}
	timeout := templateProduct.Timeout * 60

	if len(j.jobSpec.Services) == 0 {
		return nil, errors.Errorf("target services is empty")
	}

	for jobSubTaskID, target := range j.jobSpec.Services {
		var (
			deployment              *v1.Deployment
			deploymentYaml          string
			greenDeploymentSelector map[string]string
			service                 *corev1.Service
			greenService            *corev1.Service
			serviceYaml             string
			greenDeploymentName     string
		)
		if target.BlueServiceYaml != "" {
			service = &corev1.Service{}
			decoder := serializer.NewCodecFactory(scheme.Scheme).UniversalDecoder()
			err := runtime.DecodeInto(decoder, []byte(target.BlueServiceYaml), service)
			if err != nil {
				return resp, errors.Errorf("failed to decode %s k8s service yaml, err: %s", target.ServiceName, err)
			}
			serviceYaml = target.BlueServiceYaml
		} else {
			return resp, errors.Errorf("service %s blue service yaml is empty", target.ServiceName)
		}

		yamlContent, _, err := kube.FetchCurrentAppliedYaml(&kube.GeneSvcYamlOption{
			ProductName: j.workflow.Project,
			EnvName:     j.jobSpec.Env,
			ServiceName: target.ServiceName,
		})
		if err != nil {
			return resp, errors.Errorf("failed to fetch %s current applied yaml, err: %s", target.ServiceName, err)
		}
		resources := make([]*unstructured.Unstructured, 0)
		manifests := releaseutil.SplitManifests(yamlContent)
		for _, item := range manifests {
			u, err := serializer2.NewDecoder().YamlToUnstructured([]byte(item))
			if err != nil {
				return nil, errors.Errorf("failed to decode service %s yaml to unstructured: %v", target.ServiceName, err)
			}
			resources = append(resources, u)
		}

		for _, resource := range resources {
			switch resource.GetKind() {
			case setting.Deployment:
				if deployment != nil {
					return nil, errors.Errorf("service %s has more than one deployment", target.ServiceName)
				}
				deployment = &v1.Deployment{}
				err := runtime.DefaultUnstructuredConverter.FromUnstructured(resource.Object, deployment)
				if err != nil {
					return nil, errors.Errorf("failed to convert service %s deployment to deployment object: %v", target.ServiceName, err)
				}
				greenDeploymentName = deployment.Name
				if deployment.Spec.Selector == nil {
					return nil, errors.Errorf("service %s deployment selector is empty", target.ServiceName)
				}
				greenDeploymentSelector = deployment.Spec.Selector.MatchLabels
				deployment.Name = deployment.Name + "-blue"
				deployment.Labels = addLabels(deployment.Labels, map[string]string{
					types.ZadigReleaseTypeLabelKey:        types.ZadigReleaseTypeBlueGreen,
					types.ZadigReleaseServiceNameLabelKey: target.ServiceName,
				})
				deployment.Spec.Selector.MatchLabels = addLabels(deployment.Spec.Selector.MatchLabels, map[string]string{
					config.BlueGreenVersionLabelName: config.BlueVersion,
				})
				deployment.Spec.Template.Labels = addLabels(deployment.Spec.Template.Labels, map[string]string{
					config.BlueGreenVersionLabelName: config.BlueVersion,
				})
				deploymentYaml, err = toYaml(deployment)
				if err != nil {
					return resp, errors.Errorf("failed to marshal service %s deployment object: %v", target.ServiceName, err)
				}
				var newImages []*commonmodels.Container
				for _, image := range target.ServiceAndImage {
					newImages = append(newImages, &commonmodels.Container{
						Name:  image.ServiceModule,
						Image: image.Image,
					})
				}
				deploymentYaml, _, err = kube.ReplaceWorkloadImages(deploymentYaml, newImages)
				if err != nil {
					return resp, errors.Errorf("failed to replace service %s deployment image: %v", target.ServiceName, err)
				}
			case setting.Service:
				greenService = &corev1.Service{}
				err := runtime.DefaultUnstructuredConverter.FromUnstructured(resource.Object, greenService)
				if err != nil {
					return resp, errors.Errorf("failed to convert service %s service to service object: %v", target.ServiceName, err)
				}
				target.GreenServiceName = greenService.Name
			}
		}
		if deployment == nil || service == nil {
			return resp, errors.Errorf("service %s has no deployment or service", target.ServiceName)
		}
		if service.Spec.Selector == nil || deployment.Spec.Template.Labels == nil {
			return resp, errors.Errorf("service %s has no service selector or deployment has no labels", target.ServiceName)
		}
		selector, err := metav1.LabelSelectorAsSelector(metav1.SetAsLabelSelector(service.Spec.Selector))
		if err != nil {
			return resp, errors.Errorf("service %s k8s service convert to selector err: %v", target.ServiceName, err)
		}
		if !selector.Matches(labels.Set(deployment.Spec.Template.Labels)) {
			return resp, errors.Errorf("service %s k8s service selector not match deployment.spec.template labels", target.ServiceName)
		}
		if greenService == nil {
			return resp, errors.Errorf("service %s has no k8s service", target.ServiceName)
		}
		greenSelector, err := metav1.LabelSelectorAsSelector(metav1.SetAsLabelSelector(greenService.Spec.Selector))
		if err != nil {
			return resp, errors.Errorf("service %s k8s green service convert to selector err: %v", target.ServiceName, err)
		}
		if !greenSelector.Matches(labels.Set(greenDeploymentSelector)) {
			return resp, errors.Errorf("service %s k8s green service selector not match deployment.spec.template labels", target.ServiceName)
		}

		// set target value for blue_green_release ToJobs get these
		target.BlueDeploymentName = deployment.Name
		target.BlueServiceName = service.Name
		target.GreenDeploymentName = greenDeploymentName

		task := &commonmodels.JobTask{
			Name:        GenJobName(j.workflow, j.name, jobSubTaskID),
			Key:         genJobKey(j.name),
			DisplayName: genJobDisplayName(j.name),
			OriginName:  j.name,
			JobInfo: map[string]string{
				JobNameKey:     j.name,
				"service_name": target.ServiceName,
			},
			JobType: string(config.JobK8sBlueGreenDeploy),
			Spec: &commonmodels.JobTaskBlueGreenDeployV2Spec{
				Production: j.jobSpec.Production,
				Env:        j.jobSpec.Env,
				Service: &commonmodels.BlueGreenDeployV2Service{
					ServiceName:         target.ServiceName,
					BlueServiceYaml:     serviceYaml,
					BlueServiceName:     service.Name,
					BlueDeploymentYaml:  deploymentYaml,
					BlueDeploymentName:  deployment.Name,
					GreenServiceName:    target.GreenServiceName,
					GreenDeploymentName: greenDeploymentName,
					ServiceAndImage:     target.ServiceAndImage,
				},
				DeployTimeout: timeout,
			},
			ErrorPolicy: j.errorPolicy,
		}
		resp = append(resp, task)
	}

	return resp, nil
}

func (j BlueGreenDeployJobController) SetRepo(repo *types.Repository) error {
	return nil
}

func (j BlueGreenDeployJobController) SetRepoCommitInfo() error {
	return nil
}

func (j BlueGreenDeployJobController) GetVariableList(jobName string, getAggregatedVariables, getRuntimeVariables, getPlaceHolderVariables, getServiceSpecificVariables, getReferredKeyValVariables bool) ([]*commonmodels.KeyVal, error) {
	return make([]*commonmodels.KeyVal, 0), nil
}

func (j BlueGreenDeployJobController) GetUsedRepos() ([]*types.Repository, error) {
	return make([]*types.Repository, 0), nil
}

func (j BlueGreenDeployJobController) RenderDynamicVariableOptions(key string, option *RenderDynamicVariableValue) ([]string, error) {
	return nil, fmt.Errorf("invalid job type: %s to render dynamic variable", j.name)
}

func (j BlueGreenDeployJobController) IsServiceTypeJob() bool {
	return false
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
		if configuredService, ok := configuredServiceMap[envService.ServiceName]; ok {
			if len(configuredService.BlueServiceYaml) != 0 {
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
		} else {
			continue
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

func addLabels(labels, newLabels map[string]string) map[string]string {
	if labels == nil {
		labels = make(map[string]string)
	}
	for k, v := range newLabels {
		labels[k] = v
	}
	return labels
}
