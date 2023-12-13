/*
Copyright 2021 The KodeRover Authors.

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

package taskplugin

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	helmclient "github.com/mittwald/go-helm-client"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	yaml "gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/releaseutil"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	configbase "github.com/koderover/zadig/v2/pkg/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models/template"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/microservice/warpdrive/config"
	"github.com/koderover/zadig/v2/pkg/microservice/warpdrive/core/service/taskplugin/s3"
	"github.com/koderover/zadig/v2/pkg/microservice/warpdrive/core/service/types"
	"github.com/koderover/zadig/v2/pkg/microservice/warpdrive/core/service/types/task"
	"github.com/koderover/zadig/v2/pkg/setting"
	kubeclient "github.com/koderover/zadig/v2/pkg/shared/kube/client"
	helmtool "github.com/koderover/zadig/v2/pkg/tool/helmclient"
	"github.com/koderover/zadig/v2/pkg/tool/httpclient"
	krkubeclient "github.com/koderover/zadig/v2/pkg/tool/kube/client"
	"github.com/koderover/zadig/v2/pkg/tool/kube/getter"
	"github.com/koderover/zadig/v2/pkg/tool/kube/label"
	"github.com/koderover/zadig/v2/pkg/tool/kube/updater"
	s3tool "github.com/koderover/zadig/v2/pkg/tool/s3"
	fsutil "github.com/koderover/zadig/v2/pkg/util/fs"
)

// InitializeReleaseImagePlugin ...
func InitializeReleaseImagePlugin(taskType config.TaskType) TaskPlugin {
	return &ReleaseImagePlugin{
		Name:       taskType,
		kubeClient: krkubeclient.Client(),
		clientset:  krkubeclient.Clientset(),
		restConfig: krkubeclient.RESTConfig(),
		httpClient: httpclient.New(
			httpclient.SetHostURL(configbase.AslanServiceAddress()),
		),
	}
}

// ReleaseImagePlugin Plugin name should be compatible with task type
type ReleaseImagePlugin struct {
	Name          config.TaskType
	HubServerAddr string
	KubeNamespace string
	JobName       string
	FileName      string
	kubeClient    client.Client
	clientset     kubernetes.Interface
	restConfig    *rest.Config
	Task          *task.ReleaseImage
	Log           *zap.SugaredLogger
	httpClient    *httpclient.Client
	StorageURI    string
	Timeout       <-chan time.Time
}

func (p *ReleaseImagePlugin) SetAckFunc(func()) {
}

const (
	// RelealseImageTaskTimeout ...
	RelealseImageTaskTimeout = 60 * 5 // 5 minutes
)

// Init ...
func (p *ReleaseImagePlugin) Init(jobname, filename string, xl *zap.SugaredLogger) {
	p.JobName = jobname
	p.FileName = filename
	// SetLogger ...
	p.Log = xl
}

// Type ...
func (p *ReleaseImagePlugin) Type() config.TaskType {
	return p.Name
}

// Status ...
func (p *ReleaseImagePlugin) Status() config.Status {
	return p.Task.TaskStatus
}

// SetStatus ...
func (p *ReleaseImagePlugin) SetStatus(status config.Status) {
	p.Task.TaskStatus = status
}

// TaskTimeout ...
func (p *ReleaseImagePlugin) TaskTimeout() int {
	if p.Task.Timeout == 0 {
		tm := config.ReleaseImageTimeout()
		if tm != "" {
			var err error
			p.Task.Timeout, err = strconv.Atoi(tm)
			if err != nil {
				p.Log.Warnf("failed to parse timeout settings %v", err)
				p.Task.Timeout = RelealseImageTaskTimeout
			}
		} else {
			p.Task.Timeout = RelealseImageTaskTimeout
		}
	}

	return p.Task.Timeout
}

// Run ...
func (p *ReleaseImagePlugin) Run(ctx context.Context, pipelineTask *task.Task, pipelineCtx *task.PipelineCtx, serviceName string) {
	p.KubeNamespace = pipelineTask.ConfigPayload.Build.KubeNamespace
	p.HubServerAddr = pipelineTask.ConfigPayload.HubServerAddr
	p.StorageURI = pipelineTask.StorageURI
	// 设置本次运行需要配置
	//t.Workspace = fmt.Sprintf("%s/%s", pipelineTask.ConfigPayload.NFS.Path, pipelineTask.PipelineName)
	releases := make([]task.RepoImage, 0)
	for _, v := range p.Task.Releases {
		if cfg, ok := pipelineTask.ConfigPayload.RepoConfigs[v.RepoID]; ok {
			v.Username = cfg.AccessKey
			v.Password = cfg.SecretKey
			releases = append(releases, v)
		}
	}

	distributes := make([]*task.DistributeInfo, 0)
	for _, distribute := range p.Task.DistributeInfo {
		if cfg, ok := pipelineTask.ConfigPayload.RepoConfigs[distribute.RepoID]; ok {
			distribute.RepoAK = cfg.AccessKey
			distribute.RepoSK = cfg.SecretKey
			distributes = append(distributes, distribute)
		}
	}

	if len(distributes) == 0 {
		fmt.Println("distribute is 0")
		return
	}

	jobCtx := &types.PredatorContext{
		JobType: setting.ReleaseImageJob,
		//Docker build context
		DockerBuildCtx: &task.DockerBuildCtx{
			ImageName:       p.Task.ImageTest,
			ImageReleaseTag: p.Task.ImageRelease,
		},
		//Registry host/user/password
		DockerRegistry: &types.DockerRegistry{
			Host:     pipelineTask.ConfigPayload.Registry.Addr,
			UserName: pipelineTask.ConfigPayload.Registry.AccessKey,
			Password: pipelineTask.ConfigPayload.Registry.SecretKey,
		},

		ReleaseImages:  releases,
		DistributeInfo: distributes,
	}

	jobCtxBytes, err := yaml.Marshal(jobCtx)
	if err != nil {
		msg := fmt.Sprintf("cannot mashal predetor.Context data: %v", err)
		p.Log.Error(msg)
		p.Task.TaskStatus = config.StatusFailed
		p.Task.Error = msg
		return
	}

	// 重置错误信息
	p.Task.Error = ""

	jobLabel := &label.JobLabel{
		PipelineName: pipelineTask.PipelineName,
		ServiceName:  serviceName,
		TaskID:       pipelineTask.TaskID,
		TaskType:     string(p.Type()),
		PipelineType: string(pipelineTask.Type),
	}

	if err := ensureDeleteConfigMap(p.KubeNamespace, jobLabel, p.kubeClient); err != nil {
		msg := fmt.Sprintf("ensureDeleteConfigMap error: %v", err)
		p.Log.Error(msg)
		p.Task.TaskStatus = config.StatusFailed
		p.Task.Error = msg
		return
	}

	if err := createJobConfigMap(
		p.KubeNamespace, p.JobName, jobLabel, string(jobCtxBytes), p.kubeClient); err != nil {
		msg := fmt.Sprintf("createJobConfigMap error: %v", err)
		p.Log.Error(msg)
		p.Task.TaskStatus = config.StatusFailed
		p.Task.Error = msg
		return
	}
	p.Log.Infof("succeed to create cm for image job %s", p.JobName)

	job, err := buildJob(p.Type(), pipelineTask.ConfigPayload.Release.PredatorImage, p.JobName, serviceName, "", "", pipelineTask.ConfigPayload.Build.KubeNamespace, setting.MinRequest, setting.MinRequestSpec, pipelineCtx, pipelineTask, []*task.RegistryNamespace{})
	if err != nil {
		msg := fmt.Sprintf("create release image job context error: %v", err)
		p.Log.Error(msg)
		p.Task.TaskStatus = config.StatusFailed
		p.Task.Error = msg
		return
	}

	if err := ensureDeleteJob(p.KubeNamespace, jobLabel, p.kubeClient); err != nil {
		msg := fmt.Sprintf("delete release image job error: %v", err)
		p.Log.Error(msg)
		p.Task.TaskStatus = config.StatusFailed
		p.Task.Error = msg
		return
	}

	job.Namespace = p.KubeNamespace
	startTime := time.Now().Unix()
	for _, distribute := range p.Task.DistributeInfo {
		distribute.DistributeStartTime = startTime
	}

	if err := updater.CreateJob(job, p.kubeClient); err != nil {
		msg := fmt.Sprintf("create release image job error: %v", err)
		p.Log.Error(msg)
		p.Task.TaskStatus = config.StatusFailed
		p.Task.Error = msg
		return
	}
	p.Timeout = time.After(time.Duration(p.TaskTimeout()) * time.Second)
	p.Log.Infof("succeed to create image job %s", p.JobName)
}

// Wait ...
func (p *ReleaseImagePlugin) Wait(ctx context.Context) {
	var err error
	var status config.Status
	defer func() {
		if err != nil {
			p.Log.Error(err)
			p.Task.TaskStatus = config.StatusFailed
			p.Task.Error = err.Error()
			return
		} else {
			p.SetStatus(config.StatusPassed)
		}
	}()
	status, err = waitJobEnd(ctx, p.TaskTimeout(), p.Timeout, p.KubeNamespace, p.JobName, p.kubeClient, p.clientset, p.restConfig, p.Log)
	distributeEndtime := time.Now().Unix()
	for _, distribute := range p.Task.DistributeInfo {
		distribute.DistributeEndTime = distributeEndtime
		distribute.DistributeStatus = string(status)
	}
	// if the distribution stage failed, then deploy part won't run
	if status != config.StatusPassed {
		err = errors.Errorf("failed to distribute images to the repository, err: %v", err)
		return
	}
	// otherwise, run any deploy subtasks
DistributeLoop:
	for _, distribute := range p.Task.DistributeInfo {
		if !distribute.DeployEnabled {
			continue
		}
		// set the start time on deployment start.
		distribute.DeployStartTime = time.Now().Unix()
		distribute.DeployStatus = "running"
		if distribute.DeployClusterID != "" {
			p.restConfig, err = kubeclient.GetRESTConfig(p.HubServerAddr, distribute.DeployClusterID)
			if err != nil {
				err = errors.WithMessage(err, "can't get k8s rest config")
				distribute.DeployStatus = string(config.StatusFailed)
				distribute.DeployEndTime = time.Now().Unix()
				continue
			}
			p.kubeClient, err = kubeclient.GetKubeClient(p.HubServerAddr, distribute.DeployClusterID)
			if err != nil {
				err = errors.WithMessage(err, "can't init k8s client")
				distribute.DeployStatus = string(config.StatusFailed)
				distribute.DeployEndTime = time.Now().Unix()
				continue
			}
			p.clientset, err = kubeclient.GetClientset(p.HubServerAddr, distribute.DeployClusterID)
			if err != nil {
				err = errors.WithMessage(err, "can't init k8s clientset")
				distribute.DeployStatus = string(config.StatusFailed)
				distribute.DeployEndTime = time.Now().Unix()
				continue
			}
		}

		versionInfo, err := p.clientset.Discovery().ServerVersion()
		if err != nil {
			err = errors.WithMessage(err, "can't get k8s version")
			distribute.DeployStatus = string(config.StatusFailed)
			distribute.DeployEndTime = time.Now().Unix()
			continue
		}
		// k8s deploy type service goes here
		if distribute.DeployServiceType != setting.HelmDeployType {
			replaced := false
			// get servcie info
			var (
				serviceInfo *types.ServiceTmpl
				selector    labels.Selector
			)
			serviceInfo, err = p.getService(ctx, distribute.DeployServiceName, distribute.DeployServiceType, p.Task.ProductName, 0)
			if err != nil {
				err = errors.WithMessage(err, "failed to get service info")
				distribute.DeployStatus = string(config.StatusFailed)
				distribute.DeployEndTime = time.Now().Unix()
			}
			if serviceInfo.WorkloadType == "" {

				var deployments []*appsv1.Deployment
				var statefulSets []*appsv1.StatefulSet
				var cronJobs []*batchv1.CronJob
				var cronJobBetas []*batchv1beta1.CronJob
				versionInfo, errGetVersion := p.clientset.Discovery().ServerVersion()
				if errGetVersion != nil {
					err = fmt.Errorf("failed to get kubernetes version: %v", errGetVersion)
					return
				}
				deployments, statefulSets, cronJobs, cronJobBetas, err = fetchRelatedWorkloads(ctx, distribute.DeployEnv, distribute.DeployNamespace, p.Task.ProductName,
					distribute.DeployServiceName, p.kubeClient, versionInfo, p.httpClient, p.Log)
				if err != nil {
					return
				}

				selector = labels.Set{setting.ProductLabel: p.Task.ProductName, setting.ServiceLabel: distribute.DeployServiceName}.AsSelector()

				deployments, err = getter.ListDeployments(distribute.DeployNamespace, selector, p.kubeClient)
				if err != nil {
					err = errors.WithMessage(err, "failed to get deployment")
					distribute.DeployStatus = string(config.StatusFailed)
					distribute.DeployEndTime = time.Now().Unix()
					continue
				}

				statefulSets, err = getter.ListStatefulSets(distribute.DeployNamespace, selector, p.kubeClient)
				if err != nil {
					err = errors.WithMessage(err, "failed to get statefulset")
					distribute.DeployStatus = string(config.StatusFailed)
					distribute.DeployEndTime = time.Now().Unix()
					continue
				}

			DeploymentLoop:
				for _, deploy := range deployments {
					for _, container := range deploy.Spec.Template.Spec.Containers {
						if container.Name == distribute.DeployContainerName {
							err = updater.UpdateDeploymentImage(deploy.Namespace, deploy.Name, distribute.DeployContainerName, distribute.Image, p.kubeClient)
							if err != nil {
								err = errors.WithMessagef(
									err,
									"failed to update container image in %s/deployments/%s/%s",
									distribute.DeployNamespace, deploy.Name, container.Name)
								distribute.DeployEndTime = time.Now().Unix()
								distribute.DeployStatus = string(config.StatusFailed)
								p.SetStatus(config.StatusFailed)
								continue DistributeLoop
							}
							replaced = true
							break DeploymentLoop
						}
					}
				}
			StatefulSetLoop:
				for _, sts := range statefulSets {
					for _, container := range sts.Spec.Template.Spec.Containers {
						if container.Name == distribute.DeployContainerName {
							err = updater.UpdateStatefulSetImage(sts.Namespace, sts.Name, distribute.DeployContainerName, distribute.Image, p.kubeClient)
							if err != nil {
								err = errors.WithMessagef(
									err,
									"failed to update container image in %s/statefulset/%s/%s",
									distribute.DeployNamespace, sts.Name, container.Name)
								distribute.DeployEndTime = time.Now().Unix()
								distribute.DeployStatus = string(config.StatusFailed)
								p.SetStatus(config.StatusFailed)
								continue DistributeLoop
							}
							replaced = true
							break StatefulSetLoop
						}
					}
				}
			CronLoop:
				for _, sts := range cronJobs {
					for _, container := range sts.Spec.JobTemplate.Spec.Template.Spec.Containers {
						if container.Name == distribute.DeployContainerName {
							err = updater.UpdateCronJobImage(sts.Namespace, sts.Name, distribute.DeployContainerName, distribute.Image, p.kubeClient, kubeclient.VersionLessThan121(versionInfo))
							if err != nil {
								err = errors.WithMessagef(
									err,
									"failed to update container image in %s/cornJob/%s/%s",
									distribute.DeployNamespace, sts.Name, container.Name)
								distribute.DeployEndTime = time.Now().Unix()
								distribute.DeployStatus = string(config.StatusFailed)
								p.SetStatus(config.StatusFailed)
								continue DistributeLoop
							}
							replaced = true
							break CronLoop
						}
					}
				}
			CronBetaLoop:
				for _, sts := range cronJobBetas {
					for _, container := range sts.Spec.JobTemplate.Spec.Template.Spec.Containers {
						if container.Name == distribute.DeployContainerName {
							err = updater.UpdateCronJobImage(sts.Namespace, sts.Name, distribute.DeployContainerName, distribute.Image, p.kubeClient, kubeclient.VersionLessThan121(versionInfo))
							if err != nil {
								err = errors.WithMessagef(
									err,
									"failed to update container image in %s/cornJobBeta/%s/%s",
									distribute.DeployNamespace, sts.Name, container.Name)
								distribute.DeployEndTime = time.Now().Unix()
								distribute.DeployStatus = string(config.StatusFailed)
								p.SetStatus(config.StatusFailed)
								continue DistributeLoop
							}
							replaced = true
							break CronBetaLoop
						}
					}
				}
			} else {
				switch serviceInfo.WorkloadType {
				case setting.StatefulSet:
					var statefulSet *appsv1.StatefulSet
					var found bool
					statefulSet, found, err = getter.GetStatefulSet(distribute.DeployNamespace, distribute.DeployServiceName, p.kubeClient)
					if !found {
						err = fmt.Errorf("statefulset %s not found", distribute.DeployServiceName)
					}
					if err != nil {
						err = errors.WithMessage(err, "failed to get statefulset")
						distribute.DeployStatus = string(config.StatusFailed)
						distribute.DeployEndTime = time.Now().Unix()
						continue
					}
					for _, container := range statefulSet.Spec.Template.Spec.Containers {
						if container.Name == distribute.DeployContainerName {
							err = updater.UpdateStatefulSetImage(statefulSet.Namespace, statefulSet.Name, distribute.DeployContainerName, distribute.Image, p.kubeClient)
							if err != nil {
								err = errors.WithMessagef(
									err,
									"failed to update container image in %s/statefulsets/%s/%s",
									distribute.DeployNamespace, statefulSet.Name, container.Name)
								distribute.DeployStatus = string(config.StatusFailed)
								distribute.DeployEndTime = time.Now().Unix()
								continue DistributeLoop
							}
							replaced = true
							break
						}
					}
				case setting.Deployment:
					var deployment *appsv1.Deployment
					var found bool
					deployment, found, err = getter.GetDeployment(distribute.DeployNamespace, distribute.DeployServiceName, p.kubeClient)
					if !found {
						err = fmt.Errorf("deployment %s not found", distribute.DeployServiceName)
					}
					if err != nil {
						err = errors.WithMessage(err, "failed to get deployment")
						distribute.DeployStatus = string(config.StatusFailed)
						distribute.DeployEndTime = time.Now().Unix()
						continue
					}
					for _, container := range deployment.Spec.Template.Spec.Containers {
						if container.Name == distribute.DeployContainerName {
							err = updater.UpdateDeploymentImage(deployment.Namespace, deployment.Name, distribute.DeployContainerName, distribute.Image, p.kubeClient)
							if err != nil {
								err = errors.WithMessagef(
									err,
									"failed to update container image in %s/deployment/%s/%s",
									distribute.DeployNamespace, deployment.Name, container.Name)
								distribute.DeployStatus = string(config.StatusFailed)
								distribute.DeployEndTime = time.Now().Unix()
								continue DistributeLoop
							}
							replaced = true
							break
						}
					}
				case setting.CronJob:
					cronJob, cronJobBeta, found, errFoundCron := getter.GetCronJob(distribute.DeployNamespace, distribute.DeployServiceName, p.kubeClient, kubeclient.VersionLessThan121(versionInfo))
					if !found {
						err = fmt.Errorf("cronJob %s not found", distribute.DeployServiceName)
					}
					if errFoundCron != nil {
						err = errors.WithMessage(err, "failed to get cronJob")
						distribute.DeployStatus = string(config.StatusFailed)
						distribute.DeployEndTime = time.Now().Unix()
						continue
					}
					if cronJob != nil {
						for _, container := range cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers {
							if container.Name == distribute.DeployContainerName {
								err = updater.UpdateCronJobImage(cronJob.Namespace, cronJob.Name, distribute.DeployContainerName, distribute.Image, p.kubeClient, kubeclient.VersionLessThan121(versionInfo))
								if err != nil {
									err = errors.WithMessagef(
										err,
										"failed to update container image in %s/cornJob/%s/%s",
										distribute.DeployNamespace, cronJob.Name, container.Name)
									distribute.DeployEndTime = time.Now().Unix()
									distribute.DeployStatus = string(config.StatusFailed)
									continue DistributeLoop
								}
								replaced = true
								break
							}
						}
					}
					if cronJobBeta != nil {
						for _, container := range cronJobBeta.Spec.JobTemplate.Spec.Template.Spec.Containers {
							if container.Name == distribute.DeployContainerName {
								err = updater.UpdateCronJobImage(cronJobBeta.Namespace, cronJobBeta.Name, distribute.DeployContainerName, distribute.Image, p.kubeClient, kubeclient.VersionLessThan121(versionInfo))
								if err != nil {
									err = errors.WithMessagef(
										err,
										"failed to update container image in %s/cornJobBeta/%s/%s",
										distribute.DeployNamespace, cronJobBeta.Name, container.Name)
									distribute.DeployEndTime = time.Now().Unix()
									distribute.DeployStatus = string(config.StatusFailed)
									continue DistributeLoop
								}
								replaced = true
								break
							}
						}
					}
				}

			}
			if !replaced {
				err = errors.Errorf(
					"container %s is not found in resources with label %s", distribute.DeployContainerName, selector)
				distribute.DeployStatus = string(config.StatusFailed)
				distribute.DeployEndTime = time.Now().Unix()
				break
			}
			// if all is done in one deployment, update its status to success and endtime
			distribute.DeployStatus = string(config.StatusPassed)
			distribute.DeployEndTime = time.Now().Unix()
		} else {
			// helm deployment type logic goes here
			var (
				productInfo              *types.Product
				mergedValuesYaml         string
				replacedMergedValuesYaml string
				servicePath              string
				serviceRender            *template.ServiceRender
				chartPath                string
				replaceValuesMap         map[string]interface{}
				helmClient               helmclient.Client
			)

			p.Log.Infof("start helm deploy, productName %s serviceName %s containerName %s namespace %s",
				p.Task.ProductName,
				distribute.DeployServiceName,
				distribute.DeployContainerName,
				distribute.DeployNamespace)

			productInfo, err = p.getProductInfo(ctx, &EnvArgs{
				EnvName:     distribute.DeployEnv,
				ProductName: p.Task.ProductName,
			})
			if err != nil {
				err = errors.WithMessagef(
					err,
					"failed to get product %s/%s",
					distribute.DeployNamespace,
					distribute.DeployServiceName)
				distribute.DeployStatus = string(config.StatusFailed)
				distribute.DeployEndTime = time.Now().Unix()
				continue DistributeLoop
			}

			serviceRevisionInProduct := int64(0)
			var targetContainer *commonmodels.Container
			for _, service := range productInfo.GetServiceMap() {
				if service.ServiceName == distribute.DeployServiceName {
					serviceRevisionInProduct = service.Revision
					serviceRender = service.Render
					for _, container := range service.Containers {
						if container.Name == distribute.DeployContainerName {
							targetContainer = container
							break
						}
					}
					break
				}
			}
			if serviceRender == nil {
				serviceRender = &template.ServiceRender{ServiceName: distribute.DeployServiceName}
			}

			if targetContainer == nil {
				err = errors.Errorf("failed to find target container %s from service %s", distribute.DeployContainerName, distribute.DeployServiceName)
				distribute.DeployStatus = string(config.StatusFailed)
				distribute.DeployEndTime = time.Now().Unix()
				continue DistributeLoop
			}

			if targetContainer.ImagePath == nil {
				err = errors.Errorf("failed to get image path of  %s from service %s", distribute.DeployContainerName, distribute.DeployServiceName)
				distribute.DeployStatus = string(config.StatusFailed)
				distribute.DeployEndTime = time.Now().Unix()
				continue DistributeLoop
			}

			// use revision of service currently applied in environment instead of the latest revision
			path, errDownload := p.downloadService(p.Task.ProductName, distribute.DeployServiceName,
				p.StorageURI, serviceRevisionInProduct)
			if errDownload != nil {
				p.Log.Warnf("failed to get chart of revision: %d for service: %s, use latest version",
					serviceRevisionInProduct, distribute.DeployServiceName)
				path, errDownload = p.downloadService(p.Task.ProductName, distribute.DeployServiceName,
					p.StorageURI, 0)
				if errDownload != nil {
					err = errors.WithMessagef(
						errDownload,
						"failed to download service %s/%s",
						distribute.DeployNamespace,
						distribute.DeployServiceName,
					)
					distribute.DeployStatus = string(config.StatusFailed)
					distribute.DeployEndTime = time.Now().Unix()
					continue DistributeLoop
				}
			}

			chartPath, err = fsutil.RelativeToCurrentPath(path)
			if err != nil {
				err = errors.WithMessagef(
					err,
					"failed to get relative path %s",
					servicePath,
				)
				distribute.DeployStatus = string(config.StatusFailed)
				distribute.DeployEndTime = time.Now().Unix()
				continue DistributeLoop
			}

			// prepare image replace info
			validMatchData := commonutil.GetValidMatchData(targetContainer.ImagePath)

			imageKVS := make([]*helmtool.KV, 0)

			replaceValuesMap, err = commonutil.AssignImageData(distribute.Image, validMatchData)
			if err != nil {
				err = errors.WithMessagef(
					err,
					"failed to pase image uri %s/%s",
					distribute.DeployNamespace,
					distribute.DeployServiceName,
				)
				distribute.DeployStatus = string(config.StatusFailed)
				distribute.DeployEndTime = time.Now().Unix()
				continue DistributeLoop
			}
			p.Log.Infof("assing image data for image: %s, assign data: %v", targetContainer.Image, replaceValuesMap)

			for key, value := range replaceValuesMap {
				imageKVS = append(imageKVS, &helmtool.KV{
					Key:   key,
					Value: value,
				})
			}

			// merge override values and kvs into service's yaml
			mergedValuesYaml, err = helmtool.MergeOverrideValues("", productInfo.DefaultValues, serviceRender.GetOverrideYaml(), serviceRender.OverrideValues, imageKVS)
			if err != nil {
				err = errors.WithMessagef(
					err,
					"failed to merge override values %s",
					productInfo.DefaultValues,
				)
				distribute.DeployStatus = string(config.StatusFailed)
				distribute.DeployEndTime = time.Now().Unix()
				continue DistributeLoop
			}
			p.Log.Infof("final minimum merged values.yaml: \n%s", mergedValuesYaml)
			replacedMergedValuesYaml = mergedValuesYaml

			helmClient, err = helmtool.NewClientFromNamespace(distribute.DeployClusterID, distribute.DeployNamespace)
			if err != nil {
				err = errors.WithMessagef(
					err,
					"failed to create helm client %s/%s",
					distribute.DeployNamespace,
					distribute.DeployServiceName,
				)
				distribute.DeployStatus = string(config.StatusFailed)
				distribute.DeployEndTime = time.Now().Unix()
				continue DistributeLoop
			}

			releaseName := distribute.ReleaseName

			ensureUpgrade := func() error {
				hrs, errHistory := helmClient.ListReleaseHistory(releaseName, 10)
				if errHistory != nil {
					// list history should not block deploy operation, error will be logged instead of returned
					p.Log.Errorf("failed to list release history, release: %s, err: %s", releaseName, errHistory)
					return nil
				}
				if len(hrs) == 0 {
					return nil
				}
				releaseutil.Reverse(hrs, releaseutil.SortByRevision)
				rel := hrs[0]

				if rel.Info.Status.IsPending() {
					return fmt.Errorf("failed to upgrade release: %s with exceptional status: %s", releaseName, rel.Info.Status)
				}
				return nil
			}

			err = ensureUpgrade()
			if err != nil {
				distribute.DeployStatus = string(config.StatusFailed)
				distribute.DeployEndTime = time.Now().Unix()
				continue DistributeLoop
			}

			timeOut := p.TaskTimeout()
			chartSpec := helmclient.ChartSpec{
				ReleaseName: releaseName,
				ChartName:   chartPath,
				Namespace:   distribute.DeployNamespace,
				ReuseValues: true,
				Version:     serviceRender.ChartVersion,
				ValuesYaml:  replacedMergedValuesYaml,
				SkipCRDs:    false,
				UpgradeCRDs: true,
				Timeout:     time.Second * time.Duration(timeOut),
				Wait:        true,
				Replace:     true,
				MaxHistory:  10,
			}

			done := make(chan bool)
			go func(chan bool) {
				if _, err = helmClient.InstallOrUpgradeChart(ctx, &chartSpec, nil); err != nil {
					err = errors.WithMessagef(
						err,
						"failed to upgrade helm chart %s/%s",
						distribute.DeployNamespace,
						distribute.DeployServiceName,
					)
					done <- false
				} else {
					done <- true
				}
			}(done)

			select {
			case <-done:
				break
			case <-time.After(chartSpec.Timeout + time.Minute):
				err = fmt.Errorf("failed to upgrade relase: %s, timeout", chartSpec.ReleaseName)
			}
			if err != nil {
				distribute.DeployStatus = string(config.StatusFailed)
				distribute.DeployEndTime = time.Now().Unix()
				continue DistributeLoop
			}

			distribute.DeployStatus = string(config.StatusPassed)
			distribute.DeployEndTime = time.Now().Unix()
		}
	}
}

// Complete ...
func (p *ReleaseImagePlugin) Complete(ctx context.Context, pipelineTask *task.Task, serviceName string) {
	jobLabel := &label.JobLabel{
		PipelineName: pipelineTask.PipelineName,
		ServiceName:  serviceName,
		TaskID:       pipelineTask.TaskID,
		TaskType:     string(p.Type()),
		PipelineType: string(pipelineTask.Type),
	}

	// 清理用户取消和超时的任务
	defer func() {
		if err := ensureDeleteConfigMap(p.KubeNamespace, jobLabel, p.kubeClient); err != nil {
			p.Log.Error(err)
		}
		if err := ensureDeleteJob(p.KubeNamespace, jobLabel, p.kubeClient); err != nil {
			p.Log.Error(err)
		}
	}()

	// 保存实时日志到s3
	err := saveContainerLog(pipelineTask, p.KubeNamespace, "", p.FileName, jobLabel, p.kubeClient)
	if err != nil {
		p.Log.Error(err)
		p.Task.Error = err.Error()
		return
	}

	p.Task.LogFile = p.JobName
}

// SetTask ...
func (p *ReleaseImagePlugin) SetTask(t map[string]interface{}) error {
	task, err := ToReleaseImageTask(t)
	if err != nil {
		return err
	}
	p.Task = task
	return nil
}

// GetTask ...
func (p *ReleaseImagePlugin) GetTask() interface{} {
	return p.Task
}

// IsTaskDone ...
func (p *ReleaseImagePlugin) IsTaskDone() bool {
	if p.Task.TaskStatus != config.StatusCreated && p.Task.TaskStatus != config.StatusRunning {
		return true
	}
	return false
}

// IsTaskFailed ...
func (p *ReleaseImagePlugin) IsTaskFailed() bool {
	if p.Task.TaskStatus == config.StatusFailed || p.Task.TaskStatus == config.StatusTimeout || p.Task.TaskStatus == config.StatusCancelled {
		return true
	}
	return false
}

// SetStartTime ...
func (p *ReleaseImagePlugin) SetStartTime() {
	startTime := time.Now().Unix()
	p.Task.StartTime = startTime
	for _, distribute := range p.Task.DistributeInfo {
		distribute.DistributeStartTime = startTime
	}
}

// SetEndTime ...
func (p *ReleaseImagePlugin) SetEndTime() {
	p.Task.EndTime = time.Now().Unix()
}

// IsTaskEnabled ...
func (p *ReleaseImagePlugin) IsTaskEnabled() bool {
	return p.Task.Enabled
}

// ResetError ...
func (p *ReleaseImagePlugin) ResetError() {
	p.Task.Error = ""
}

func (p *ReleaseImagePlugin) getService(ctx context.Context, name, serviceType, productName string, revision int64) (*types.ServiceTmpl, error) {
	url := fmt.Sprintf("/api/service/services/%s/%s", name, serviceType)

	s := &types.ServiceTmpl{}
	_, err := p.httpClient.Get(url, httpclient.SetResult(s), httpclient.SetQueryParams(map[string]string{
		"projectName": productName,
		"revision":    fmt.Sprintf("%d", revision),
	}))
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (p *ReleaseImagePlugin) getProductInfo(ctx context.Context, args *EnvArgs) (*types.Product, error) {
	url := fmt.Sprintf("/api/environment/environments/%s/productInfo", args.EnvName)

	prod := &types.Product{}
	_, err := p.httpClient.Get(url, httpclient.SetResult(prod), httpclient.SetQueryParam("projectName", args.ProductName))
	if err != nil {
		return nil, err
	}
	return prod, nil
}

func (p *ReleaseImagePlugin) downloadService(productName, serviceName, storageURI string, revision int64) (string, error) {
	logger := p.Log

	fileName := serviceName
	if revision > 0 {
		fileName = fmt.Sprintf("%s-%d", serviceName, revision)
	}
	tarball := fmt.Sprintf("%s.tar.gz", fileName)
	localBase := configbase.LocalTestServicePath(productName, serviceName)
	tarFilePath := filepath.Join(localBase, tarball)

	exists, err := fsutil.FileExists(tarFilePath)
	if err != nil {
		return "", err
	}
	if exists {
		return tarFilePath, nil
	}

	s3Storage, err := s3.UnmarshalNewS3StorageFromEncrypted(storageURI)
	if err != nil {
		return "", err
	}

	s3Storage.Subfolder = filepath.Join(s3Storage.Subfolder, configbase.ObjectStorageServicePath(productName, serviceName))
	forcedPathStyle := true
	if s3Storage.Provider == setting.ProviderSourceAli {
		forcedPathStyle = false
	}
	s3Client, err := s3tool.NewClient(s3Storage.Endpoint, s3Storage.Ak, s3Storage.Sk, s3Storage.Region, s3Storage.Insecure, forcedPathStyle)
	if err != nil {
		p.Log.Errorf("failed to create s3 client, err: %s", err)
		return "", err
	}
	if err = s3Client.Download(s3Storage.Bucket, s3Storage.GetObjectPath(tarball), tarFilePath); err != nil {
		logger.Errorf("failed to download file from s3, err: %s", err)
		return "", err
	}

	exists, err = fsutil.FileExists(tarFilePath)
	if err != nil {
		return "", err
	}
	if !exists {
		return "", fmt.Errorf("file %s on s3 not found", s3Storage.GetObjectPath(tarball))
	}

	return tarFilePath, nil
}
