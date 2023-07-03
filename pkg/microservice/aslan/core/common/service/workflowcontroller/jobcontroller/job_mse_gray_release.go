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

	"go.uber.org/zap"
	"helm.sh/helm/v3/pkg/releaseutil"
	versionedclient "istio.io/client-go/pkg/clientset/versioned"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	crClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	"github.com/koderover/zadig/pkg/tool/kube/informer"
	"github.com/koderover/zadig/pkg/tool/kube/serializer"
	"github.com/koderover/zadig/pkg/types"
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

	var (
		err error
	)
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
	for _, resource := range resources {
		switch resource.GetKind() {
		case setting.Deployment:
			deploymentObj := &v1.Deployment{}
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(resource.Object, deploymentObj)
			if err != nil {
				logError(c.job, fmt.Sprintf("failed to convert service %s deployment to deployment object: %v", service.ServiceName, err), c.logger)
				return
			}
			if deploymentObj.Labels == nil {
				logError(c.job, fmt.Sprintf("service %s deployment label nil", service.ServiceName), c.logger)
				return
			}
			setLabels(deploymentObj.Labels, c.jobTaskSpec.GrayTag, service.ServiceName)
			setLabels(deploymentObj.Spec.Selector.MatchLabels, c.jobTaskSpec.GrayTag, service.ServiceName)
			setLabels(deploymentObj.Spec.Template.Labels, c.jobTaskSpec.GrayTag, service.ServiceName)
			//c.kubeClient.Create()
		case setting.ConfigMap:
			configMapObj := &corev1.ConfigMap{}
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(resource.Object, configMapObj)
			if err != nil {
				logError(c.job, fmt.Sprintf("failed to convert service %s configmap to configmap object: %v", service.ServiceName, err), c.logger)
				return
			}
			setLabels(configMapObj.Labels, c.jobTaskSpec.GrayTag, service.ServiceName)
		default:
			//return resp, errors.Errorf("service %s resource type %s not allowed", service.ServiceName, resource.GetKind())

		}
	}

	return
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

func setLabels(labels map[string]string, grayTag, serviceName string) {
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[types.ZadigReleaseVersionLabelKey] = grayTag
	labels[types.ZadigReleaseMSEGrayTagLabelKey] = grayTag
	labels[types.ZadigReleaseTypeLabelKey] = "gray"
	labels[types.ZadigReleaseServiceNameLabelKey] = serviceName
}
