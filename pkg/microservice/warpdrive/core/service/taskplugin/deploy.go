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
	"encoding/json"
	"fmt"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	configbase "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/warpdrive/config"
	"github.com/koderover/zadig/pkg/microservice/warpdrive/core/service/taskplugin/s3"
	"github.com/koderover/zadig/pkg/microservice/warpdrive/core/service/types"
	"github.com/koderover/zadig/pkg/microservice/warpdrive/core/service/types/task"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/kube/wrapper"
	"github.com/koderover/zadig/pkg/tool/helmclient"
	"github.com/koderover/zadig/pkg/tool/httpclient"
	krkubeclient "github.com/koderover/zadig/pkg/tool/kube/client"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"github.com/koderover/zadig/pkg/tool/kube/multicluster"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
	s3tool "github.com/koderover/zadig/pkg/tool/s3"
	"github.com/koderover/zadig/pkg/util"
)

// InitializeDeployTaskPlugin to initiate deploy task plugin and return ref
func InitializeDeployTaskPlugin(taskType config.TaskType) TaskPlugin {
	return &DeployTaskPlugin{
		Name:       taskType,
		kubeClient: krkubeclient.Client(),
		restConfig: krkubeclient.RESTConfig(),
		httpClient: httpclient.New(
			httpclient.SetAuthScheme(setting.RootAPIKey),
			httpclient.SetAuthToken(config.PoetryAPIRootKey()),
			httpclient.SetHostURL(configbase.AslanServiceAddress()),
		),
	}
}

// DeployTaskPlugin Plugin name should be compatible with task type
type DeployTaskPlugin struct {
	Name         config.TaskType
	JobName      string
	kubeClient   client.Client
	restConfig   *rest.Config
	Task         *task.Deploy
	Log          *zap.SugaredLogger
	ReplaceImage string

	httpClient *httpclient.Client
}

func (p *DeployTaskPlugin) SetAckFunc(func()) {
}

const (
	// DeployTimeout ...
	DeployTimeout    = 60 * 10 // 10 minutes
	ImageRegexString = "^[a-zA-Z0-9.:\\/-]+$"
)

var (
	ImageRegex = regexp.MustCompile(ImageRegexString)
)

// Init ...
func (p *DeployTaskPlugin) Init(jobname, filename string, xl *zap.SugaredLogger) {
	p.JobName = jobname
	// SetLogger ...
	p.Log = xl
}

// Type ...
func (p *DeployTaskPlugin) Type() config.TaskType {
	return p.Name
}

// Status ...
func (p *DeployTaskPlugin) Status() config.Status {
	return p.Task.TaskStatus
}

// SetStatus ...
func (p *DeployTaskPlugin) SetStatus(status config.Status) {
	p.Task.TaskStatus = status
}

// TaskTimeout ...
func (p *DeployTaskPlugin) TaskTimeout() int {
	if p.Task.Timeout == 0 {
		p.Task.Timeout = DeployTimeout
	} else {
		if !p.Task.IsRestart {
			p.Task.Timeout = p.Task.Timeout * 60
		}
	}

	return p.Task.Timeout
}

type EnvArgs struct {
	EnvName     string `json:"env_name"`
	ProductName string `json:"product_name"`
}

type SelectorBuilder struct {
	ProductName  string
	GroupName    string
	ServiceName  string
	ConfigBackup string
	EnvName      string
}

const (
	// ProductLabel ...
	ProductLabel = "s-product"
	// GroupLabel ...
	GroupLabel = "s-group"
	// ServiceLabel ...
	ServiceLabel = "s-service"
	// ConfigBackupLabel ...
	ConfigBackupLabel = "config-backup"
	// NamespaceLabel
	EnvNameLabel = "s-env"
	// EnvName ...
	UpdateBy   = "update-by"
	UpdateByID = "update-by-id"
	UpdateTime = "update-time"
)

func (p *DeployTaskPlugin) Run(ctx context.Context, pipelineTask *task.Task, _ *task.PipelineCtx, _ string) {
	var (
		err      error
		replaced = false
	)

	defer func() {
		if err != nil {
			p.Log.Error(err)
			p.Task.TaskStatus = config.StatusFailed
			p.Task.Error = err.Error()
			return
		}
	}()

	if pipelineTask.ConfigPayload.DeployClusterID != "" {
		p.restConfig, err = multicluster.GetRESTConfig(pipelineTask.ConfigPayload.HubServerAddr, pipelineTask.ConfigPayload.DeployClusterID)
		if err != nil {
			err = errors.WithMessage(err, "can't get k8s rest config")
			return
		}

		p.kubeClient, err = multicluster.GetKubeClient(pipelineTask.ConfigPayload.HubServerAddr, pipelineTask.ConfigPayload.DeployClusterID)
		if err != nil {
			err = errors.WithMessage(err, "can't init k8s client")
			return
		}
	}

	if p.Task.ServiceType != setting.HelmDeployType {
		selector := labels.Set{setting.ProductLabel: p.Task.ProductName, setting.ServiceLabel: p.Task.ServiceName}.AsSelector()

		var deployments []*appsv1.Deployment
		deployments, err = getter.ListDeployments(p.Task.Namespace, selector, p.kubeClient)
		if err != nil {
			return
		}

		var statefulSets []*appsv1.StatefulSet
		statefulSets, err = getter.ListStatefulSets(p.Task.Namespace, selector, p.kubeClient)
		if err != nil {
			return
		}

		for _, deploy := range deployments {
			for _, container := range deploy.Spec.Template.Spec.Containers {
				if container.Name == p.Task.ContainerName {
					err = updater.UpdateDeploymentImage(deploy.Namespace, deploy.Name, p.Task.ContainerName, p.Task.Image, p.kubeClient)
					if err != nil {
						err = errors.WithMessagef(
							err,
							"failed to update container image in %s/deployments/%s/%s",
							p.Task.Namespace, deploy.Name, container.Name)
						return
					}
					p.Task.ReplaceResources = append(p.Task.ReplaceResources, task.Resource{
						Kind:      setting.Deployment,
						Container: container.Name,
						Origin:    container.Image,
						Name:      deploy.Name,
					})
					replaced = true
				}
			}
		}

		for _, sts := range statefulSets {
			for _, container := range sts.Spec.Template.Spec.Containers {
				if container.Name == p.Task.ContainerName {
					err = updater.UpdateStatefulSetImage(sts.Namespace, sts.Name, p.Task.ContainerName, p.Task.Image, p.kubeClient)
					if err != nil {
						err = errors.WithMessagef(
							err,
							"failed to update container image in %s/statefulsets/%s/%s",
							p.Task.Namespace, sts.Name, container.Name)
						return
					}
					p.Task.ReplaceResources = append(p.Task.ReplaceResources, task.Resource{
						Kind:      setting.StatefulSet,
						Container: container.Name,
						Origin:    container.Image,
						Name:      sts.Name,
					})
					replaced = true
				}
			}
		}
		if !replaced {
			err = errors.Errorf(
				"container %s is not found in resources with label %s", p.Task.ContainerName, selector)
			return
		}
	} else if p.Task.ServiceType == setting.HelmDeployType {
		var (
			productInfo       *types.Product
			chartInfoMap      = make(map[string]*types.RenderChart)
			renderChart       *types.RenderChart
			isExist           = false
			replaceValuesYaml string
			yamlValuesByte    []byte
			renderInfo        *types.RenderSet
			helmClient        helmclient.Client
			serviceTemplate   *types.ServiceTmpl
		)

		deployments, _ := getter.ListDeployments(p.Task.Namespace, nil, p.kubeClient)
		for _, deploy := range deployments {
			for _, container := range deploy.Spec.Template.Spec.Containers {
				if strings.Contains(container.Image, p.Task.ContainerName) {
					p.Log.Infof("deployments find match container.name:%s", container.Name)
					p.Task.ReplaceResources = append(p.Task.ReplaceResources, task.Resource{
						Kind:      setting.Deployment,
						Container: container.Name,
						Origin:    container.Image,
						Name:      deploy.Name,
					})
				}
			}
		}

		statefulSets, _ := getter.ListStatefulSets(p.Task.Namespace, nil, p.kubeClient)
		for _, sts := range statefulSets {
			for _, container := range sts.Spec.Template.Spec.Containers {
				if strings.Contains(container.Image, p.Task.ContainerName) {
					p.Log.Infof("statefulSets find match container.name:%s", container.Name)
					p.Task.ReplaceResources = append(p.Task.ReplaceResources, task.Resource{
						Kind:      setting.StatefulSet,
						Container: container.Name,
						Origin:    container.Image,
						Name:      sts.Name,
					})
				}
			}
		}

		productInfo, err = p.getProductInfo(ctx, &EnvArgs{EnvName: p.Task.EnvName, ProductName: p.Task.ProductName})
		if err != nil {
			err = errors.WithMessagef(
				err,
				"failed to get product %s/%s",
				p.Task.Namespace, p.Task.ServiceName)
			return
		}
		renderInfo, err = p.getRenderSet(ctx, productInfo.Render.Name, productInfo.Render.Revision)
		if err != nil {
			err = errors.WithMessagef(
				err,
				"failed to get getRenderSet %s/%d",
				productInfo.Render.Name, productInfo.Render.Revision)
			return
		}
		for _, chartInfo := range renderInfo.ChartInfos {
			chartInfoMap[chartInfo.ServiceName] = chartInfo
		}
		if renderChart, isExist = chartInfoMap[p.Task.ServiceName]; isExist {
			yamlValuesByte, err = yaml.YAMLToJSON([]byte(renderChart.ValuesYaml))
			if err != nil {
				err = errors.WithMessagef(
					err,
					"failed to YAMLToJSON %s/%s",
					p.Task.Namespace, p.Task.ServiceName)
				return
			}

			var currentValuesYamlMap map[string]interface{}
			if err = json.Unmarshal(yamlValuesByte, &currentValuesYamlMap); err != nil {
				err = errors.WithMessagef(
					err,
					"failed to Unmarshal values.yaml %s/%s",
					p.Task.Namespace, p.Task.ServiceName)
				return
			}

			//找到需要替换的镜像
			p.ReplaceImage = ""
			p.recursionReplaceImage(currentValuesYamlMap, p.Task.Image)
			if p.ReplaceImage != "" {
				//先替换镜像名称
				var oldImageName, newImageName, newImageTag string
				oldImageArr := strings.Split(p.ReplaceImage, ":")
				newImageArr := strings.Split(p.Task.Image, ":")

				if len(oldImageArr) < 2 || len(newImageArr) < 2 {
					err = errors.WithMessagef(
						err,
						"image is invalid %s/%s",
						p.ReplaceImage, p.Task.Image)
					return
				}

				oldImageName = oldImageArr[0]
				newImageName = newImageArr[0]
				newImageTag = newImageArr[1]
				//根据value找到对应的key
				currentChartInfoMap := util.GetJSONData(currentValuesYamlMap)
				sameKeyMap := make(map[string]interface{})
				for mapKey, mapValue := range currentChartInfoMap {
					if mapValue == oldImageName {
						sameKeyMap[mapKey] = newImageName
						imageTag := strings.Replace(mapKey, ".repository", ".tag", 1)
						sameKeyMap[imageTag] = newImageTag
					}
				}

				replaceMap := util.ReplaceMapValue(currentValuesYamlMap, sameKeyMap)
				replaceValuesYaml, err = util.JSONToYaml(replaceMap)
				if err != nil {
					err = errors.WithMessagef(
						err,
						"failed to jsonToYaml %s/%s",
						p.Task.Namespace, p.Task.ServiceName)
					return
				}
			} else {
				p.recursionReplaceImageByColon(currentValuesYamlMap, p.Task.Image)
				if p.ReplaceImage != "" {
					replaceValuesYaml = renderChart.ValuesYaml
					replaceValuesYaml = strings.Replace(replaceValuesYaml, p.ReplaceImage, p.Task.Image, -1)
				}
			}
			if replaceValuesYaml != "" {
				helmClient, err = helmclient.NewClientFromRestConf(p.restConfig, p.Task.Namespace)
				if err != nil {
					err = errors.WithMessagef(
						err,
						"failed to create helm client %s/%s",
						p.Task.Namespace, p.Task.ServiceName)
					return
				}
				chartSpec := helmclient.ChartSpec{
					ReleaseName: fmt.Sprintf("%s-%s", p.Task.Namespace, p.Task.ServiceName),
					ChartName:   fmt.Sprintf("%s/%s", p.Task.Namespace, p.Task.ServiceName),
					Namespace:   p.Task.Namespace,
					Wait:        true,
					ReuseValues: true,
					Version:     renderChart.ChartVersion,
					ValuesYaml:  replaceValuesYaml,
					SkipCRDs:    false,
					UpgradeCRDs: true,
					Timeout:     time.Second * DeployTimeout,
				}

				serviceTemplate, err = p.getService(ctx, p.Task.ServiceName, p.Task.ServiceType, p.Task.ProductName)
				if err != nil {
					err = errors.WithMessagef(
						err,
						"failed to get service %s/%s",
						p.Task.Namespace, p.Task.ServiceName)
					return
				}

				base := path.Join(pipelineTask.ConfigPayload.S3Storage.Path, serviceTemplate.RepoName, serviceTemplate.LoadPath)
				if err = p.downloadService(pipelineTask, p.Task.ServiceName, serviceTemplate.RepoName); err != nil {
					err = errors.WithMessagef(
						err,
						"failed to download service %s/%s",
						p.Task.Namespace, p.Task.ServiceName)
					return
				}

				if err = helmClient.InstallOrUpgradeChart(context.Background(), &chartSpec, &helmclient.ChartOption{
					ChartPath: base}, p.Log); err != nil {
					err = errors.WithMessagef(
						err,
						"failed to Install helm chart %s/%s",
						p.Task.Namespace, p.Task.ServiceName)
					return
				}

				//替换环境变量中的chartInfos
				for _, chartInfo := range renderInfo.ChartInfos {
					if chartInfo.ServiceName == p.Task.ServiceName {
						chartInfo.ValuesYaml = replaceValuesYaml
						break
					}
				}
				_ = p.updateRenderSet(ctx, &types.RenderSet{
					Name:       renderInfo.Name,
					Revision:   renderInfo.Revision,
					ChartInfos: renderInfo.ChartInfos,
				})

			}
			return
		}
		err = errors.WithMessagef(
			err,
			"failed to update container image in %s/%s，not find",
			p.Task.Namespace, p.Task.ServiceName)
	}
}

func (p *DeployTaskPlugin) getProductInfo(ctx context.Context, args *EnvArgs) (*types.Product, error) {
	url := fmt.Sprintf("/api/environment/environments/%s/productInfo", args.ProductName)

	prod := &types.Product{}
	_, err := p.httpClient.Get(url, httpclient.SetResult(prod), httpclient.SetQueryParam("envName", args.EnvName))
	if err != nil {
		return nil, err
	}
	return prod, nil
}

func (p *DeployTaskPlugin) getService(ctx context.Context, name, serviceType, productName string) (*types.ServiceTmpl, error) {
	url := fmt.Sprintf("/api/service/services/%s/%s", name, serviceType)

	s := &types.ServiceTmpl{}
	_, err := p.httpClient.Get(url, httpclient.SetResult(s), httpclient.SetQueryParam("productName", productName))
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (p *DeployTaskPlugin) downloadService(pipelineTask *task.Task, serviceName, repoName string) error {
	var (
		s3Storage *s3.S3
		err       error
		base      string
	)
	base = path.Join(pipelineTask.ConfigPayload.S3Storage.Path, repoName)
	if s3Storage, err = s3.NewS3StorageFromEncryptedURI(pipelineTask.StorageURI); err != nil {
		return err
	}
	subFolderName := serviceName + "-" + setting.HelmDeployType
	if s3Storage.Subfolder != "" {
		s3Storage.Subfolder = fmt.Sprintf("%s/%s/%s", s3Storage.Subfolder, subFolderName, "service")
	} else {
		s3Storage.Subfolder = fmt.Sprintf("%s/%s", subFolderName, "service")
	}

	filePath := fmt.Sprintf("%s.tar.gz", serviceName)
	tarFilePath := path.Join(base, filePath)
	forcedPathStyle := false
	if s3Storage.Provider == setting.ProviderSourceSystemDefault {
		forcedPathStyle = true
	}
	s3client, err := s3tool.NewClient(s3Storage.Endpoint, s3Storage.Ak, s3Storage.Sk, s3Storage.Insecure, forcedPathStyle)
	if err != nil {
		p.Log.Errorf("failed to create s3 client, err: %+v", err)
		return err
	}
	objectKey := s3Storage.GetObjectPath(filePath)
	if err := s3client.Download(s3Storage.Bucket, objectKey, tarFilePath); err != nil {
		p.Log.Errorf("s3下载文件失败 err:%v", err)
		return err
	}
	if err := util.UnTar("/", tarFilePath); err != nil {
		p.Log.Errorf("unTar err:%v", err)
		return err
	}
	if err := os.Remove(tarFilePath); err != nil {
		p.Log.Errorf("remove file err:%v", err)
		return err
	}
	return nil
}

func (p *DeployTaskPlugin) getRenderSet(ctx context.Context, name string, revision int64) (*types.RenderSet, error) {
	url := fmt.Sprintf("/api/project/renders/render/%s/revision/%d", name, revision)

	rs := &types.RenderSet{}
	_, err := p.httpClient.Get(url, httpclient.SetResult(rs))
	if err != nil {
		return nil, err
	}
	return rs, nil
}

func (p *DeployTaskPlugin) updateRenderSet(ctx context.Context, args *types.RenderSet) error {
	url := "/api/project/renders"

	_, err := p.httpClient.Put(url, httpclient.SetBody(args))

	return err
}

// 递归循环找到要替换的镜像的标签
func (p *DeployTaskPlugin) recursionReplaceImage(jsonValues map[string]interface{}, image string) {
	for jsonKey, jsonValue := range jsonValues {
		levelMap := p.isMap(jsonValue)
		if levelMap != nil {
			p.recursionReplaceImage(levelMap, image)
		} else if repository, isStr := jsonValue.(string); isStr {
			if !strings.Contains(jsonKey, "repository") {
				continue
			}
			if imageTag, isExist := jsonValues["tag"]; isExist {
				if imageTag == "" {
					continue
				}
				currentImageName := strings.Split(image, ":")[0]
				currentImageNameArr := strings.Split(currentImageName, "/")
				serviceName := currentImageNameArr[len(currentImageNameArr)-1]
				if strings.Contains(repository, serviceName) {
					p.Log.Infof("find replace imageName:%s imageTag:%v", repository, imageTag)
					p.ReplaceImage = fmt.Sprintf("%s:%v", repository, imageTag)
					return
				}
			}
		}
	}
}

// 递归循环找到根据包含冒号要替换的镜像的标签
func (p *DeployTaskPlugin) recursionReplaceImageByColon(jsonValues map[string]interface{}, image string) {
	for _, jsonValue := range jsonValues {
		levelMap := p.isMap(jsonValue)
		if levelMap != nil {
			p.recursionReplaceImageByColon(levelMap, image)
		} else if repository, isStr := jsonValue.(string); isStr {
			if strings.Contains(repository, ":") && ImageRegex.MatchString(repository) {
				currentImageName := strings.Split(image, ":")[0]
				currentImageNameArr := strings.Split(currentImageName, "/")
				serviceName := currentImageNameArr[len(currentImageNameArr)-1]
				if strings.Contains(repository, serviceName) {
					p.Log.Infof("find replace image:%s", repository)
					p.ReplaceImage = repository
					return
				}
			}
		}
	}
}

func (p *DeployTaskPlugin) isMap(yamlMap interface{}) map[string]interface{} {
	switch value := yamlMap.(type) {
	case map[string]interface{}:
		return value
	default:
		return nil
	}
	return nil
}

// Wait ...
func (p *DeployTaskPlugin) Wait(ctx context.Context) {
	// skip waiting for reset image task
	if p.Task.SkipWaiting {
		p.Task.TaskStatus = config.StatusPassed
		return
	}

	timeout := time.After(time.Duration(p.TaskTimeout()) * time.Second)

	selector := labels.Set{setting.ProductLabel: p.Task.ProductName, setting.ServiceLabel: p.Task.ServiceName}.AsSelector()

	for {
		select {
		case <-ctx.Done():
			p.Task.TaskStatus = config.StatusCancelled
			return

		case <-timeout:
			p.Task.TaskStatus = config.StatusTimeout

			pods, err := getter.ListPods(p.Task.Namespace, selector, p.kubeClient)
			if err != nil {
				p.Task.Error = err.Error()
				return
			}

			var msg []string
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

			if len(msg) != 0 {
				p.Task.Error = strings.Join(msg, "\n")
			}

			return

		default:
			time.Sleep(time.Second * 2)
			ready := true
			var err error
		L:
			for _, resource := range p.Task.ReplaceResources {
				switch resource.Kind {
				case setting.Deployment:
					d, found, e := getter.GetDeployment(p.Task.Namespace, resource.Name, p.kubeClient)
					if e != nil {
						err = e
					}
					if e != nil || !found {
						p.Log.Errorf(
							"failed to check deployment ready status %s/%s/%s - %v",
							p.Task.Namespace,
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
					s, found, e := getter.GetStatefulSet(p.Task.Namespace, resource.Name, p.kubeClient)
					if e != nil {
						err = e
					}
					if err != nil || !found {
						p.Log.Errorf(
							"failed to check statefulSet ready status %s/%s/%s",
							p.Task.Namespace,
							resource.Kind,
							resource.Name,
							e,
						)
						ready = false
					} else {
						ready = wrapper.StatefulSet(s).Ready()
					}

					if !ready {
						break L
					}
				}
			}

			if ready {
				p.Task.TaskStatus = config.StatusPassed
			}

			if p.IsTaskDone() {
				return
			}
		}
	}
}

func (p *DeployTaskPlugin) Complete(ctx context.Context, pipelineTask *task.Task, serviceName string) {
}

func (p *DeployTaskPlugin) SetTask(t map[string]interface{}) error {
	task, err := ToDeployTask(t)
	if err != nil {
		return err
	}
	p.Task = task

	return nil
}

// GetTask ...
func (p *DeployTaskPlugin) GetTask() interface{} {
	return p.Task
}

// IsTaskDone ...
func (p *DeployTaskPlugin) IsTaskDone() bool {
	if p.Task.TaskStatus != config.StatusCreated && p.Task.TaskStatus != config.StatusRunning {
		return true
	}
	return false
}

// IsTaskFailed ...
func (p *DeployTaskPlugin) IsTaskFailed() bool {
	if p.Task.TaskStatus == config.StatusFailed || p.Task.TaskStatus == config.StatusTimeout || p.Task.TaskStatus == config.StatusCancelled {
		return true
	}
	return false
}

// SetStartTime ...
func (p *DeployTaskPlugin) SetStartTime() {
	p.Task.StartTime = time.Now().Unix()
}

// SetEndTime ...
func (p *DeployTaskPlugin) SetEndTime() {
	p.Task.EndTime = time.Now().Unix()
}

// IsTaskEnabled ...
func (p *DeployTaskPlugin) IsTaskEnabled() bool {
	return p.Task.Enabled
}

// ResetError ...
func (p *DeployTaskPlugin) ResetError() {
	p.Task.Error = ""
}
