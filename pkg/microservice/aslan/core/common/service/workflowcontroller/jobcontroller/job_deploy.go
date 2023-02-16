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

package jobcontroller

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	"github.com/koderover/zadig/pkg/shared/kube/wrapper"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
)

type DeployJobCtl struct {
	job         *commonmodels.JobTask
	namespace   string
	workflowCtx *commonmodels.WorkflowTaskCtx
	logger      *zap.SugaredLogger
	kubeClient  crClient.Client
	restConfig  *rest.Config
	jobTaskSpec *commonmodels.JobTaskDeploySpec
	ack         func()
}

func NewDeployJobCtl(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, ack func(), logger *zap.SugaredLogger) *DeployJobCtl {
	jobTaskSpec := &commonmodels.JobTaskDeploySpec{}
	if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
		logger.Error(err)
	}
	job.Spec = jobTaskSpec
	return &DeployJobCtl{
		job:         job,
		workflowCtx: workflowCtx,
		logger:      logger,
		ack:         ack,
		jobTaskSpec: jobTaskSpec,
	}
}

func (c *DeployJobCtl) Clean(ctx context.Context) {}

func (c *DeployJobCtl) Run(ctx context.Context) {
	c.job.Status = config.StatusRunning
	c.ack()
	if err := c.run(ctx); err != nil {
		return
	}
	if c.jobTaskSpec.SkipCheckRunStatus {
		c.job.Status = config.StatusPassed
		return
	}
	c.wait(ctx)
}

func (c *DeployJobCtl) run(ctx context.Context) error {
	var (
		err      error
		replaced = false
	)
	env, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    c.workflowCtx.ProjectName,
		EnvName: c.jobTaskSpec.Env,
	})
	if err != nil {
		msg := fmt.Sprintf("find project error: %v", err)
		logError(c.job, msg, c.logger)
		return errors.New(msg)
	}
	c.namespace = env.Namespace
	c.jobTaskSpec.ClusterID = env.ClusterID

	c.restConfig, err = kubeclient.GetRESTConfig(config.HubServerAddress(), c.jobTaskSpec.ClusterID)
	if err != nil {
		msg := fmt.Sprintf("can't get k8s rest config: %v", err)
		logError(c.job, msg, c.logger)
		return errors.New(msg)
	}

	c.kubeClient, err = kubeclient.GetKubeClient(config.HubServerAddress(), c.jobTaskSpec.ClusterID)
	if err != nil {
		msg := fmt.Sprintf("can't init k8s client: %v", err)
		logError(c.job, msg, c.logger)
		return errors.New(msg)
	}

	// get servcie info
	var (
		serviceInfo *commonmodels.Service
	)
	serviceInfo, err = commonrepo.NewServiceColl().Find(
		&commonrepo.ServiceFindOption{
			ServiceName:   c.jobTaskSpec.ServiceName,
			ProductName:   c.workflowCtx.ProjectName,
			ExcludeStatus: setting.ProductStatusDeleting,
			Type:          c.jobTaskSpec.ServiceType,
		})
	if err != nil {
		// Maybe it is a share service, the entity is not under the project
		serviceInfo, err = commonrepo.NewServiceColl().Find(
			&commonrepo.ServiceFindOption{
				ServiceName:   c.jobTaskSpec.ServiceName,
				ExcludeStatus: setting.ProductStatusDeleting,
				Type:          c.jobTaskSpec.ServiceType,
			})
		if err != nil {
			msg := fmt.Sprintf("find service %s error: %v", c.jobTaskSpec.ServiceName, err)
			logError(c.job, msg, c.logger)
			return errors.New(msg)
		}
	}

	if serviceInfo.WorkloadType == "" {
		var deployments []*appsv1.Deployment
		var statefulSets []*appsv1.StatefulSet
		deployments, statefulSets, err = kube.FetchRelatedWorkloads(env.Namespace, c.jobTaskSpec.ServiceName, env, c.kubeClient)
		if err != nil {
			logError(c.job, err.Error(), c.logger)
			return err
		}

	L:
		for _, deploy := range deployments {
			for _, container := range deploy.Spec.Template.Spec.Containers {
				if container.Name == c.jobTaskSpec.ServiceModule {
					err = updater.UpdateDeploymentImage(deploy.Namespace, deploy.Name, c.jobTaskSpec.ServiceModule, c.jobTaskSpec.Image, c.kubeClient)
					if err != nil {
						msg := fmt.Sprintf("failed to update container image in %s/deployments/%s/%s: %v", env.Namespace, deploy.Name, container.Name, err)
						logError(c.job, msg, c.logger)
						return errors.New(msg)
					}
					c.jobTaskSpec.ReplaceResources = append(c.jobTaskSpec.ReplaceResources, commonmodels.Resource{
						Kind:      setting.Deployment,
						Container: container.Name,
						Origin:    container.Image,
						Name:      deploy.Name,
					})
					replaced = true
					c.jobTaskSpec.RelatedPodLabels = append(c.jobTaskSpec.RelatedPodLabels, deploy.Spec.Template.Labels)
					break L
				}
			}
		}
	Loop:
		for _, sts := range statefulSets {
			for _, container := range sts.Spec.Template.Spec.Containers {
				if container.Name == c.jobTaskSpec.ServiceModule {
					err = updater.UpdateStatefulSetImage(sts.Namespace, sts.Name, c.jobTaskSpec.ServiceModule, c.jobTaskSpec.Image, c.kubeClient)
					if err != nil {
						msg := fmt.Sprintf("failed to update container image in %s/statefulsets/%s/%s: %v", env.Namespace, sts.Name, container.Name, err)
						logError(c.job, msg, c.logger)
						return errors.New(msg)
					}
					c.jobTaskSpec.ReplaceResources = append(c.jobTaskSpec.ReplaceResources, commonmodels.Resource{
						Kind:      setting.StatefulSet,
						Container: container.Name,
						Origin:    container.Image,
						Name:      sts.Name,
					})
					replaced = true
					c.jobTaskSpec.RelatedPodLabels = append(c.jobTaskSpec.RelatedPodLabels, sts.Spec.Template.Labels)
					break Loop
				}
			}
		}
	} else {
		switch serviceInfo.WorkloadType {
		case setting.StatefulSet:
			var statefulSet *appsv1.StatefulSet
			var found bool
			statefulSet, found, err = getter.GetStatefulSet(env.Namespace, c.jobTaskSpec.ServiceName, c.kubeClient)
			if err != nil {
				logError(c.job, err.Error(), c.logger)
				return err
			}
			if !found {
				msg := fmt.Sprintf("statefulset %s not found", c.jobTaskSpec.ServiceName)
				logError(c.job, msg, c.logger)
				return errors.New(msg)
			}
			for _, container := range statefulSet.Spec.Template.Spec.Containers {
				if container.Name == c.jobTaskSpec.ServiceModule {
					err = updater.UpdateStatefulSetImage(statefulSet.Namespace, statefulSet.Name, c.jobTaskSpec.ServiceModule, c.jobTaskSpec.Image, c.kubeClient)
					if err != nil {
						msg := fmt.Sprintf("failed to update container image in %s/statefulsets/%s/%s: %v", env.Namespace, statefulSet.Name, container.Name, err)
						logError(c.job, msg, c.logger)
						return errors.New(msg)
					}
					c.jobTaskSpec.ReplaceResources = append(c.jobTaskSpec.ReplaceResources, commonmodels.Resource{
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
			var found bool
			deployment, found, err = getter.GetDeployment(env.Namespace, c.jobTaskSpec.ServiceName, c.kubeClient)
			if err != nil {
				logError(c.job, err.Error(), c.logger)
				return err
			}
			if !found {
				msg := fmt.Sprintf("deployment %s not found", c.jobTaskSpec.ServiceName)
				logError(c.job, msg, c.logger)
				return errors.New(msg)
			}
			for _, container := range deployment.Spec.Template.Spec.Containers {
				if container.Name == c.jobTaskSpec.ServiceModule {
					err = updater.UpdateDeploymentImage(deployment.Namespace, deployment.Name, c.jobTaskSpec.ServiceModule, c.jobTaskSpec.Image, c.kubeClient)
					if err != nil {
						msg := fmt.Sprintf("failed to update container image in %s/deployments/%s/%s: %v", env.Namespace, deployment.Name, container.Name, err)
						logError(c.job, msg, c.logger)
						return errors.New(msg)
					}
					c.jobTaskSpec.ReplaceResources = append(c.jobTaskSpec.ReplaceResources, commonmodels.Resource{
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
		msg := fmt.Sprintf("service %s container name %s is not found in env %s", c.jobTaskSpec.ServiceName, c.jobTaskSpec.ServiceModule, c.jobTaskSpec.Env)
		logError(c.job, msg, c.logger)
		return errors.New(msg)
	}
	if err := updateProductImageByNs(env.Namespace, c.workflowCtx.ProjectName, c.jobTaskSpec.ServiceName, map[string]string{c.jobTaskSpec.ServiceModule: c.jobTaskSpec.Image}, c.logger); err != nil {
		c.logger.Error(err)
	}
	c.job.Spec = c.jobTaskSpec
	return nil
}

func workLoadDeployStat(kubeClient client.Client, namespace string, labelMaps []map[string]string) error {
	for _, label := range labelMaps {
		selector := labels.Set(label).AsSelector()
		pods, err := getter.ListPods(namespace, selector, kubeClient)
		if err != nil {
			return err
		}
		for _, pod := range pods {
			allContainerStatuses := make([]corev1.ContainerStatus, 0)
			allContainerStatuses = append(allContainerStatuses, pod.Status.InitContainerStatuses...)
			allContainerStatuses = append(allContainerStatuses, pod.Status.ContainerStatuses...)
			for _, cs := range allContainerStatuses {
				if cs.State.Waiting != nil {
					switch cs.State.Waiting.Reason {
					case "ImagePullBackOff", "ErrImagePull", "CrashLoopBackOff", "ErrImageNeverPull":
						return fmt.Errorf("pod: %s, %s: %s", pod.Name, cs.State.Waiting.Reason, cs.State.Waiting.Message)
					}
				}
			}
		}
	}
	return nil
}

func (c *DeployJobCtl) wait(ctx context.Context) {
	timeout := time.After(time.Duration(c.timeout()) * time.Second)

	for {
		select {
		case <-ctx.Done():
			c.job.Status = config.StatusCancelled
			return

		case <-timeout:
			var msg []string
			for _, label := range c.jobTaskSpec.RelatedPodLabels {
				selector := labels.Set(label).AsSelector()
				pods, err := getter.ListPods(c.namespace, selector, c.kubeClient)
				if err != nil {
					msg := fmt.Sprintf("list pods error: %v", err)
					logError(c.job, msg, c.logger)
					return
				}
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
			}

			if len(msg) != 0 {
				err := errors.New(strings.Join(msg, "\n"))
				logError(c.job, err.Error(), c.logger)
				return
			}
			c.job.Status = config.StatusTimeout
			return

		default:
			time.Sleep(time.Second * 2)
			ready := true
			var err error
		L:
			for _, resource := range c.jobTaskSpec.ReplaceResources {
				if err := workLoadDeployStat(c.kubeClient, c.namespace, c.jobTaskSpec.RelatedPodLabels); err != nil {
					logError(c.job, err.Error(), c.logger)
					return
				}
				switch resource.Kind {
				case setting.Deployment:
					d, found, e := getter.GetDeployment(c.namespace, resource.Name, c.kubeClient)
					if e != nil {
						err = e
					}
					if e != nil || !found {
						c.logger.Errorf(
							"failed to check deployment ready status %s/%s/%s - %v",
							c.namespace,
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
					st, found, e := getter.GetStatefulSet(c.namespace, resource.Name, c.kubeClient)
					if e != nil {
						err = e
					}
					if err != nil || !found {
						c.logger.Errorf(
							"failed to check statefulSet ready status %s/%s/%s",
							c.namespace,
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
				c.job.Status = config.StatusPassed
				return
			}
		}
	}
}

func (c *DeployJobCtl) timeout() int {
	if c.jobTaskSpec.Timeout == 0 {
		c.jobTaskSpec.Timeout = setting.DeployTimeout
	}
	return c.jobTaskSpec.Timeout
}
