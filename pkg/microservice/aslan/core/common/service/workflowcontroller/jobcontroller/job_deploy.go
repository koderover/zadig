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
	"golang.org/x/exp/slices"
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

	var updateRevision bool
	if slices.Contains(c.jobTaskSpec.DeployContents, config.DeployConfig) && c.jobTaskSpec.UpdateConfig {
		updateRevision = true
	}
	varsYaml, err := c.getVarsYaml()
	if err != nil {
		msg := fmt.Sprintf("generate vars yaml error: %v", err)
		logError(c.job, msg, c.logger)
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
		logError(c.job, msg, c.logger)
		return errors.New(msg)
	}
	currentYaml, _, err := kube.FetchCurrentAppliedYaml(option)
	if err != nil {
		msg := fmt.Sprintf("get current service yaml error: %v", err)
		logError(c.job, msg, c.logger)
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
		logError(c.job, msg, c.logger)
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
		logError(c.job, msg, c.logger)
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
