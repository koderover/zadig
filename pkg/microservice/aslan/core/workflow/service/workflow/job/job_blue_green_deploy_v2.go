/*
 * Copyright 2023 The KodeRover Authors.
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
	"strings"

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

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/kube"
	aslanUtil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/setting"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	serializer2 "github.com/koderover/zadig/v2/pkg/tool/kube/serializer"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/util"
)

type BlueGreenDeployV2Job struct {
	job      *commonmodels.Job
	workflow *commonmodels.WorkflowV4
	spec     *commonmodels.BlueGreenDeployV2JobSpec
}

func (j *BlueGreenDeployV2Job) Instantiate() error {
	j.spec = &commonmodels.BlueGreenDeployV2JobSpec{}
	if err := commonmodels.IToiYaml(j.job.Spec, j.spec); err != nil {
		return err
	}
	j.job.Spec = j.spec
	return nil
}

func (j *BlueGreenDeployV2Job) SetPreset() error {
	j.spec = &commonmodels.BlueGreenDeployV2JobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return err
	}

	product, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{EnvName: j.spec.Env, Name: j.workflow.Project, Production: util.GetBoolPointer(j.spec.Production)})
	if err != nil {
		return errors.Errorf("failed to find product %s, env %s, err: %s", j.workflow.Project, j.spec.Env, err)
	}
	for _, target := range j.spec.Services {
	L:
		for _, services := range product.Services {
			for _, productService := range services {
				if productService.ServiceName == target.ServiceName {
					for _, container := range productService.Containers {
						target.ServiceAndImage = append(target.ServiceAndImage, &commonmodels.BlueGreenDeployV2ServiceModuleAndImage{
							ServiceModule: container.Name,
							Image:         container.Image,
							ImageName:     container.ImageName,
							Name:          container.Name,
							ServiceName:   target.ServiceName,
						})
					}
					break L
				}
			}
		}

		if target.BlueServiceYaml == "" {
			yamlContent, _, err := kube.FetchCurrentAppliedYaml(&kube.GeneSvcYamlOption{
				ProductName: j.workflow.Project,
				EnvName:     j.spec.Env,
				ServiceName: target.ServiceName,
			})
			if err != nil {
				return errors.Errorf("failed to fetch %s current applied yaml, err: %s", target.ServiceName, err)
			}
			resources := make([]*unstructured.Unstructured, 0)
			manifests := releaseutil.SplitManifests(yamlContent)
			for _, item := range manifests {
				u, err := serializer2.NewDecoder().YamlToUnstructured([]byte(item))
				if err != nil {
					return errors.Errorf("failed to decode service %s yaml to unstructured: %v", target.ServiceName, err)
				}
				resources = append(resources, u)
			}

			serviceNum := 0
			for _, resource := range resources {
				switch resource.GetKind() {
				case setting.Service:
					if serviceNum > 0 {
						return errors.Errorf("service %s has more than one service", target.ServiceName)
					}
					serviceNum++
					service := &corev1.Service{}
					err := runtime.DefaultUnstructuredConverter.FromUnstructured(resource.Object, service)
					if err != nil {
						return errors.Errorf("failed to convert service %s service to service object: %v", target.ServiceName, err)
					}
					service.Name = service.Name + "-blue"
					if service.Spec.Selector == nil {
						service.Spec.Selector = make(map[string]string)
					}
					service.Spec.Selector[config.BlueGreenVerionLabelName] = config.BlueVersion
					target.BlueServiceYaml, err = toYaml(service)
					if err != nil {
						return errors.Errorf("failed to marshal service %s service object: %v", target.ServiceName, err)
					}
				}
			}
			if serviceNum == 0 {
				return errors.Errorf("service %s has no service", target.ServiceName)
			}
		}
	}
	j.job.Spec = j.spec
	return nil
}

func (j *BlueGreenDeployV2Job) MergeArgs(args *commonmodels.Job) error {
	if j.job.Name == args.Name && j.job.JobType == args.JobType {
		j.spec = &commonmodels.BlueGreenDeployV2JobSpec{}
		if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
			return err
		}
		j.job.Spec = j.spec
		argsSpec := &commonmodels.BlueGreenDeployV2JobSpec{}
		if err := commonmodels.IToi(args.Spec, argsSpec); err != nil {
			return err
		}
		j.spec.Services = argsSpec.Services
		j.job.Spec = j.spec
	}
	return nil
}

func (j *BlueGreenDeployV2Job) ToJobs(taskID int64) ([]*commonmodels.JobTask, error) {
	resp := []*commonmodels.JobTask{}
	j.spec = &commonmodels.BlueGreenDeployV2JobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return resp, err
	}

	templateProduct, err := templaterepo.NewProductColl().Find(j.workflow.Project)
	if err != nil {
		return resp, fmt.Errorf("cannot find product %s: %w", j.workflow.Project, err)
	}
	timeout := templateProduct.Timeout * 60

	if len(j.spec.Services) == 0 {
		return resp, errors.Errorf("target services is empty")
	}

	for _, target := range j.spec.Services {
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
			EnvName:     j.spec.Env,
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
				return resp, errors.Errorf("failed to decode service %s yaml to unstructured: %v", target.ServiceName, err)
			}
			resources = append(resources, u)
		}

		for _, resource := range resources {
			switch resource.GetKind() {
			case setting.Deployment:
				if deployment != nil {
					return resp, errors.Errorf("service %s has more than one deployment", target.ServiceName)
				}
				deployment = &v1.Deployment{}
				err := runtime.DefaultUnstructuredConverter.FromUnstructured(resource.Object, deployment)
				if err != nil {
					return resp, errors.Errorf("failed to convert service %s deployment to deployment object: %v", target.ServiceName, err)
				}
				greenDeploymentName = deployment.Name
				if deployment.Spec.Selector == nil {
					return resp, errors.Errorf("service %s deployment selector is empty", target.ServiceName)
				}
				greenDeploymentSelector = deployment.Spec.Selector.MatchLabels
				deployment.Name = deployment.Name + "-blue"
				deployment.Labels = addLabels(deployment.Labels, map[string]string{
					types.ZadigReleaseTypeLabelKey:        types.ZadigReleaseTypeBlueGreen,
					types.ZadigReleaseServiceNameLabelKey: target.ServiceName,
				})
				deployment.Spec.Selector.MatchLabels = addLabels(deployment.Spec.Selector.MatchLabels, map[string]string{
					config.BlueGreenVerionLabelName: config.BlueVersion,
				})
				deployment.Spec.Template.Labels = addLabels(deployment.Spec.Template.Labels, map[string]string{
					config.BlueGreenVerionLabelName: config.BlueVersion,
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
			Name: jobNameFormat(j.job.Name + "-" + target.ServiceName),
			Key:  strings.Join([]string{j.job.Name, target.ServiceName}, "."),
			JobInfo: map[string]string{
				JobNameKey:     j.job.Name,
				"service_name": target.ServiceName,
			},
			JobType: string(config.JobK8sBlueGreenDeploy),
			Spec: &commonmodels.JobTaskBlueGreenDeployV2Spec{
				Production: j.spec.Production,
				Env:        j.spec.Env,
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
		}
		resp = append(resp, task)
	}

	j.job.Spec = j.spec
	return resp, nil
}

func (j *BlueGreenDeployV2Job) LintJob() error {
	j.spec = &commonmodels.BlueGreenDeployV2JobSpec{}
	if err := aslanUtil.CheckZadigXLicenseStatus(); err != nil {
		return e.ErrLicenseInvalid.AddDesc("")
	}
	if err := commonmodels.IToiYaml(j.job.Spec, j.spec); err != nil {
		return err
	}
	if j.spec.Version == "" {
		return errors.Errorf("job %s version is too old and not supported, please remove it and create a new one", j.job.Name)
	}
	quoteJobs := []*commonmodels.Job{}
	for _, stage := range j.workflow.Stages {
		for _, job := range stage.Jobs {
			if job.JobType != config.JobK8sBlueGreenRelease {
				continue
			}
			releaseJobSpec := &commonmodels.BlueGreenReleaseV2JobSpec{}
			if err := commonmodels.IToiYaml(job.Spec, releaseJobSpec); err != nil {
				return err
			}
			if releaseJobSpec.FromJob == j.job.Name {
				quoteJobs = append(quoteJobs, job)
			}
		}
	}
	if len(quoteJobs) == 0 {
		return errors.Errorf("no blue-green release job quote blue-green deploy job %s", j.job.Name)
	}
	if len(quoteJobs) > 1 {
		return errors.Errorf("more than one blue-green release job quote blue-green deploy job %s", j.job.Name)
	}
	jobRankmap := getJobRankMap(j.workflow.Stages)
	if jobRankmap[j.job.Name] >= jobRankmap[quoteJobs[0].Name] {
		return errors.Errorf("blue-green release job %s should run before blue-green deploy job %s", quoteJobs[0].Name, j.job.Name)
	}
	return nil
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
