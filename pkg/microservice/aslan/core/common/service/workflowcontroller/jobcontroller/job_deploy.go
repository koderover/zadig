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
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
	versionedclient "istio.io/client-go/pkg/clientset/versioned"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/kube"
	commontypes "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/types"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/kube/wrapper"
	"github.com/koderover/zadig/v2/pkg/tool/kube/getter"
	"github.com/koderover/zadig/v2/pkg/tool/kube/updater"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types/job"
)

type DeployJobCtl struct {
	job         *commonmodels.JobTask
	namespace   string
	workflowCtx *commonmodels.WorkflowTaskCtx
	logger      *zap.SugaredLogger
	kubeClient  crClient.Client
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

func (c *DeployJobCtl) preRun() {
	// set variables output
	for _, svc := range c.jobTaskSpec.ServiceAndImages {
		// deploy job key is jobName.serviceName
		c.workflowCtx.GlobalContextSet(job.GetJobOutputKey(c.job.Key+"."+svc.ServiceModule, IMAGEKEY), svc.Image)
		c.workflowCtx.GlobalContextSet(job.GetJobOutputKey(c.job.Key+"."+svc.ServiceModule, IMAGETAGKEY), util.ExtractImageTag(svc.Image))
	}
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

	// k8s projects
	//if c.jobTaskSpec.CreateEnvType == "system" {
	var updateRevision bool
	if slices.Contains(c.jobTaskSpec.DeployContents, config.DeployConfig) && c.jobTaskSpec.UpdateConfig {
		updateRevision = true
	}

	varsYaml := ""
	varKVs := []*commontypes.RenderVariableKV{}
	if slices.Contains(c.jobTaskSpec.DeployContents, config.DeployVars) {
		varsYaml, err = commontypes.RenderVariableKVToYaml(c.jobTaskSpec.VariableKVs, true)
		if err != nil {
			msg := fmt.Sprintf("generate vars yaml error: %v", err)
			logError(c.job, msg, c.logger)
			return errors.New(msg)
		}
		varKVs = c.jobTaskSpec.VariableKVs
	}
	containers := []*commonmodels.Container{}
	if slices.Contains(c.jobTaskSpec.DeployContents, config.DeployImage) {
		for _, serviceImage := range c.jobTaskSpec.ServiceAndImages {
			containers = append(containers, &commonmodels.Container{
				Name:      serviceImage.ServiceModule,
				Image:     serviceImage.Image,
				ImageName: util.ExtractImageName(serviceImage.Image),
			})
		}
	}

	option := &kube.GeneSvcYamlOption{
		ProductName:           env.ProductName,
		EnvName:               c.jobTaskSpec.Env,
		ServiceName:           c.jobTaskSpec.ServiceName,
		UpdateServiceRevision: updateRevision,
		VariableYaml:          varsYaml,
		VariableKVs:           varKVs,
		Containers:            containers,
	}
	updatedYaml, revision, resources, err := kube.GenerateRenderedYaml(option)
	if err != nil {
		msg := fmt.Sprintf("generate service yaml error: %v", err)
		logError(c.job, msg, c.logger)
		return errors.New(msg)
	}
	c.jobTaskSpec.YamlContent = updatedYaml
	c.ack()

	currentYaml, _, err := kube.FetchCurrentAppliedYaml(option)
	if err != nil {
		msg := fmt.Sprintf("get current service yaml error: %v", err)
		logError(c.job, msg, c.logger)
		return errors.New(msg)
	}

	latestRevision, err := commonrepo.NewEnvServiceVersionColl().GetLatestRevision(env.ProductName, env.EnvName, c.jobTaskSpec.ServiceName, false, env.Production)
	if err != nil {
		msg := fmt.Sprintf("get service revision error: %v", err)
		logError(c.job, msg, c.logger)
		return errors.New(msg)
	}

	c.jobTaskSpec.OriginRevision = latestRevision
	c.ack()

	// if not only deploy image, we will redeploy service
	if !onlyDeployImage(c.jobTaskSpec.DeployContents) {
		if err := c.updateSystemService(env, currentYaml, updatedYaml, c.jobTaskSpec.VariableKVs, revision, containers, updateRevision, c.jobTaskSpec.ServiceName); err != nil {
			logError(c.job, err.Error(), c.logger)
			return err
		}

		return nil
	}
	// if only deploy image, we only patch image.
	if err := c.updateServiceModuleImages(ctx, resources, env); err != nil {
		logError(c.job, err.Error(), c.logger)
		return err
	}

	return nil
}

func onlyDeployImage(deployContents []config.DeployContent) bool {
	return slices.Contains(deployContents, config.DeployImage) && len(deployContents) == 1
}

func (c *DeployJobCtl) updateSystemService(env *commonmodels.Product, currentYaml, updatedYaml string, variableKVs []*commontypes.RenderVariableKV, revision int,
	containers []*commonmodels.Container, updateRevision bool, serviceName string) error {
	addZadigLabel := !c.jobTaskSpec.Production
	if addZadigLabel {
		if !commonutil.ServiceDeployed(c.jobTaskSpec.ServiceName, env.ServiceDeployStrategy) && !updateRevision &&
			!slices.Contains(c.jobTaskSpec.DeployContents, config.DeployVars) {
			addZadigLabel = false
		}
	}

	err := kube.CheckResourceAppliedByOtherEnv(updatedYaml, env, serviceName)
	if err != nil {
		return errors.New(err.Error())
	}

	unstructuredList, err := kube.CreateOrPatchResource(&kube.ResourceApplyParam{
		ServiceName:         c.jobTaskSpec.ServiceName,
		CurrentResourceYaml: currentYaml,
		UpdateResourceYaml:  updatedYaml,
		Informer:            c.informer,
		KubeClient:          c.kubeClient,
		AddZadigLabel:       addZadigLabel,
		InjectSecrets:       true,
		SharedEnvHandler:    nil,
		ProductInfo:         env}, c.logger)

	if err != nil {
		msg := fmt.Sprintf("create or patch resource error: %v", err)
		return errors.New(msg)
	}

	variableYaml, err := commontypes.RenderVariableKVToYaml(variableKVs, true)
	if err != nil {
		msg := fmt.Sprintf("convert render variable to yaml error: %v", err)
		return errors.New(msg)
	}

	err = UpdateProductServiceDeployInfo(&ProductServiceDeployInfo{
		ProductName:           env.ProductName,
		EnvName:               c.jobTaskSpec.Env,
		ServiceName:           c.jobTaskSpec.ServiceName,
		ServiceRevision:       revision,
		VariableYaml:          variableYaml,
		VariableKVs:           variableKVs,
		Containers:            containers,
		UpdateServiceRevision: updateRevision,
		UserName:              c.workflowCtx.WorkflowTaskCreatorUsername,
		Resources:             unstructuredList,
	})
	if err != nil {
		msg := fmt.Sprintf("update service render set info error: %v", err)
		return errors.New(msg)
	}
	for _, us := range unstructuredList {
		switch us.GetKind() {
		case setting.Deployment, setting.StatefulSet:
			podLabels, _, err := unstructured.NestedStringMap(us.Object, "spec", "template", "metadata", "labels")
			if err == nil {
				c.jobTaskSpec.RelatedPodLabels = append(c.jobTaskSpec.RelatedPodLabels, podLabels)
			}
			c.jobTaskSpec.ReplaceResources = append(c.jobTaskSpec.ReplaceResources, commonmodels.Resource{Name: us.GetName(), Kind: us.GetKind()})
		case setting.CronJob, setting.Job:
			c.jobTaskSpec.ReplaceResources = append(c.jobTaskSpec.ReplaceResources, commonmodels.Resource{Name: us.GetName(), Kind: us.GetKind()})
		}
	}
	return nil
}

func UpdateExternalServiceModule(ctx context.Context, kubeClient client.Client, clientSet *kubernetes.Clientset, resources []*kube.WorkloadResource, env *commonmodels.Product, serviceName string, serviceModule *commonmodels.DeployServiceModule, detail, userName string, logger *zap.SugaredLogger) (replaceResources []commonmodels.Resource, relatedPodLabels []map[string]string, err error) {
	var replaced bool

	deployments, statefulSets, cronJobs, betaCronJobs, jobs, err := kube.FetchSelectedWorkloads(env.Namespace, resources, kubeClient, clientSet)
	if err != nil {
		return nil, nil, err
	}

L:
	for _, deploy := range deployments {
		for _, container := range deploy.Spec.Template.Spec.Containers {
			if container.Name == serviceModule.ServiceModule {
				// Check if Deployment is stuck before updating
				isStuck := kube.IsDeploymentStuckInUpdate(deploy, logger)

				err = updater.UpdateDeploymentImage(deploy.Namespace, deploy.Name, serviceModule.ServiceModule, serviceModule.Image, kubeClient)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to update container image in %s/deployments/%s/%s: %v", env.Namespace, deploy.Name, container.Name, err)
				}

				// If it was stuck, clean up stuck pods after the update
				if isStuck {
					logger.Infof("Deployment %s/%s was stuck, cleaning up stuck pods after image update", deploy.Namespace, deploy.Name)
					if fixErr := kube.HandleStuckDeployment(deploy, clientSet, logger); fixErr != nil {
						logger.Warnf("Failed to clean up stuck pods for Deployment %s/%s: %v", deploy.Namespace, deploy.Name, fixErr)
					}
				}

				replaceResources = append(replaceResources, commonmodels.Resource{
					Kind:      setting.Deployment,
					Container: container.Name,
					Origin:    container.Image,
					Name:      deploy.Name,
				})
				replaced = true
				relatedPodLabels = append(relatedPodLabels, deploy.Spec.Template.Labels)
				break L
			}
		}

		for _, container := range deploy.Spec.Template.Spec.InitContainers {
			if container.Name == serviceModule.ServiceModule {
				// Check if Deployment is stuck before updating
				isStuck := kube.IsDeploymentStuckInUpdate(deploy, logger)

				err = updater.UpdateDeploymentInitImage(deploy.Namespace, deploy.Name, serviceModule.ServiceModule, serviceModule.Image, kubeClient)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to update container image in %s/deployments/%s/%s: %v", env.Namespace, deploy.Name, container.Name, err)
				}

				// If it was stuck, clean up stuck pods after the update
				if isStuck {
					logger.Infof("Deployment %s/%s was stuck, cleaning up stuck pods after image update", deploy.Namespace, deploy.Name)
					if fixErr := kube.HandleStuckDeployment(deploy, clientSet, logger); fixErr != nil {
						logger.Warnf("Failed to clean up stuck pods for Deployment %s/%s: %v", deploy.Namespace, deploy.Name, fixErr)
					}
				}

				replaceResources = append(replaceResources, commonmodels.Resource{
					Kind:      setting.Deployment,
					Container: container.Name,
					Origin:    container.Image,
					Name:      deploy.Name,
				})
				replaced = true
				relatedPodLabels = append(relatedPodLabels, deploy.Spec.Template.Labels)
				break L
			}
		}
	}
Loop:
	for _, sts := range statefulSets {
		for _, container := range sts.Spec.Template.Spec.Containers {
			if container.Name == serviceModule.ServiceModule {
				// Check if StatefulSet is stuck before updating
				isStuck := kube.IsStatefulSetStuckInUpdate(sts, logger)

				err = updater.UpdateStatefulSetImage(sts.Namespace, sts.Name, serviceModule.ServiceModule, serviceModule.Image, kubeClient)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to update container image in %s/statefulsets/%s/%s: %v", env.Namespace, sts.Name, container.Name, err)
				}

				// If it was stuck, clean up stuck pods after the update
				if isStuck {
					logger.Infof("StatefulSet %s/%s was stuck, cleaning up stuck pods after image update", sts.Namespace, sts.Name)
					if fixErr := kube.HandleStuckStatefulSet(sts, clientSet, logger); fixErr != nil {
						logger.Warnf("Failed to clean up stuck pods for StatefulSet %s/%s: %v", sts.Namespace, sts.Name, fixErr)
					}
				}

				replaceResources = append(replaceResources, commonmodels.Resource{
					Kind:      setting.StatefulSet,
					Container: container.Name,
					Origin:    container.Image,
					Name:      sts.Name,
				})
				replaced = true
				relatedPodLabels = append(relatedPodLabels, sts.Spec.Template.Labels)
				break Loop
			}
		}
		for _, container := range sts.Spec.Template.Spec.InitContainers {
			if container.Name == serviceModule.ServiceModule {
				// Check if StatefulSet is stuck before updating
				isStuck := kube.IsStatefulSetStuckInUpdate(sts, logger)

				err = updater.UpdateStatefulSetInitImage(sts.Namespace, sts.Name, serviceModule.ServiceModule, serviceModule.Image, kubeClient)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to update container image in %s/statefulsets/%s/%s: %v", env.Namespace, sts.Name, container.Name, err)
				}

				// If it was stuck, clean up stuck pods after the update
				if isStuck {
					logger.Infof("StatefulSet %s/%s was stuck, cleaning up stuck pods after image update", sts.Namespace, sts.Name)
					if fixErr := kube.HandleStuckStatefulSet(sts, clientSet, logger); fixErr != nil {
						logger.Warnf("Failed to clean up stuck pods for StatefulSet %s/%s: %v", sts.Namespace, sts.Name, fixErr)
					}
				}

				replaceResources = append(replaceResources, commonmodels.Resource{
					Kind:      setting.StatefulSet,
					Container: container.Name,
					Origin:    container.Image,
					Name:      sts.Name,
				})
				replaced = true
				relatedPodLabels = append(relatedPodLabels, sts.Spec.Template.Labels)
				break Loop
			}
		}
	}
CronLoop:
	for _, cron := range cronJobs {
		for _, container := range cron.Spec.JobTemplate.Spec.Template.Spec.Containers {
			if container.Name == serviceModule.ServiceModule {
				err = updater.UpdateCronJobImage(cron.Namespace, cron.Name, serviceModule.ServiceModule, serviceModule.Image, kubeClient, false)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to update container image in %s/cronJob/%s/%s: %v", env.Namespace, cron.Name, container.Name, err)
				}
				replaceResources = append(replaceResources, commonmodels.Resource{
					Kind:      setting.CronJob,
					Container: container.Name,
					Origin:    container.Image,
					Name:      cron.Name,
				})
				replaced = true
				relatedPodLabels = append(relatedPodLabels, cron.Spec.JobTemplate.Spec.Template.Labels)
				break CronLoop
			}
		}
		for _, container := range cron.Spec.JobTemplate.Spec.Template.Spec.InitContainers {
			if container.Name == serviceModule.ServiceModule {
				err = updater.UpdateCronJobInitImage(cron.Namespace, cron.Name, serviceModule.ServiceModule, serviceModule.Image, kubeClient, false)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to update container image in %s/cronJob/%s/%s: %v", env.Namespace, cron.Name, container.Name, err)
				}
				replaceResources = append(replaceResources, commonmodels.Resource{
					Kind:      setting.CronJob,
					Container: container.Name,
					Origin:    container.Image,
					Name:      cron.Name,
				})
				replaced = true
				relatedPodLabels = append(relatedPodLabels, cron.Spec.JobTemplate.Spec.Template.Labels)
				break CronLoop
			}
		}
	}
BetaCronLoop:
	for _, cron := range betaCronJobs {
		for _, container := range cron.Spec.JobTemplate.Spec.Template.Spec.Containers {
			if container.Name == serviceModule.ServiceModule {
				err = updater.UpdateCronJobImage(cron.Namespace, cron.Name, serviceModule.ServiceModule, serviceModule.Image, kubeClient, true)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to update container image in %s/cronJobBeta/%s/%s: %v", env.Namespace, cron.Name, container.Name, err)
				}
				replaceResources = append(replaceResources, commonmodels.Resource{
					Kind:      setting.CronJob,
					Container: container.Name,
					Origin:    container.Image,
					Name:      cron.Name,
				})
				replaced = true
				relatedPodLabels = append(relatedPodLabels, cron.Spec.JobTemplate.Spec.Template.Labels)
				break BetaCronLoop
			}
		}
		for _, container := range cron.Spec.JobTemplate.Spec.Template.Spec.InitContainers {
			if container.Name == serviceModule.ServiceModule {
				err = updater.UpdateCronJobInitImage(cron.Namespace, cron.Name, serviceModule.ServiceModule, serviceModule.Image, kubeClient, true)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to update container image in %s/cronJobBeta/%s/%s: %v", env.Namespace, cron.Name, container.Name, err)
				}
				replaceResources = append(replaceResources, commonmodels.Resource{
					Kind:      setting.CronJob,
					Container: container.Name,
					Origin:    container.Image,
					Name:      cron.Name,
				})
				replaced = true
				relatedPodLabels = append(relatedPodLabels, cron.Spec.JobTemplate.Spec.Template.Labels)
				break BetaCronLoop
			}
		}
	}
Job:
	for _, job := range jobs {
		for _, container := range job.Spec.Template.Spec.Containers {
			if container.Name == serviceModule.ServiceModule {
				return nil, nil, fmt.Errorf("job %s/%s/%s is not supported to update image", env.Namespace, job.Name, container.Name)
				// err = updater.UpdateJobImage(job.Namespace, job.Name, serviceModule.ServiceModule, serviceModule.Image, c.kubeClient)
				// if err != nil {
				// 	return fmt.Errorf("failed to update container image in %s/job/%s/%s: %v", env.Namespace, job.Name, container.Name, err)
				// }
				// c.jobTaskSpec.ReplaceResources = append(c.jobTaskSpec.ReplaceResources, commonmodels.Resource{
				// 	Kind:      setting.Job,
				// 	Container: container.Name,
				// 	Origin:    container.Image,
				// 	Name:      job.Name,
				// })
				// replaced = true
				// c.jobTaskSpec.RelatedPodLabels = append(c.jobTaskSpec.RelatedPodLabels, job.Spec.Template.Labels)
				break Job
			}
		}
	}

	if !replaced {
		return nil, nil, fmt.Errorf("service %s container name %s is not found in env %s", serviceName, serviceModule.ServiceModule, env.EnvName)
	}
	if err := commonutil.UpdateProductImage(env.EnvName, env.ProductName, serviceName, map[string]string{serviceModule.ServiceModule: serviceModule.Image}, detail, userName, logger); err != nil {
		return nil, nil, err
	}
	return replaceResources, relatedPodLabels, nil
}

func (c *DeployJobCtl) updateServiceModuleImages(ctx context.Context, resources []*kube.WorkloadResource, env *commonmodels.Product) error {
	errList := new(multierror.Error)
	wg := sync.WaitGroup{}
	for _, serviceModule := range c.jobTaskSpec.ServiceAndImages {
		wg.Add(1)
		go func(serviceModule *commonmodels.DeployServiceModule) {
			defer wg.Done()
			replaceResources, relatedPodLabels, err := UpdateExternalServiceModule(ctx, c.kubeClient, c.clientSet, resources, env, c.jobTaskSpec.ServiceName, serviceModule, "", c.workflowCtx.WorkflowTaskCreatorUsername, c.logger)
			if err != nil {
				errList = multierror.Append(errList, err)
			} else {
				c.jobTaskSpec.ReplaceResources = append(c.jobTaskSpec.ReplaceResources, replaceResources...)
				c.jobTaskSpec.RelatedPodLabels = append(c.jobTaskSpec.RelatedPodLabels, relatedPodLabels...)
			}
		}(serviceModule)
	}
	wg.Wait()
	if err := errList.ErrorOrNil(); err != nil {
		return err
	}
	return nil
}

// 5.26 temporarily deactivate this function
// Because these errors must exist for a short period of time in some cases
func workLoadDeployStat(kubeClient client.Client, namespace string, labelMaps []map[string]string, ownerUID string) error {
	for _, label := range labelMaps {
		selector := labels.Set(label).AsSelector()
		pods, err := getter.ListPods(namespace, selector, kubeClient)
		if err != nil {
			return err
		}
		for _, pod := range pods {
			if !wrapper.Pod(pod).IsOwnerMatched(ownerUID) {
				continue
			}
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

func getResourcesPodOwnerUIDImpl(kubeClient client.Client, namespace string, serviceAndImages []*commonmodels.DeployServiceModule, deployContents []config.DeployContent, replaceResources []commonmodels.Resource, strict bool) ([]commonmodels.Resource, error) {
	containerMap := make(map[string]*commonmodels.Container)
	if strict && slices.Contains(deployContents, config.DeployImage) {
		for _, serviceImage := range serviceAndImages {
			containerMap[serviceImage.ServiceModule] = &commonmodels.Container{
				Name:      serviceImage.ServiceModule,
				Image:     serviceImage.Image,
				ImageName: util.ExtractImageName(serviceImage.Image),
			}
		}
	}
	newResources := []commonmodels.Resource{}
	for _, resource := range replaceResources {
		switch resource.Kind {
		case setting.StatefulSet:
			sts, _, err := getter.GetStatefulSet(namespace, resource.Name, kubeClient)
			if err != nil {
				return newResources, err
			}
			resource.PodOwnerUID = string(sts.ObjectMeta.UID)
		case setting.Deployment:
			deployment, _, err := getter.GetDeployment(namespace, resource.Name, kubeClient)
			if err != nil {
				return newResources, err
			}
			selector, err := metav1.LabelSelectorAsSelector(deployment.Spec.Selector)
			if err != nil {
				return nil, err
			}
			// ensure latest replicaset to be created
			replicaSets, err := getter.ListReplicaSets(namespace, selector, kubeClient)
			if err != nil {
				return newResources, err
			}
			// Only include those whose ControllerRef matches the Deployment.
			owned := make([]*appsv1.ReplicaSet, 0, len(replicaSets))
			for _, rs := range replicaSets {
				if metav1.IsControlledBy(rs, deployment) {
					owned = append(owned, rs)
				}
			}
			if len(owned) <= 0 {
				return newResources, fmt.Errorf("no replicaset found for deployment: %s", deployment.Name)
			}
			sort.Slice(owned, func(i, j int) bool {
				return owned[i].CreationTimestamp.After(owned[j].CreationTimestamp.Time)
			})

			if len(containerMap) > 0 {
				owner := owned[0]
				for _, image := range owner.Spec.Template.Spec.Containers {
					if replicaContainer, ok := containerMap[image.Name]; ok {
						if replicaContainer.Image != image.Image {
							log.Infof("meeting different container: %s/%s", replicaContainer.Name, replicaContainer.Image)
							return nil, nil
						}
					}
				}
			}

			resource.PodOwnerUID = string(owned[0].ObjectMeta.UID)
		}
		newResources = append(newResources, resource)
	}
	return newResources, nil
}

func GetResourcesPodOwnerUID(kubeClient client.Client, namespace string, serviceAndImages []*commonmodels.DeployServiceModule, deployContents []config.DeployContent, replaceResources []commonmodels.Resource) ([]commonmodels.Resource, error) {
	timeout := time.After(time.Second * 20)

	var newResources []commonmodels.Resource
	var err error

	for {
		if len(newResources) > 0 || err != nil {
			break
		}
		select {
		case <-timeout:
			newResources, err = getResourcesPodOwnerUIDImpl(kubeClient, namespace, serviceAndImages, deployContents, replaceResources, false)
			break
		default:
			time.Sleep(2 * time.Second)
			newResources, err = getResourcesPodOwnerUIDImpl(kubeClient, namespace, serviceAndImages, deployContents, replaceResources, true)
			break
		}
	}
	return newResources, nil
}

func (c *DeployJobCtl) wait(ctx context.Context) {
	timeout := time.After(time.Duration(c.timeout()) * time.Second)
	resources, err := GetResourcesPodOwnerUID(c.kubeClient, c.namespace, c.jobTaskSpec.ServiceAndImages, c.jobTaskSpec.DeployContents, c.jobTaskSpec.ReplaceResources)
	if err != nil {
		msg := fmt.Sprintf("get resource owner info error: %v", err)
		logError(c.job, msg, c.logger)
		return
	}

	c.jobTaskSpec.ReplaceResources = resources
	status, err := CheckDeployStatus(ctx, c.kubeClient, c.namespace, c.jobTaskSpec.RelatedPodLabels, c.jobTaskSpec.ReplaceResources, timeout, c.logger)
	if err != nil {
		logError(c.job, err.Error(), c.logger)
		return
	}
	c.job.Status = status
}

func CheckDeployStatus(ctx context.Context, kubeClient crClient.Client, namespace string, relatedPodLabels []map[string]string, replaceResources []commonmodels.Resource, timeout <-chan time.Time, logger *zap.SugaredLogger) (config.Status, error) {
	for {
		select {
		case <-ctx.Done():
			return config.StatusCancelled, nil
		case <-timeout:
			var msg []string
			for _, label := range relatedPodLabels {
				selector := labels.Set(label).AsSelector()
				pods, err := getter.ListPods(namespace, selector, kubeClient)
				if err != nil {
					msg := fmt.Sprintf("list pods error: %v", err)
					return config.StatusFailed, errors.New(msg)
				}
				for _, pod := range pods {
					podResource := wrapper.Pod(pod).Resource()
					if podResource.Status != setting.StatusRunning && podResource.Status != setting.StatusSucceeded {
						for _, cs := range podResource.Containers {
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
				return config.StatusFailed, err
			}
			return config.StatusTimeout, nil

		default:
			time.Sleep(time.Second * 2)
			ready := true
			var err error
		L:
			for _, resource := range replaceResources {
				if err := workLoadDeployStat(kubeClient, namespace, relatedPodLabels, resource.PodOwnerUID); err != nil {
					return config.StatusFailed, err
				}
				switch resource.Kind {
				case setting.Deployment:
					d, found, e := getter.GetDeployment(namespace, resource.Name, kubeClient)
					if e != nil {
						err = e
					}
					if e != nil || !found {
						logger.Errorf(
							"failed to check deployment ready status %s/%s/%s - %v",
							namespace,
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
					sts, found, e := getter.GetStatefulSet(namespace, resource.Name, kubeClient)
					if e != nil {
						err = e
					}
					if err != nil || !found {
						logger.Errorf(
							"failed to check statefulSet ready status %s/%s/%s - %v",
							namespace,
							resource.Kind,
							resource.Name,
							e,
						)
						ready = false
					} else {
						ready = sts.Status.ObservedGeneration >= sts.ObjectMeta.Generation &&
							sts.Status.UpdatedReplicas == *sts.Spec.Replicas &&
							sts.Status.ReadyReplicas == *sts.Spec.Replicas &&
							sts.Status.CurrentRevision == sts.Status.UpdateRevision
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

func (c *DeployJobCtl) timeout() int {
	if c.jobTaskSpec.Timeout == 0 {
		c.jobTaskSpec.Timeout = setting.DeployTimeout
	}
	return c.jobTaskSpec.Timeout
}

func (c *DeployJobCtl) SaveInfo(ctx context.Context) error {
	modules := make([]string, 0)
	for _, module := range c.jobTaskSpec.ServiceAndImages {
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
		ServiceModule: moduleList,
		TargetEnv:     c.jobTaskSpec.Env,
		Production:    c.jobTaskSpec.Production,
	})
}
