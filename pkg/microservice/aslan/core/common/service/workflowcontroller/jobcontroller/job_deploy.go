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
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
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
	"github.com/koderover/zadig/pkg/tool/kube/informer"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
	versionedclient "istio.io/client-go/pkg/clientset/versioned"
	"k8s.io/client-go/kubernetes"
)

type DeployJobCtl struct {
	job         *commonmodels.JobTask
	namespace   string
	workflowCtx *commonmodels.WorkflowTaskCtx
	logger      *zap.SugaredLogger
	kubeClient  crClient.Client
	restConfig  *rest.Config
	informer    informers.SharedInformerFactory
	clientSet   *kubernetes.Clientset
	istioClient *versionedclient.Clientset
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

func (c *DeployJobCtl) getVarsYaml() (string, error) {
	vars := []*commonmodels.VariableKV{}
	for _, v := range c.jobTaskSpec.KeyVals {
		vars = append(vars, &commonmodels.VariableKV{Key: v.Key, Value: v.Value})
	}
	return kube.GenerateYamlFromKV(vars)
}

func (c *DeployJobCtl) run(ctx context.Context) error {
	var (
		err error
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
	c.clientSet, err = kubeclient.GetKubeClientSet(config.HubServerAddress(), c.jobTaskSpec.ClusterID)
	if err != nil {
		msg := fmt.Sprintf("can't init k8s clientset: %v", err)
		logError(c.job, msg, c.logger)
		return errors.New(msg)
	}

	c.informer, err = informer.NewInformer(c.jobTaskSpec.ClusterID, c.namespace, c.clientSet)
	if err != nil {
		msg := fmt.Sprintf("can't init k8s informer: %v", err)
		logError(c.job, msg, c.logger)
		return errors.New(msg)
	}

	c.istioClient, err = versionedclient.NewForConfig(c.restConfig)
	if err != nil {
		msg := fmt.Sprintf("can't init k8s istio client: %v", err)
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
	if c.jobTaskSpec.CreateEnvType == "system" {
		if err := c.updateSystemService(env); err != nil {
			logError(c.job, err.Error(), c.logger)
			return err
		}
	} else {
		errList := new(multierror.Error)
		wg := sync.WaitGroup{}
		for _, serviceModule := range c.jobTaskSpec.ServiceAndImages {
			wg.Add(1)
			go func(serviceModule *commonmodels.DeployServiceModule) {
				defer wg.Done()
				if err := c.updateExternalServiceModule(ctx, serviceInfo.WorkloadType, env, serviceModule); err != nil {
					errList = multierror.Append(errList, err)
				}
			}(serviceModule)
		}
		wg.Wait()
		if err := errList.ErrorOrNil(); err != nil {
			logError(c.job, err.Error(), c.logger)
			return err
		}
	}
	return nil
}

func (c *DeployJobCtl) updateSystemService(env *commonmodels.Product) error {
	var updateRevision bool
	if slices.Contains(c.jobTaskSpec.DeployContents, config.DeployConfig) && c.jobTaskSpec.UpdateConfig {
		updateRevision = true
	}
	varsYaml, err := c.getVarsYaml()
	if err != nil {
		msg := fmt.Sprintf("generate vars yaml error: %v", err)
		return errors.New(msg)
	}
	containers := []*commonmodels.Container{}
	if slices.Contains(c.jobTaskSpec.DeployContents, config.DeployImage) {
		for _, serviceImage := range c.jobTaskSpec.ServiceAndImages {
			containers = append(containers, &commonmodels.Container{Name: serviceImage.ServiceModule, Image: serviceImage.Image})
		}
	}
	option := &kube.GeneSvcYamlOption{ProductName: env.ProductName, EnvName: c.jobTaskSpec.Env, ServiceName: c.jobTaskSpec.ServiceName, UpdateServiceRevision: updateRevision, VariableYaml: varsYaml, Containers: containers}
	updatedYaml, revision, err := kube.GenerateRenderedYaml(option)
	if err != nil {
		msg := fmt.Sprintf("generate service yaml error: %v", err)
		return errors.New(msg)
	}

	c.jobTaskSpec.YamlContent = updatedYaml
	c.ack()

	currentYaml, _, err := kube.FetchCurrentAppliedYaml(option)
	if err != nil {
		msg := fmt.Sprintf("get current service yaml error: %v", err)
		return errors.New(msg)
	}
	unstructruedList, err := kube.CreateOrPatchResource(&kube.ResourceApplyParam{
		ServiceName:         c.jobTaskSpec.ServiceName,
		CurrentResourceYaml: currentYaml,
		UpdateResourceYaml:  updatedYaml,
		Informer:            c.informer,
		KubeClient:          c.kubeClient,
		AddZadigLabel:       !c.jobTaskSpec.Production,
		InjectSecrets:       true,
		SharedEnvHandler:    nil,
		ProductInfo:         env}, c.logger)

	if err != nil {
		msg := fmt.Sprintf("create or patch resource error: %v", err)
		return errors.New(msg)
	}

	err = UpdateProductServiceDeployInfo(&ProductServiceDeployInfo{
		ProductName:     env.ProductName,
		EnvName:         c.jobTaskSpec.Env,
		ServiceName:     c.jobTaskSpec.ServiceName,
		ServiceRevision: revision,
		VariableYaml:    varsYaml,
		Containers:      containers,
	})
	if err != nil {
		msg := fmt.Sprintf("update service render set info error: %v", err)
		return errors.New(msg)
	}
	for _, unstructrued := range unstructruedList {
		switch unstructrued.GetKind() {
		case setting.Deployment, setting.StatefulSet:
			podLabels, _, err := unstructured.NestedStringMap(unstructrued.Object, "spec", "template", "metadata", "labels")
			if err == nil {
				c.jobTaskSpec.RelatedPodLabels = append(c.jobTaskSpec.RelatedPodLabels, podLabels)
			}
			c.jobTaskSpec.ReplaceResources = append(c.jobTaskSpec.ReplaceResources, commonmodels.Resource{Name: unstructrued.GetName(), Kind: unstructrued.GetKind()})
		}
	}
	return nil
}

func (c *DeployJobCtl) updateExternalServiceModule(ctx context.Context, workloadType string, env *commonmodels.Product, serviceModule *commonmodels.DeployServiceModule) error {
	var err error
	var replaced bool
	if workloadType == "" {
		var deployments []*appsv1.Deployment
		var statefulSets []*appsv1.StatefulSet
		deployments, statefulSets, err = kube.FetchRelatedWorkloads(env.Namespace, c.jobTaskSpec.ServiceName, env, c.kubeClient)
		if err != nil {
			return err
		}

	L:
		for _, deploy := range deployments {
			for _, container := range deploy.Spec.Template.Spec.Containers {
				if container.Name == serviceModule.ServiceModule {
					err = updater.UpdateDeploymentImage(deploy.Namespace, deploy.Name, serviceModule.ServiceModule, serviceModule.Image, c.kubeClient)
					if err != nil {
						return fmt.Errorf("failed to update container image in %s/deployments/%s/%s: %v", env.Namespace, deploy.Name, container.Name, err)
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
				if container.Name == serviceModule.ServiceModule {
					err = updater.UpdateStatefulSetImage(sts.Namespace, sts.Name, serviceModule.ServiceModule, serviceModule.Image, c.kubeClient)
					if err != nil {
						return fmt.Errorf("failed to update container image in %s/statefulsets/%s/%s: %v", env.Namespace, sts.Name, container.Name, err)
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
		switch workloadType {
		case setting.StatefulSet:
			var statefulSet *appsv1.StatefulSet
			var found bool
			statefulSet, found, err = getter.GetStatefulSet(env.Namespace, c.jobTaskSpec.ServiceName, c.kubeClient)
			if err != nil {
				return err
			}
			if !found {
				return fmt.Errorf("statefulset %s not found", c.jobTaskSpec.ServiceName)
			}
			for _, container := range statefulSet.Spec.Template.Spec.Containers {
				if container.Name == serviceModule.ServiceModule {
					err = updater.UpdateStatefulSetImage(statefulSet.Namespace, statefulSet.Name, serviceModule.ServiceModule, serviceModule.Image, c.kubeClient)
					if err != nil {
						return fmt.Errorf("failed to update container image in %s/statefulsets/%s/%s: %v", env.Namespace, statefulSet.Name, container.Name, err)
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
				return err
			}
			if !found {
				return fmt.Errorf("deployment %s not found", c.jobTaskSpec.ServiceName)
			}
			for _, container := range deployment.Spec.Template.Spec.Containers {
				if container.Name == serviceModule.ServiceModule {
					err = updater.UpdateDeploymentImage(deployment.Namespace, deployment.Name, serviceModule.ServiceModule, serviceModule.Image, c.kubeClient)
					if err != nil {
						return fmt.Errorf("failed to update container image in %s/deployments/%s/%s: %v", env.Namespace, deployment.Name, container.Name, err)
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
		return fmt.Errorf("service %s container name %s is not found in env %s", c.jobTaskSpec.ServiceName, serviceModule.ServiceModule, c.jobTaskSpec.Env)
	}
	if err := updateProductImageByNs(env.Namespace, c.workflowCtx.ProjectName, c.jobTaskSpec.ServiceName, map[string]string{serviceModule.ServiceModule: serviceModule.Image}, c.logger); err != nil {
		return err
	}
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
