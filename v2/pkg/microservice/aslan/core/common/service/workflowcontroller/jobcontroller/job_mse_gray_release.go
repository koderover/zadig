/*
Copyright 2023 The KodeRover Authors.

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
	"time"

	"go.uber.org/zap"
	"helm.sh/helm/v3/pkg/releaseutil"
	versionedclient "istio.io/client-go/pkg/clientset/versioned"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	crClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
	kubeclient "github.com/koderover/zadig/v2/pkg/shared/kube/client"
	"github.com/koderover/zadig/v2/pkg/tool/kube/informer"
	"github.com/koderover/zadig/v2/pkg/tool/kube/serializer"
)

type MseGrayReleaseJobCtl struct {
	job         *commonmodels.JobTask
	workflowCtx *commonmodels.WorkflowTaskCtx
	logger      *zap.SugaredLogger
	jobTaskSpec *commonmodels.JobTaskMseGrayReleaseSpec
	kubeClient  crClient.Client
	restConfig  *rest.Config
	informer    informers.SharedInformerFactory
	clientSet   *kubernetes.Clientset
	istioClient *versionedclient.Clientset
	namespace   string
	ack         func()
}

func NewMseGrayReleaseJobCtl(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, ack func(), logger *zap.SugaredLogger) *MseGrayReleaseJobCtl {
	jobTaskSpec := &commonmodels.JobTaskMseGrayReleaseSpec{}
	if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
		logger.Error(err)
	}
	job.Spec = jobTaskSpec
	return &MseGrayReleaseJobCtl{
		job:         job,
		workflowCtx: workflowCtx,
		logger:      logger,
		ack:         ack,
		jobTaskSpec: jobTaskSpec,
	}
}

func (c *MseGrayReleaseJobCtl) Clean(ctx context.Context) {}

func (c *MseGrayReleaseJobCtl) Run(ctx context.Context) {
	c.job.Status = config.StatusRunning
	c.ack()

	env, err := mongodb.NewProductColl().Find(&mongodb.ProductFindOptions{
		Name:    c.workflowCtx.ProjectName,
		EnvName: c.jobTaskSpec.GrayEnv,
	})
	if err != nil {
		msg := fmt.Sprintf("find project error: %v", err)
		logError(c.job, msg, c.logger)
		return
	}
	c.namespace = env.Namespace
	clusterID := env.ClusterID

	c.restConfig, err = kubeclient.GetRESTConfig(config.HubServerAddress(), clusterID)
	if err != nil {
		msg := fmt.Sprintf("can't get k8s rest config: %v", err)
		logError(c.job, msg, c.logger)
		return
	}

	c.kubeClient, err = kubeclient.GetKubeClient(config.HubServerAddress(), clusterID)
	if err != nil {
		msg := fmt.Sprintf("can't init k8s client: %v", err)
		logError(c.job, msg, c.logger)
		return
	}
	c.clientSet, err = kubeclient.GetKubeClientSet(config.HubServerAddress(), clusterID)
	if err != nil {
		msg := fmt.Sprintf("can't init k8s clientset: %v", err)
		logError(c.job, msg, c.logger)
		return
	}

	c.informer, err = informer.NewInformer(clusterID, c.namespace, c.clientSet)
	if err != nil {
		msg := fmt.Sprintf("can't init k8s informer: %v", err)
		logError(c.job, msg, c.logger)
		return
	}

	resources := make([]*unstructured.Unstructured, 0)
	service := c.jobTaskSpec.GrayService
	manifests := releaseutil.SplitManifests(service.YamlContent)
	for _, item := range manifests {
		u, err := serializer.NewDecoder().YamlToUnstructured([]byte(item))
		if err != nil {
			logError(c.job, fmt.Sprintf("failed to decode service %s yaml to unstructured: %v", service.ServiceName, err), c.logger)
			return
		}
		resources = append(resources, u)
	}
	deploymentName := ""
	for _, resource := range resources {
		switch resource.GetKind() {
		case setting.Deployment:
			deploymentObj := &v1.Deployment{}
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(resource.Object, deploymentObj)
			if err != nil {
				logError(c.job, fmt.Sprintf("failed to convert service %s deployment to deployment object: %v", service.ServiceName, err), c.logger)
				return
			}
			deploymentObj.SetNamespace(c.namespace)
			err = c.kubeClient.Create(context.Background(), deploymentObj)
			if err != nil {
				c.Error(fmt.Sprintf("failed to create deployment %s: %v", deploymentObj.Name, err))
				return
			}
			c.Info(fmt.Sprintf("create deployment %s successfully", deploymentObj.Name))
			deploymentName = deploymentObj.Name
		case setting.ConfigMap:
			configMapObj := &corev1.ConfigMap{}
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(resource.Object, configMapObj)
			if err != nil {
				logError(c.job, fmt.Sprintf("failed to convert service %s configmap to configmap object: %v", service.ServiceName, err), c.logger)
				return
			}
			configMapObj.SetNamespace(c.namespace)
			err = c.kubeClient.Create(context.Background(), configMapObj)
			if err != nil {
				c.Error(fmt.Sprintf("failed to create configmap %s: %v", configMapObj.Name, err))
				return
			}
			c.Info(fmt.Sprintf("create configmap %s successfully", configMapObj.Name))
		case setting.Secret:
			secretObj := &corev1.Secret{}
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(resource.Object, secretObj)
			if err != nil {
				logError(c.job, fmt.Sprintf("failed to convert service %s secret to secret object: %v", service.ServiceName, err), c.logger)
				return
			}
			secretObj.SetNamespace(c.namespace)
			err = c.kubeClient.Create(context.Background(), secretObj)
			if err != nil {
				c.Error(fmt.Sprintf("failed to create secret %s: %v", secretObj.Name, err))
				return
			}
			c.Info(fmt.Sprintf("create secret %s successfully", secretObj.Name))
		case setting.Service:
			serviceObj := &corev1.Service{}
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(resource.Object, serviceObj)
			if err != nil {
				logError(c.job, fmt.Sprintf("failed to convert service %s service to service object: %v", service.ServiceName, err), c.logger)
				return
			}
			serviceObj.SetNamespace(c.namespace)
			err = c.kubeClient.Create(context.Background(), serviceObj)
			if err != nil {
				c.Error(fmt.Sprintf("failed to create service %s: %v", serviceObj.Name, err))
				return
			}
			c.Info(fmt.Sprintf("create service %s successfully", serviceObj.Name))
		case setting.Ingress:
			ingressObj := &networkingv1.Ingress{}
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(resource.Object, ingressObj)
			if err != nil {
				logError(c.job, fmt.Sprintf("failed to convert service %s ingress to ingress object: %v", service.ServiceName, err), c.logger)
				return
			}
			ingressObj.SetNamespace(c.namespace)
			err = c.kubeClient.Create(context.Background(), ingressObj)
			if err != nil {
				c.Error(fmt.Sprintf("failed to create ingress %s: %v", ingressObj.Name, err))
				return
			}
			c.Info(fmt.Sprintf("create ingress %s successfully", ingressObj.Name))
		default:
			c.Error(fmt.Sprintf("service %s resource type %s not allowed", service.ServiceName, resource.GetKind()))
			return
		}
	}

	if !c.jobTaskSpec.SkipCheckRunStatus && deploymentName != "" {
		c.Info(fmt.Sprintf("waiting for deployment %s ready", deploymentName))
		timeout := time.After(c.timeout() * time.Second)
		for {
			select {
			case <-ctx.Done():
				c.job.Status = config.StatusCancelled
				return
			case <-timeout:
				c.Error(fmt.Sprintf("deployment %s not ready in %d seconds", deploymentName, c.timeout()))
				return
			default:
			}
			time.Sleep(3 * time.Second)
			deploymentObj := &v1.Deployment{}
			err := c.kubeClient.Get(context.Background(), types.NamespacedName{
				Namespace: c.namespace,
				Name:      deploymentName,
			}, deploymentObj)
			if err != nil {
				c.Error(fmt.Sprintf("failed to get deployment %s: %v", deploymentName, err))
				return
			}
			if deploymentObj.Status.ReadyReplicas == deploymentObj.Status.Replicas {
				c.Info(fmt.Sprintf("deployment %s ready", deploymentName))
				break
			}
		}
	}

	c.job.Status = config.StatusPassed
	return
}

func (c *MseGrayReleaseJobCtl) Info(msg string) {
	c.jobTaskSpec.Events = append(c.jobTaskSpec.Events, &commonmodels.Event{
		EventType: "info",
		Message:   msg,
		Time:      time.Now().Format("2006-01-02 15:04:05"),
	})
	c.ack()
}

func (c *MseGrayReleaseJobCtl) Error(msg string) {
	c.jobTaskSpec.Events = append(c.jobTaskSpec.Events, &commonmodels.Event{
		EventType: "error",
		Message:   msg,
		Time:      time.Now().Format("2006-01-02 15:04:05"),
	})
	logError(c.job, msg, c.logger)
}

func (c *MseGrayReleaseJobCtl) timeout() time.Duration {
	if c.jobTaskSpec.Timeout == 0 {
		c.jobTaskSpec.Timeout = setting.DeployTimeout
	}
	return time.Duration(c.jobTaskSpec.Timeout)
}

func (c *MseGrayReleaseJobCtl) SaveInfo(ctx context.Context) error {
	return mongodb.NewJobInfoColl().Create(context.TODO(), &commonmodels.JobInfo{
		Type:                c.job.JobType,
		WorkflowName:        c.workflowCtx.WorkflowName,
		WorkflowDisplayName: c.workflowCtx.WorkflowDisplayName,
		TaskID:              c.workflowCtx.TaskID,
		ProductName:         c.workflowCtx.ProjectName,
		StartTime:           c.job.StartTime,
		EndTime:             c.job.EndTime,
		Duration:            c.job.EndTime - c.job.StartTime,
		Status:              string(c.job.Status),
	})
}
