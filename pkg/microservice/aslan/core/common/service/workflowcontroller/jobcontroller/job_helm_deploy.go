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
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	crClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	helmservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/helm"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
	helmtool "github.com/koderover/zadig/v2/pkg/tool/helmclient"
	"github.com/koderover/zadig/v2/pkg/tool/kube/getter"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types/job"
	"github.com/koderover/zadig/v2/pkg/util"
)

type HelmDeployJobCtl struct {
	job         *commonmodels.JobTask
	namespace   string
	workflowCtx *commonmodels.WorkflowTaskCtx
	logger      *zap.SugaredLogger
	kubeClient  crClient.Client
	restConfig  *rest.Config
	jobTaskSpec *commonmodels.JobTaskHelmDeploySpec
	ack         func()
}

func NewHelmDeployJobCtl(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, ack func(), logger *zap.SugaredLogger) *HelmDeployJobCtl {
	jobTaskSpec := &commonmodels.JobTaskHelmDeploySpec{}
	if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
		logger.Error(err)
	}
	job.Spec = jobTaskSpec
	return &HelmDeployJobCtl{
		job:         job,
		workflowCtx: workflowCtx,
		logger:      logger,
		ack:         ack,
		jobTaskSpec: jobTaskSpec,
	}
}

func (c *HelmDeployJobCtl) Clean(ctx context.Context) {}

func (c *HelmDeployJobCtl) Run(ctx context.Context) {
	c.job.Status = config.StatusRunning
	c.ack()

	// set IMAGE job output
	for _, svc := range c.jobTaskSpec.ImageAndModules {
		// helm deploy job key is jobName.serviceName
		c.workflowCtx.GlobalContextSet(job.GetJobOutputKey(c.job.Key+"."+svc.ServiceModule, IMAGEKEY), svc.Image)
	}

	productInfo, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    c.workflowCtx.ProjectName,
		EnvName: c.jobTaskSpec.Env,
	})
	if err != nil {
		msg := fmt.Sprintf("find project %s error: %v", c.workflowCtx.ProjectName, err)
		logError(c.job, msg, c.logger)
		return
	}
	if productInfo.IsSleeping() {
		msg := fmt.Sprintf("Environment %s/%s is sleeping", productInfo.ProductName, productInfo.EnvName)
		logError(c.job, msg, c.logger)
		return
	}

	c.namespace = productInfo.Namespace
	c.jobTaskSpec.ClusterID = productInfo.ClusterID

	updateServiceRevision := false
	if slices.Contains(c.jobTaskSpec.DeployContents, config.DeployConfig) && c.jobTaskSpec.UpdateConfig {
		updateServiceRevision = true
	}

	helmDeploySvc := helmservice.NewHelmDeployService()
	newEnvService, tmplSvc, err := helmDeploySvc.GenNewEnvService(productInfo, c.jobTaskSpec.ServiceName, updateServiceRevision)
	if err != nil {
		msg := fmt.Sprintf("failed to generate new env service, error: %v", err)
		logError(c.job, msg, c.logger)
		return
	}
	newEnvService.DeployStrategy = setting.ServiceDeployStrategyDeploy

	finalValuesYaml := ""
	if len(c.jobTaskSpec.DeployContents) == 1 && slices.Contains(c.jobTaskSpec.DeployContents, config.DeployImage) {
		finalValuesYaml, err = helmDeploySvc.GenMergedValues(newEnvService, productInfo.DefaultValues, c.jobTaskSpec.GetDeployImages())
		if err != nil {
			msg := fmt.Sprintf("failed to generate merged values yaml, err: %s", err)
			logError(c.job, msg, c.logger)
			return
		}
	} else if len(c.jobTaskSpec.DeployContents) == 1 && slices.Contains(c.jobTaskSpec.DeployContents, config.DeployConfig) {
		// only deploy config
		finalValuesYaml, err = helmDeploySvc.GenMergedValues(newEnvService, productInfo.DefaultValues, nil)
		if err != nil {
			msg := fmt.Sprintf("failed to generate merged values yaml, err: %s", err)
			logError(c.job, msg, c.logger)
			return
		}
	} else {
		images := []string{}
		if c.jobTaskSpec.Source == config.SourceFromJob {
			images = c.jobTaskSpec.GetDeployImages()
		}

		newEnvService.GetServiceRender().SetOverrideYaml(c.jobTaskSpec.VariableYaml)
		finalValuesYaml, err = helmDeploySvc.GenMergedValues(newEnvService, productInfo.DefaultValues, images)
		if err != nil {
			msg := fmt.Sprintf("failed to generate merged values yaml, err: %s", err)
			logError(c.job, msg, c.logger)
			return
		}
	}

	c.jobTaskSpec.YamlContent = finalValuesYaml
	c.jobTaskSpec.UserSuppliedValue = newEnvService.GetServiceRender().GetOverrideYaml()

	latestRevision, err := commonrepo.NewEnvServiceVersionColl().GetLatestRevision(productInfo.ProductName, productInfo.EnvName, c.jobTaskSpec.ServiceName, false, productInfo.Production)
	if err != nil {
		msg := fmt.Sprintf("get service revision error: %v", err)
		logError(c.job, msg, c.logger)
		return
	}

	c.jobTaskSpec.OriginRevision = latestRevision

	c.ack()

	c.logger.Infof("start helm deploy, productName %s serviceName %s namespace %s, values %s, overrideKVs: %s updateServiceRevision %v, revision %d",
		c.workflowCtx.ProjectName, c.jobTaskSpec.ServiceName, c.namespace, finalValuesYaml, newEnvService.GetServiceRender().OverrideValues, updateServiceRevision, newEnvService.Revision)

	timeOut := c.timeout()

	done := make(chan bool)
	util.Go(func() {
		if err = kube.DeploySingleHelmRelease(productInfo, newEnvService, tmplSvc, nil, c.jobTaskSpec.Timeout, c.workflowCtx.WorkflowTaskCreatorUsername); err != nil {
			err = errors.WithMessagef(err,
				"failed to upgrade helm chart %s/%s",
				c.namespace, c.jobTaskSpec.ServiceName)
			done <- false
		} else {
			done <- true
		}
	})

	timeout := time.After(time.Second*time.Duration(timeOut) + time.Minute)
	if !c.jobTaskSpec.SkipCheckRunStatus {
		// we add timeout check here in case helm stuck in pending status
		select {
		case result := <-done:
			if !result {
				logError(c.job, err.Error(), c.logger)
				return
			}

			if !c.jobTaskSpec.SkipCheckHelmWorkfloadStatus {
				c.job.Status, err = c.checkWorkloadStatus(ctx, productInfo, newEnvService, tmplSvc, timeout)
				if err != nil {
					logError(c.job, err.Error(), c.logger)
					return
				}
			}
			break
		case <-timeout:
			logError(c.job, fmt.Sprintf("failed to upgrade helm chart %s/%s, timeout", c.namespace, c.jobTaskSpec.ServiceName), c.logger)
			c.job.Status = config.StatusTimeout
			return
		}
		if err != nil {
			logError(c.job, err.Error(), c.logger)
			return
		}
	}

	c.job.Status = config.StatusPassed
}

type DeployResource struct {
	PodOwnerUID      types.UID
	Unstructured     *unstructured.Unstructured
	RelatedPodLabels []map[string]string
}

func (c *HelmDeployJobCtl) checkWorkloadStatus(ctx context.Context, productInfo *commonmodels.Product, newEnvService *commonmodels.ProductService, tmplSvc *commonmodels.Service, timeout <-chan time.Time) (config.Status, error) {
	var err error
	releaseName := newEnvService.ReleaseName
	if newEnvService.FromZadig() {
		releaseName = util.GeneReleaseName(tmplSvc.GetReleaseNaming(), tmplSvc.ProductName, productInfo.Namespace, productInfo.EnvName, tmplSvc.ServiceName)
	}

	c.kubeClient, err = clientmanager.NewKubeClientManager().GetControllerRuntimeClient(c.jobTaskSpec.ClusterID)
	if err != nil {
		return config.StatusFailed, fmt.Errorf("can't init k8s client: %v", err)
	}

	helmClient, err := helmtool.NewClientFromNamespace(productInfo.ClusterID, productInfo.Namespace)
	if err != nil {
		return config.StatusFailed, err
	}

	release, err := helmClient.GetRelease(releaseName)
	if err != nil {
		return config.StatusFailed, fmt.Errorf("failed to get release %s, err: %v", releaseName, err)
	}

	unstructuredList, _, err := kube.ManifestToUnstructured(release.Manifest)
	if err != nil {
		return config.StatusFailed, fmt.Errorf("failed to convert manifest to unstructured, err: %v", err)
	}

	// resources, err := GetResourcesPodOwnerUID(c.kubeClient, c.namespace, c.jobTaskSpec.ServiceAndImages, c.jobTaskSpec.DeployContents, c.jobTaskSpec.ReplaceResources)
	// if err != nil {
	// 	return fmt.Errorf("failed to get resources pod owner uid, err: %v", err)
	// }

	relatedPodLabels := make([]map[string]string, 0)
	resources := []commonmodels.Resource{}

	for _, u := range unstructuredList {
		switch u.GetKind() {
		case setting.Deployment, setting.StatefulSet:
			resources = append(resources, commonmodels.Resource{
				Kind: u.GetKind(),
				// Container: container.Name,
				// Origin:    container.Image,
				Name: u.GetName(),
			})
			relatedPodLabels = append(relatedPodLabels, u.GetLabels())
		}
	}

	resources, err = GetResourcesPodOwnerUID(c.kubeClient, c.namespace, nil, c.jobTaskSpec.DeployContents, resources)
	if err != nil {
		return config.StatusFailed, fmt.Errorf("failed to get resources pod owner uid, err: %v", err)
	}

	// deployResources, err := c.getDeployResources(unstructuredList)
	// if err != nil {
	// 	return config.StatusFailed, fmt.Errorf("failed to get deploy resources, err: %v", err)
	// }

	// replaceResources := []commonmodels.Resource{}
	// for _, resource := range deployResources {
	// 	relatedPodLabels = append(relatedPodLabels, resource.RelatedPodLabels...)
	// 	resource := commonmodels.Resource{
	// 		Kind:        resource.Unstructured.GetKind(),
	// 		Name:        resource.Unstructured.GetName(),
	// 		PodOwnerUID: string(resource.PodOwnerUID),
	// 	}
	// 	replaceResources = append(replaceResources, resource)
	// }

	status, err := CheckDeployStatus(ctx, c.kubeClient, c.namespace, relatedPodLabels, resources, timeout, c.logger)
	if err != nil {
		return status, fmt.Errorf("failed to check workload status, err: %v", err)
	}
	return config.StatusPassed, nil

	// ready := true
	// for {
	// 	select {
	// 	case <-ctx.Done():
	// 		return config.StatusFailed, fmt.Errorf("failed to check workload status for service: %s, context done", c.jobTaskSpec.ServiceName)
	// 	case <-timeout:
	// 		var msg []string
	// 		for _, resource := range deployResources {
	// 			for _, label := range resource.RelatedPodLabels {
	// 				selector := labels.Set(label).AsSelector()
	// 				pods, err := getter.ListPods(c.namespace, selector, c.kubeClient)
	// 				if err != nil {
	// 					msg := fmt.Sprintf("list pods error: %v", err)
	// 					return config.StatusFailed, errors.New(msg)
	// 				}
	// 				for _, pod := range pods {
	// 					podResource := wrapper.Pod(pod).Resource()
	// 					if podResource.Status != setting.StatusRunning && podResource.Status != setting.StatusSucceeded {
	// 						for _, cs := range podResource.Containers {
	// 							// message为空不认为是错误状态，有可能还在waiting
	// 							if cs.Message != "" {
	// 								msg = append(msg, fmt.Sprintf("Status: %s, Reason: %s, Message: %s", cs.Status, cs.Reason, cs.Message))
	// 							}
	// 						}
	// 					}
	// 				}
	// 			}
	// 		}

	// 		if len(msg) != 0 {
	// 			err := errors.New(strings.Join(msg, "\n"))
	// 			return config.StatusFailed, err
	// 		}
	// 		return config.StatusTimeout, nil
	// 	default:
	// 	L:
	// 		for _, resource := range deployResources {
	// 			err = workLoadDeployStat(c.kubeClient, c.namespace, resource.RelatedPodLabels, string(resource.PodOwnerUID))
	// 			if err != nil {
	// 				return config.StatusFailed, fmt.Errorf("failed to get workload deploy status, err: %v", err)
	// 			}

	// 			switch resource.Unstructured.GetKind() {
	// 			case setting.StatefulSet:
	// 				sts, found, e := getter.GetStatefulSet(c.namespace, resource.Unstructured.GetName(), c.kubeClient)
	// 				if e != nil {
	// 					err = e
	// 				}
	// 				if err != nil || !found {
	// 					c.logger.Errorf(
	// 						"failed to check statefulset ready status %s/%s/%s - %v",
	// 						c.namespace,
	// 						resource.Unstructured.GetKind(),
	// 						resource.Unstructured.GetName(),
	// 						e,
	// 					)
	// 					ready = false
	// 				} else {
	// 					ready = wrapper.StatefulSet(sts).Ready()
	// 				}

	// 				if !ready {
	// 					break L
	// 				}
	// 			case setting.Deployment:
	// 				deployment, found, e := getter.GetDeployment(c.namespace, resource.Unstructured.GetName(), c.kubeClient)
	// 				if e != nil {
	// 					err = e
	// 				}
	// 				if err != nil || !found {
	// 					c.logger.Errorf(
	// 						"failed to check deployment ready status %s/%s/%s - %v",
	// 						c.namespace,
	// 						resource.Unstructured.GetKind(),
	// 						resource.Unstructured.GetName(),
	// 						e,
	// 					)
	// 					ready = false
	// 				} else {
	// 					ready = wrapper.Deployment(deployment).Ready()
	// 				}

	// 				if !ready {
	// 					break L
	// 				}
	// 			}
	// 		}

	// 		if ready {
	// 			return config.StatusPassed, nil
	// 		}
	// 	}
	// }
}

func (c *HelmDeployJobCtl) getDeployResources(resources []*unstructured.Unstructured) ([]*DeployResource, error) {
	// timeout := time.After(time.Second * 20)

	// var newResources []commonmodels.Resource
	// var err error

	deployResources := []*DeployResource{}
	for _, u := range resources {
		switch u.GetKind() {
		case setting.StatefulSet:
			sts, found, err := getter.GetStatefulSet(c.namespace, u.GetName(), c.kubeClient)
			if err != nil || !found {
				return nil, fmt.Errorf("failed to get statefulset %s, err: %v", u.GetName(), err)
			}

			podOwnerUID := sts.GetObjectMeta().GetUID()
			deployResource := &DeployResource{
				PodOwnerUID:      podOwnerUID,
				RelatedPodLabels: []map[string]string{sts.Spec.Template.Labels},
				Unstructured:     u,
			}
			deployResources = append(deployResources, deployResource)
		case setting.Deployment:
			log.Debugf("namespace: %s", c.namespace)
			log.Debugf("name: %s", u.GetName())
			deployment, _, err := getter.GetDeployment(c.namespace, u.GetName(), c.kubeClient)
			if err != nil {
				return nil, fmt.Errorf("failed to get deployment %s, err: %v", u.GetName(), err)
			}
			log.Debugf("deployment: %+v", deployment)

			selector, err := metav1.LabelSelectorAsSelector(deployment.Spec.Selector)
			if err != nil {
				return nil, fmt.Errorf("failed to get selector from deployment %s, err: %v", deployment.Name, err)
			}
			// ensure latest replicaset to be created
			replicaSets, err := getter.ListReplicaSets(c.namespace, selector, c.kubeClient)
			if err != nil {
				return nil, fmt.Errorf("failed to list replicaset for deployment %s, err: %v", deployment.Name, err)
			}
			// Only include those whose ControllerRef matches the Deployment.
			owned := make([]*appsv1.ReplicaSet, 0, len(replicaSets))
			for _, rs := range replicaSets {
				if metav1.IsControlledBy(rs, deployment) {
					owned = append(owned, rs)
				}
			}
			if len(owned) <= 0 {
				return nil, fmt.Errorf("no replicaset found for deployment: %s", deployment.Name)
			}

			podOwnerUID := owned[0].ObjectMeta.UID
			deployResource := &DeployResource{
				PodOwnerUID:      podOwnerUID,
				RelatedPodLabels: []map[string]string{deployment.Spec.Template.Labels},
				Unstructured:     u,
			}
			deployResources = append(deployResources, deployResource)
		}
	}
	return deployResources, nil
}

func (c *HelmDeployJobCtl) timeout() int {
	if c.jobTaskSpec.Timeout == 0 {
		c.jobTaskSpec.Timeout = setting.DeployTimeout
	}
	return c.jobTaskSpec.Timeout
}

func (c *HelmDeployJobCtl) SaveInfo(ctx context.Context) error {
	modules := make([]string, 0)
	for _, module := range c.jobTaskSpec.ImageAndModules {
		modules = append(modules, module.ServiceModule)
	}
	moduleList := strings.Join(modules, ",")
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

		ServiceType:   c.jobTaskSpec.ServiceType,
		ServiceName:   c.jobTaskSpec.ServiceName,
		TargetEnv:     c.jobTaskSpec.Env,
		ServiceModule: moduleList,
		Production:    c.jobTaskSpec.IsProduction,
	})
}

func (c *HelmDeployJobCtl) getVarsYaml() (string, error) {
	vars := []*commonmodels.VariableKV{}
	for _, v := range c.jobTaskSpec.KeyVals {
		vars = append(vars, &commonmodels.VariableKV{Key: v.Key, Value: v.Value})
	}
	return kube.GenerateYamlFromKV(vars)
}
