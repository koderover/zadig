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
	"strings"
	"time"

	"go.uber.org/zap"
	versionedclient "istio.io/client-go/pkg/clientset/versioned"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	crClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	kubeclient "github.com/koderover/zadig/v2/pkg/shared/kube/client"
	"github.com/koderover/zadig/v2/pkg/tool/kube/getter"
	"github.com/koderover/zadig/v2/pkg/tool/kube/informer"
	"github.com/koderover/zadig/v2/pkg/types"
)

type MseGrayOfflineJobCtl struct {
	job         *commonmodels.JobTask
	workflowCtx *commonmodels.WorkflowTaskCtx
	logger      *zap.SugaredLogger
	jobTaskSpec *commonmodels.JobTaskMseGrayOfflineSpec
	kubeClient  crClient.Client
	restConfig  *rest.Config
	informer    informers.SharedInformerFactory
	clientSet   *kubernetes.Clientset
	istioClient *versionedclient.Clientset
	ack         func()
}

func NewMseGrayOfflineJobCtl(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, ack func(), logger *zap.SugaredLogger) *MseGrayOfflineJobCtl {
	jobTaskSpec := &commonmodels.JobTaskMseGrayOfflineSpec{}
	if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
		logger.Error(err)
	}
	job.Spec = jobTaskSpec
	return &MseGrayOfflineJobCtl{
		job:         job,
		workflowCtx: workflowCtx,
		logger:      logger,
		ack:         ack,
		jobTaskSpec: jobTaskSpec,
	}
}

func (c *MseGrayOfflineJobCtl) Clean(ctx context.Context) {}

func (c *MseGrayOfflineJobCtl) Run(ctx context.Context) {
	c.job.Status = config.StatusRunning
	c.ack()

	env, err := mongodb.NewProductColl().Find(&mongodb.ProductFindOptions{
		Name:    c.workflowCtx.ProjectName,
		EnvName: c.jobTaskSpec.Env,
	})
	if err != nil {
		msg := fmt.Sprintf("find env %s error: %v", c.jobTaskSpec.Env, err)
		logError(c.job, msg, c.logger)
		return
	}
	c.jobTaskSpec.Namespace = env.Namespace
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
	c.informer, err = informer.NewInformer(clusterID, c.jobTaskSpec.Namespace, c.clientSet)
	if err != nil {
		msg := fmt.Sprintf("can't init k8s informer: %v", err)
		logError(c.job, msg, c.logger)
		return
	}
	selector := labels.Set{
		types.ZadigReleaseTypeLabelKey:    types.ZadigReleaseTypeMseGray,
		types.ZadigReleaseVersionLabelKey: c.jobTaskSpec.GrayTag,
	}.AsSelector()
	deploymentList, err := getter.ListDeployments(c.jobTaskSpec.Namespace, selector, c.kubeClient)
	if err != nil {
		logError(c.job, fmt.Sprintf("can't list deployment: %v", err), c.logger)
		return
	}
	configMapList, err := getter.ListConfigMaps(c.jobTaskSpec.Namespace, selector, c.kubeClient)
	if err != nil {
		logError(c.job, fmt.Sprintf("can't list configmap: %v", err), c.logger)
		return
	}
	secretList, err := getter.ListSecrets(c.jobTaskSpec.Namespace, selector, c.kubeClient)
	if err != nil {
		logError(c.job, fmt.Sprintf("can't list secret: %v", err), c.logger)
		return
	}
	serviceList, err := getter.ListServices(c.jobTaskSpec.Namespace, selector, c.kubeClient)
	if err != nil {
		logError(c.job, fmt.Sprintf("can't list service: %v", err), c.logger)
		return
	}
	version, err := c.clientSet.Discovery().ServerVersion()
	if err != nil {
		logError(c.job, fmt.Sprintf("Failed to determine server version, error is: %s", err), c.logger)
		return
	}
	var (
		ingressExtentionList  []*extensionsv1beta1.Ingress
		ingressNetworkingList []*networkingv1.Ingress
	)
	if kubeclient.VersionLessThan122(version) {
		ingressExtentionList, err = getter.ListExtensionsV1Beta1Ingresses(selector, c.informer)
		if err != nil {
			logError(c.job, fmt.Sprintf("can't list ingress: %v", err), c.logger)
			return
		}
	} else {
		ingressNetworkingList, err = getter.ListNetworkingV1Ingress(selector, c.informer)
		if err != nil {
			logError(c.job, fmt.Sprintf("can't list ingress: %v", err), c.logger)
			return
		}
	}

	serviceAndErrorMap := make(map[string][]string)
	for _, deployment := range deploymentList {
		err := c.kubeClient.Delete(context.Background(), deployment)
		if serviceName, ok := deployment.GetLabels()[types.ZadigReleaseServiceNameLabelKey]; ok {
			if serviceAndErrorMap[serviceName] == nil {
				serviceAndErrorMap[serviceName] = []string{}
			}
			if err != nil {
				serviceAndErrorMap[serviceName] = append(serviceAndErrorMap[serviceName], fmt.Sprintf("delete deployment %s error: %v", deployment.Name, err))
				c.Error(fmt.Sprintf("delete deployment %s error: %v", deployment.Name, err))
				continue
			}
			c.Info(fmt.Sprintf("delete deployment %s success", deployment.Name))
		}
	}
	for _, configMap := range configMapList {
		err := c.kubeClient.Delete(context.Background(), configMap)
		if serviceName, ok := configMap.GetLabels()[types.ZadigReleaseServiceNameLabelKey]; ok {
			if serviceAndErrorMap[serviceName] == nil {
				serviceAndErrorMap[serviceName] = []string{}
			}
			if err != nil {
				serviceAndErrorMap[serviceName] = append(serviceAndErrorMap[serviceName], fmt.Sprintf("delete configmap %s error: %v", configMap.Name, err))
				c.Error(fmt.Sprintf("delete configmap %s error: %v", configMap.Name, err))
				continue
			}
			c.Info(fmt.Sprintf("delete configmap %s success", configMap.Name))
		}
	}
	for _, secret := range secretList {
		err := c.kubeClient.Delete(context.Background(), secret)
		if serviceName, ok := secret.GetLabels()[types.ZadigReleaseServiceNameLabelKey]; ok {
			if serviceAndErrorMap[serviceName] == nil {
				serviceAndErrorMap[serviceName] = []string{}
			}
			if err != nil {
				serviceAndErrorMap[serviceName] = append(serviceAndErrorMap[serviceName], fmt.Sprintf("delete secret %s error: %v", secret.Name, err))
				c.Error(fmt.Sprintf("delete secret %s error: %v", secret.Name, err))
				continue
			}
			c.Info(fmt.Sprintf("delete secret %s success", secret.Name))
		}
	}
	for _, service := range serviceList {
		err := c.kubeClient.Delete(context.Background(), service)
		if serviceName, ok := service.GetLabels()[types.ZadigReleaseServiceNameLabelKey]; ok {
			if serviceAndErrorMap[serviceName] == nil {
				serviceAndErrorMap[serviceName] = []string{}
			}
			if err != nil {
				serviceAndErrorMap[serviceName] = append(serviceAndErrorMap[serviceName], fmt.Sprintf("delete service %s error: %v", service.Name, err))
				c.Error(fmt.Sprintf("delete service %s error: %v", service.Name, err))
				continue
			}
			c.Info(fmt.Sprintf("delete service %s success", service.Name))
		}
	}
	for _, ingress := range ingressExtentionList {
		err := c.kubeClient.Delete(context.Background(), ingress)
		if serviceName, ok := ingress.GetLabels()[types.ZadigReleaseServiceNameLabelKey]; ok {
			if serviceAndErrorMap[serviceName] == nil {
				serviceAndErrorMap[serviceName] = []string{}
			}
			if err != nil {
				serviceAndErrorMap[serviceName] = append(serviceAndErrorMap[serviceName], fmt.Sprintf("delete ingress %s error: %v", ingress.Name, err))
				c.Error(fmt.Sprintf("delete ingress %s error: %v", ingress.Name, err))
				continue
			}
			c.Info(fmt.Sprintf("delete ingress %s success", ingress.Name))
		}
	}
	for _, ingress := range ingressNetworkingList {
		err := c.kubeClient.Delete(context.Background(), ingress)
		if serviceName, ok := ingress.GetLabels()[types.ZadigReleaseServiceNameLabelKey]; ok {
			if serviceAndErrorMap[serviceName] == nil {
				serviceAndErrorMap[serviceName] = []string{}
			}
			if err != nil {
				serviceAndErrorMap[serviceName] = append(serviceAndErrorMap[serviceName], fmt.Sprintf("delete ingress %s error: %v", ingress.Name, err))
				c.Error(fmt.Sprintf("delete ingress %s error: %v", ingress.Name, err))
				continue
			}
			c.Info(fmt.Sprintf("delete ingress %s success", ingress.Name))
		}
	}
	fail := false
	for service, errors := range serviceAndErrorMap {
		if len(errors) == 0 {
			c.jobTaskSpec.OfflineServices = append(c.jobTaskSpec.OfflineServices, &commonmodels.MseGrayOfflineService{
				ServiceName: service,
				Status:      config.StatusPassed,
			})
		} else {
			c.jobTaskSpec.OfflineServices = append(c.jobTaskSpec.OfflineServices, &commonmodels.MseGrayOfflineService{
				ServiceName: service,
				Status:      config.StatusFailed,
				Error:       strings.Join(errors, "\n"),
			})
			fail = true
		}
	}
	if fail {
		c.job.Status = config.StatusFailed
	} else {
		c.job.Status = config.StatusPassed
	}
	return
}

func (c *MseGrayOfflineJobCtl) Info(msg string) {
	c.jobTaskSpec.Events = append(c.jobTaskSpec.Events, &commonmodels.Event{
		EventType: "info",
		Message:   msg,
		Time:      time.Now().Format("2006-01-02 15:04:05"),
	})
	c.ack()
}

func (c *MseGrayOfflineJobCtl) Error(msg string) {
	c.jobTaskSpec.Events = append(c.jobTaskSpec.Events, &commonmodels.Event{
		EventType: "error",
		Message:   msg,
		Time:      time.Now().Format("2006-01-02 15:04:05"),
	})
	logError(c.job, msg, c.logger)
}

func (c *MseGrayOfflineJobCtl) SaveInfo(ctx context.Context) error {
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
