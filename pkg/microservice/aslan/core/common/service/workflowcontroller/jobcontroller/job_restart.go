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
	"sort"
	"time"

	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
	helmtool "github.com/koderover/zadig/v2/pkg/tool/helmclient"
	"github.com/koderover/zadig/v2/pkg/util"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	versionedclient "istio.io/client-go/pkg/clientset/versioned"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/repository"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/kube/getter"
	"github.com/koderover/zadig/v2/pkg/tool/kube/updater"
)

type RestartJobCtl struct {
	job         *commonmodels.JobTask
	namespace   string
	workflowCtx *commonmodels.WorkflowTaskCtx
	logger      *zap.SugaredLogger
	kubeClient  crClient.Client
	informer    informers.SharedInformerFactory
	clientSet   *kubernetes.Clientset
	istioClient *versionedclient.Clientset
	jobTaskSpec *commonmodels.JobTaskRestartSpec
	ack         func()
}

func NewRestartJobCtl(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, ack func(), logger *zap.SugaredLogger) *RestartJobCtl {
	jobTaskSpec := &commonmodels.JobTaskRestartSpec{}
	if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
		logger.Error(err)
	}
	job.Spec = jobTaskSpec
	return &RestartJobCtl{
		job:         job,
		workflowCtx: workflowCtx,
		logger:      logger,
		ack:         ack,
		jobTaskSpec: jobTaskSpec,
	}
}

func (c *RestartJobCtl) Clean(ctx context.Context) {}

func (c *RestartJobCtl) Run(ctx context.Context) {
	c.job.Status = config.StatusRunning
	c.ack()
	c.preRun()
	if err := c.run(ctx); err != nil {
		return
	}
	if c.jobTaskSpec.SkipCheckRunStatus {
		c.job.Status = config.StatusPassed
		return
	}
	c.wait(ctx)
}

func (c *RestartJobCtl) preRun() {
}

func (c *RestartJobCtl) run(ctx context.Context) error {
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
	if env.IsSleeping() {
		msg := fmt.Sprintf("Environment %s/%s is sleeping", env.ProductName, env.EnvName)
		logError(c.job, msg, c.logger)
		return errors.New(msg)
	}

	c.namespace = env.Namespace
	c.jobTaskSpec.ClusterID = env.ClusterID

	c.kubeClient, err = clientmanager.NewKubeClientManager().GetControllerRuntimeClient(c.jobTaskSpec.ClusterID)
	if err != nil {
		msg := fmt.Sprintf("can't init k8s client: %v", err)
		logError(c.job, msg, c.logger)
		return errors.New(msg)
	}
	c.clientSet, err = clientmanager.NewKubeClientManager().GetKubernetesClientSet(c.jobTaskSpec.ClusterID)
	if err != nil {
		msg := fmt.Sprintf("can't init k8s clientset: %v", err)
		logError(c.job, msg, c.logger)
		return errors.New(msg)
	}

	c.informer, err = clientmanager.NewKubeClientManager().GetInformer(c.jobTaskSpec.ClusterID, c.namespace)
	if err != nil {
		msg := fmt.Sprintf("can't init k8s informer: %v", err)
		logError(c.job, msg, c.logger)
		return errors.New(msg)
	}

	c.istioClient, err = clientmanager.NewKubeClientManager().GetIstioClientSet(c.jobTaskSpec.ClusterID)
	if err != nil {
		msg := fmt.Sprintf("can't init k8s istio client: %v", err)
		logError(c.job, msg, c.logger)
		return errors.New(msg)
	}

	if c.jobTaskSpec.DeployType == setting.K8SDeployType {
		err = c.restartK8sService(ctx, env, c.jobTaskSpec.ServiceName)
		if err != nil {
			logError(c.job, err.Error(), c.logger)
			return err
		}
	} else if c.jobTaskSpec.DeployType == setting.HelmDeployType {
		err = c.restartHelmService(ctx, env, c.jobTaskSpec.ServiceName)
		if err != nil {
			logError(c.job, err.Error(), c.logger)
			return err
		}
	} else {
		return fmt.Errorf("unsupported service type: %s", c.jobTaskSpec.DeployType)
	}

	c.ack()

	return nil
}

func (c *RestartJobCtl) restartK8sService(ctx context.Context, env *commonmodels.Product, serviceName string) error {
	envService := env.GetServiceMap()[serviceName]
	if envService == nil {
		return fmt.Errorf("service %s not found in env %s/%s", serviceName, env.ProductName, env.EnvName)
	}

	templateService, err := repository.QueryTemplateService(&commonrepo.ServiceFindOption{
		ProductName: env.ProductName,
		ServiceName: serviceName,
		Revision:    envService.Revision,
	}, env.Production)
	if err != nil {
		return fmt.Errorf("failed to query template service %s/%s: %v", env.ProductName, serviceName, err)
	}

	option := &kube.GeneSvcYamlOption{
		ProductName: env.ProductName,
		EnvName:     c.jobTaskSpec.Env,
		ServiceName: serviceName,
	}

	_, resources, err := kube.FetchImportedManifests(option, env, templateService, envService.GetServiceRender())
	if err != nil {
		return fmt.Errorf("failed to fetch imported manifests: %v", err)
	}

	replaceResources, relatedPodLabels, err := restartWorkloadResources(ctx, c.kubeClient, c.clientSet, resources, env)
	if err != nil {
		return fmt.Errorf("failed to restart workload resources: %v", err)
	}

	c.jobTaskSpec.ReplaceResources = append(c.jobTaskSpec.ReplaceResources, replaceResources...)
	c.jobTaskSpec.RelatedPodLabels = append(c.jobTaskSpec.RelatedPodLabels, relatedPodLabels...)

	return nil
}

func (c *RestartJobCtl) restartHelmService(ctx context.Context, env *commonmodels.Product, serviceName string) error {
	envService := env.GetServiceMap()[serviceName]
	if envService == nil {
		return fmt.Errorf("service %s not found in env %s/%s", serviceName, env.ProductName, env.EnvName)
	}

	templateService, err := repository.QueryTemplateService(&commonrepo.ServiceFindOption{
		ProductName: env.ProductName,
		ServiceName: serviceName,
		Revision:    envService.Revision,
	}, env.Production)
	if err != nil {
		return fmt.Errorf("failed to query template service %s/%s: %v", env.ProductName, serviceName, err)
	}

	releaseName := util.GeneReleaseName(templateService.GetReleaseNaming(), env.ProductName, env.Namespace, env.EnvName, serviceName)

	helmClient, err := helmtool.NewClientFromNamespace(env.ClusterID, env.Namespace)
	if err != nil {
		return fmt.Errorf("failed to create helm client: %v", err)
	}

	release, err := helmClient.GetRelease(releaseName)
	if err != nil {
		return fmt.Errorf("failed to get release %q in namespace %q: %s", releaseName, env.Namespace, err)
	}

	unstructuredList, _, err := kube.ManifestToUnstructured(release.Manifest)
	if err != nil {
		return fmt.Errorf("failed to convert manifest to unstructured: %v", err)
	}

	relatedPodLabels := make([]map[string]string, 0)
	resources := []*kube.WorkloadResource{}

	for _, u := range unstructuredList {
		switch u.GetKind() {
		case setting.Deployment, setting.StatefulSet:
			resources = append(resources, &kube.WorkloadResource{
				Type: u.GetKind(),
				Name: u.GetName(),
			})
			relatedPodLabels = append(relatedPodLabels, u.GetLabels())
		}
	}

	replaceResources, relatedPodLabels, err := restartWorkloadResources(ctx, c.kubeClient, c.clientSet, resources, env)
	if err != nil {
		return fmt.Errorf("failed to restart workload resources: %v", err)
	}

	c.jobTaskSpec.ReplaceResources = append(c.jobTaskSpec.ReplaceResources, replaceResources...)
	c.jobTaskSpec.RelatedPodLabels = append(c.jobTaskSpec.RelatedPodLabels, relatedPodLabels...)

	return nil
}

func restartWorkloadResources(ctx context.Context, kubeClient client.Client, clientSet *kubernetes.Clientset, resources []*kube.WorkloadResource, env *commonmodels.Product) (replaceResources []commonmodels.Resource, relatedPodLabels []map[string]string, err error) {
	deployments, statefulSets, _, _, _, err := kube.FetchSelectedWorkloads(env.Namespace, resources, kubeClient, clientSet)
	if err != nil {
		return nil, nil, err
	}

	for _, deployment := range deployments {
		err = updater.RestartDeployment(deployment.Namespace, deployment.Name, kubeClient)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to restart deployment %s/%s: %v", deployment.Namespace, deployment.Name, err)
		}

		selector, err := metav1.LabelSelectorAsSelector(deployment.Spec.Selector)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get selector for deployment %s/%s: %v", deployment.Namespace, deployment.Name, err)
		}

		// ensure latest replicaset to be created
		replicaSets, err := getter.ListReplicaSets(deployment.Namespace, selector, kubeClient)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to list replica sets for deployment %s/%s: %v", deployment.Namespace, deployment.Name, err)
		}

		// Only include those whose ControllerRef matches the Deployment.
		owned := make([]*appsv1.ReplicaSet, 0, len(replicaSets))
		for _, rs := range replicaSets {
			if metav1.IsControlledBy(rs, deployment) {
				owned = append(owned, rs)
			}
		}
		if len(owned) <= 0 {
			return nil, nil, fmt.Errorf("no replicaset found for deployment: %s", deployment.Name)
		}
		sort.Slice(owned, func(i, j int) bool {
			return owned[i].CreationTimestamp.After(owned[j].CreationTimestamp.Time)
		})

		replaceResources = append(replaceResources, commonmodels.Resource{
			Kind:        setting.Deployment,
			Name:        deployment.Name,
			PodOwnerUID: string(owned[0].ObjectMeta.UID),
		})
		relatedPodLabels = append(relatedPodLabels, deployment.Spec.Template.Labels)
	}

	for _, sts := range statefulSets {
		err = updater.RestartStatefulSet(sts.Namespace, sts.Name, kubeClient)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to restart statefulset %s/%s: %v", sts.Namespace, sts.Name, err)
		}

		replaceResources = append(replaceResources, commonmodels.Resource{
			Kind:        setting.StatefulSet,
			Name:        sts.Name,
			PodOwnerUID: string(sts.ObjectMeta.UID),
		})
		relatedPodLabels = append(relatedPodLabels, sts.Spec.Template.Labels)
	}

	return replaceResources, relatedPodLabels, nil
}

func (c *RestartJobCtl) wait(ctx context.Context) {
	timeout := time.After(time.Duration(c.timeout()) * time.Second)

	status, err := CheckDeployStatus(ctx, c.kubeClient, c.namespace, c.jobTaskSpec.RelatedPodLabels, c.jobTaskSpec.ReplaceResources, nil, timeout, c.logger)
	if err != nil {
		logError(c.job, err.Error(), c.logger)
		return
	}
	c.job.Status = status
}

func (c *RestartJobCtl) timeout() int {
	if c.jobTaskSpec.Timeout == 0 {
		c.jobTaskSpec.Timeout = setting.DeployTimeout
	}
	return c.jobTaskSpec.Timeout
}

func (c *RestartJobCtl) SaveInfo(ctx context.Context) error {
	return commonrepo.NewJobInfoColl().Create(context.TODO(), &commonmodels.JobInfo{
		Type:                c.job.JobType,
		WorkflowName:        c.workflowCtx.WorkflowName,
		WorkflowDisplayName: c.workflowCtx.WorkflowDisplayName,
		TaskID:              c.workflowCtx.TaskID,
		ProductName:         c.workflowCtx.ProjectName,
		StartTime:           c.job.StartTime,
		EndTime:             c.job.EndTime,
		Duration:            c.job.EndTime - c.job.StartTime,
		Status:              string(c.job.Status),

		ServiceType: c.jobTaskSpec.DeployType,
		ServiceName: c.jobTaskSpec.ServiceName,
		TargetEnv:   c.jobTaskSpec.Env,
		Production:  c.jobTaskSpec.Production,
	})
}
