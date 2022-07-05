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

package stepcontroller

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	"github.com/koderover/zadig/pkg/shared/kube/wrapper"
	krkubeclient "github.com/koderover/zadig/pkg/tool/kube/client"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
	"github.com/koderover/zadig/pkg/types/step"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type deployCtl struct {
	step        *commonmodels.StepTask
	deploySpec  *step.StepDeploySpec
	workflowCtx *commonmodels.WorkflowTaskCtx
	namespace   string
	log         *zap.SugaredLogger
	kubeClient  client.Client
	restConfig  *rest.Config
}

func NewDeployCtl(stepTask *commonmodels.StepTask, workflowCtx *commonmodels.WorkflowTaskCtx, log *zap.SugaredLogger) (*deployCtl, error) {
	yamlString, err := yaml.Marshal(stepTask.Spec)
	if err != nil {
		return nil, fmt.Errorf("marshal deploy spec error: %v", err)
	}
	deploySpec := &step.StepDeploySpec{}
	if err := yaml.Unmarshal(yamlString, &deploySpec); err != nil {
		return nil, fmt.Errorf("unmarshal deploy spec error: %v", err)
	}
	return &deployCtl{deploySpec: deploySpec, workflowCtx: workflowCtx, log: log, step: stepTask}, nil
}

func (s *deployCtl) PreRun(ctx context.Context) error {
	return nil
}

func (s *deployCtl) Run(ctx context.Context) (config.Status, error) {
	if err := s.run(ctx); err != nil {
		return config.StatusFailed, err
	}
	return s.wait(ctx)
}

func (s *deployCtl) AfterRun(ctx context.Context) error {
	return nil
}

func (s *deployCtl) run(ctx context.Context) error {
	var (
		err      error
		replaced = false
	)
	env, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    s.workflowCtx.ProjectName,
		EnvName: s.deploySpec.Env,
	})
	if err != nil {
		return err
	}
	s.namespace = env.Namespace
	s.deploySpec.ClusterID = env.ClusterID

	if s.deploySpec.ClusterID != "" {
		s.restConfig, err = kubeclient.GetRESTConfig(config.HubServerAddress(), s.deploySpec.ClusterID)
		if err != nil {
			err = errors.WithMessage(err, "can't get k8s rest config")
			return err
		}

		s.kubeClient, err = kubeclient.GetKubeClient(config.HubServerAddress(), s.deploySpec.ClusterID)
		if err != nil {
			err = errors.WithMessage(err, "can't init k8s client")
			return err
		}
	} else {
		s.kubeClient = krkubeclient.Client()
		s.restConfig = krkubeclient.RESTConfig()
	}

	// get servcie info
	var (
		serviceInfo *commonmodels.Service
		selector    labels.Selector
	)
	serviceInfo, err = commonrepo.NewServiceColl().Find(
		&commonrepo.ServiceFindOption{
			ServiceName:   s.deploySpec.ServiceName,
			ProductName:   s.workflowCtx.ProjectName,
			ExcludeStatus: setting.ProductStatusDeleting,
			Type:          s.deploySpec.ServiceType,
		})
	if err != nil {
		// Maybe it is a share service, the entity is not under the project
		serviceInfo, err = commonrepo.NewServiceColl().Find(
			&commonrepo.ServiceFindOption{
				ServiceName:   s.deploySpec.ServiceName,
				ExcludeStatus: setting.ProductStatusDeleting,
				Type:          s.deploySpec.ServiceType,
			})
		if err != nil {
			return err
		}
	}

	if serviceInfo.WorkloadType == "" {
		selector = labels.Set{setting.ProductLabel: s.workflowCtx.ProjectName, setting.ServiceLabel: s.deploySpec.ServiceName}.AsSelector()

		var deployments []*appsv1.Deployment
		deployments, err = getter.ListDeployments(env.Namespace, selector, s.kubeClient)
		if err != nil {
			return err
		}

		var statefulSets []*appsv1.StatefulSet
		statefulSets, err = getter.ListStatefulSets(env.Namespace, selector, s.kubeClient)
		if err != nil {
			return err
		}

	L:
		for _, deploy := range deployments {
			for _, container := range deploy.Spec.Template.Spec.Containers {
				if container.Name == s.deploySpec.ServiceModule {
					err = updater.UpdateDeploymentImage(deploy.Namespace, deploy.Name, s.deploySpec.ServiceModule, s.deploySpec.Image, s.kubeClient)
					if err != nil {
						err = errors.WithMessagef(
							err,
							"failed to update container image in %s/deployments/%s/%s",
							env.Namespace, deploy.Name, container.Name)
						return err
					}
					s.deploySpec.ReplaceResources = append(s.deploySpec.ReplaceResources, step.Resource{
						Kind:      setting.Deployment,
						Container: container.Name,
						Origin:    container.Image,
						Name:      deploy.Name,
					})
					replaced = true
					break L
				}
			}
		}
	Loop:
		for _, sts := range statefulSets {
			for _, container := range sts.Spec.Template.Spec.Containers {
				if container.Name == s.deploySpec.ServiceModule {
					err = updater.UpdateStatefulSetImage(sts.Namespace, sts.Name, s.deploySpec.ServiceModule, s.deploySpec.Image, s.kubeClient)
					if err != nil {
						err = errors.WithMessagef(
							err,
							"failed to update container image in %s/statefulsets/%s/%s",
							env.Namespace, sts.Name, container.Name)
						return err
					}
					s.deploySpec.ReplaceResources = append(s.deploySpec.ReplaceResources, step.Resource{
						Kind:      setting.StatefulSet,
						Container: container.Name,
						Origin:    container.Image,
						Name:      sts.Name,
					})
					replaced = true
					break Loop
				}
			}
		}
	} else {
		switch serviceInfo.WorkloadType {
		case setting.StatefulSet:
			var statefulSet *appsv1.StatefulSet
			statefulSet, _, err = getter.GetStatefulSet(env.Namespace, s.deploySpec.ServiceName, s.kubeClient)
			if err != nil {
				return err
			}
			for _, container := range statefulSet.Spec.Template.Spec.Containers {
				if container.Name == s.deploySpec.ServiceModule {
					err = updater.UpdateStatefulSetImage(statefulSet.Namespace, statefulSet.Name, s.deploySpec.ServiceModule, s.deploySpec.Image, s.kubeClient)
					if err != nil {
						err = errors.WithMessagef(
							err,
							"failed to update container image in %s/statefulsets/%s/%s",
							env.Namespace, statefulSet.Name, container.Name)
						return err
					}
					s.deploySpec.ReplaceResources = append(s.deploySpec.ReplaceResources, step.Resource{
						Kind:      setting.StatefulSet,
						Container: container.Name,
						Origin:    container.Image,
						Name:      statefulSet.Name,
					})
					replaced = true
					break
				}
			}
		case setting.Deployment:
			var deployment *appsv1.Deployment
			deployment, _, err = getter.GetDeployment(env.Namespace, s.deploySpec.ServiceName, s.kubeClient)
			if err != nil {
				return err
			}
			for _, container := range deployment.Spec.Template.Spec.Containers {
				if container.Name == s.deploySpec.ServiceModule {
					err = updater.UpdateDeploymentImage(deployment.Namespace, deployment.Name, s.deploySpec.ServiceModule, s.deploySpec.Image, s.kubeClient)
					if err != nil {
						err = errors.WithMessagef(
							err,
							"failed to update container image in %s/deployments/%s/%s",
							env.Namespace, deployment.Name, container.Name)
						return err
					}
					s.deploySpec.ReplaceResources = append(s.deploySpec.ReplaceResources, step.Resource{
						Kind:      setting.Deployment,
						Container: container.Name,
						Origin:    container.Image,
						Name:      deployment.Name,
					})
					replaced = true
					break
				}
			}
		}
	}
	if !replaced {
		err = errors.Errorf("service %s container name %s is not found in env %s", s.deploySpec.ServiceName, s.deploySpec.ServiceModule, s.deploySpec.Env)
		return err
	}
	s.step.Spec = s.deploySpec
	return nil
}

func (s *deployCtl) wait(ctx context.Context) (config.Status, error) {
	timeout := time.After(time.Duration(s.timeout()) * time.Second)

	selector := labels.Set{setting.ProductLabel: s.workflowCtx.ProjectName, setting.ServiceLabel: s.deploySpec.ServiceName}.AsSelector()

	for {
		select {
		case <-ctx.Done():
			return config.StatusCancelled, nil

		case <-timeout:
			pods, err := getter.ListPods(s.namespace, selector, s.kubeClient)
			if err != nil {
				return config.StatusFailed, err
			}

			var msg []string
			for _, pod := range pods {
				podResource := wrapper.Pod(pod).Resource()
				if podResource.Status != setting.StatusRunning && podResource.Status != setting.StatusSucceeded {
					for _, cs := range podResource.ContainerStatuses {
						// message为空不认为是错误状态，有可能还在waiting
						if cs.Message != "" {
							msg = append(msg, fmt.Sprintf("Status: %s, Reason: %s, Message: %s", cs.Status, cs.Reason, cs.Message))
						}
					}
				}
			}

			if len(msg) != 0 {
				err = errors.New(strings.Join(msg, "\n"))
			}

			return config.StatusTimeout, err

		default:
			time.Sleep(time.Second * 2)
			ready := true
			var err error
		L:
			for _, resource := range s.deploySpec.ReplaceResources {
				switch resource.Kind {
				case setting.Deployment:
					d, found, e := getter.GetDeployment(s.namespace, resource.Name, s.kubeClient)
					if e != nil {
						err = e
					}
					if e != nil || !found {
						s.log.Errorf(
							"failed to check deployment ready status %s/%s/%s - %v",
							s.namespace,
							resource.Kind,
							resource.Name,
							e,
						)
						ready = false
					} else {
						ready = wrapper.Deployment(d).Ready()
					}

					if !ready {
						break L
					}
				case setting.StatefulSet:
					st, found, e := getter.GetStatefulSet(s.namespace, resource.Name, s.kubeClient)
					if e != nil {
						err = e
					}
					if err != nil || !found {
						s.log.Errorf(
							"failed to check statefulSet ready status %s/%s/%s",
							s.namespace,
							resource.Kind,
							resource.Name,
							e,
						)
						ready = false
					} else {
						ready = wrapper.StatefulSet(st).Ready()
					}

					if !ready {
						break L
					}
				}
			}

			if ready {
				return config.StatusPassed, nil
			}
		}
	}
}

func (s *deployCtl) timeout() int {
	if s.deploySpec.Timeout == 0 {
		s.deploySpec.Timeout = setting.DeployTimeout
	}
	return s.deploySpec.Timeout
}
