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

package taskplugin

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	helmclient "github.com/mittwald/go-helm-client"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"helm.sh/helm/v3/pkg/releaseutil"
	"k8s.io/apimachinery/pkg/util/sets"
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
	s3tool "github.com/koderover/zadig/v2/pkg/tool/s3"
	fsutil "github.com/koderover/zadig/v2/pkg/util/fs"
)

// InitializeHelmDeployTaskPlugin initiates a plugin to deploy helm charts and return ref
func InitializeHelmDeployTaskPlugin(taskType config.TaskType) *HelmDeployTaskPlugin {
	return &HelmDeployTaskPlugin{
		Name:       taskType,
		kubeClient: krkubeclient.Client(),
		restConfig: krkubeclient.RESTConfig(),
		httpClient: httpclient.New(
			httpclient.SetHostURL(configbase.AslanServiceAddress()),
		),
	}
}

// HelmDeployTaskPlugin Plugin name should be compatible with task type
type HelmDeployTaskPlugin struct {
	Name         config.TaskType
	JobName      string
	kubeClient   client.Client
	restConfig   *rest.Config
	Task         *task.Deploy
	Log          *zap.SugaredLogger
	ReplaceImage string

	ContentPlugins []*HelmDeployTaskPlugin
	httpClient     *httpclient.Client
}

func (p *HelmDeployTaskPlugin) SetAckFunc(func()) {}

func (p *HelmDeployTaskPlugin) Init(jobname, filename string, xl *zap.SugaredLogger) {
	p.JobName = jobname
	p.Log = xl
}

func (p *HelmDeployTaskPlugin) Type() config.TaskType {
	return p.Name
}

func (p *HelmDeployTaskPlugin) Status() config.Status {
	return p.Task.TaskStatus
}

func (p *HelmDeployTaskPlugin) SetStatus(status config.Status) {
	p.Task.TaskStatus = status
	for _, subPlugin := range p.ContentPlugins {
		subPlugin.Task.TaskStatus = status
	}
}

func (p *HelmDeployTaskPlugin) TaskTimeout() int {
	if p.Task.Timeout == 0 {
		p.Task.Timeout = setting.DeployTimeout
	}

	return p.Task.Timeout
}

func (p *HelmDeployTaskPlugin) Run(ctx context.Context, pipelineTask *task.Task, _ *task.PipelineCtx, _ string) {
	var (
		err error
	)

	defer func() {
		if err != nil {
			p.Log.Error(err)
			p.SetStatus(config.StatusFailed)
			p.Task.Error = err.Error()
			return
		}
	}()

	if pipelineTask.ConfigPayload.DeployClusterID != "" {
		p.restConfig, err = kubeclient.GetRESTConfig(pipelineTask.ConfigPayload.HubServerAddr, pipelineTask.ConfigPayload.DeployClusterID)
		if err != nil {
			err = errors.WithMessage(err, "can't get k8s rest config")
			return
		}

		p.kubeClient, err = kubeclient.GetKubeClient(pipelineTask.ConfigPayload.HubServerAddr, pipelineTask.ConfigPayload.DeployClusterID)
		if err != nil {
			err = errors.WithMessage(err, "can't init k8s client")
			return
		}
	}
	// all involved containers
	containerNameSet := sets.NewString()
	for _, sPlugin := range p.ContentPlugins {
		singleContainerName := strings.TrimSuffix(sPlugin.Task.ContainerName, "_"+p.Task.ServiceName)
		containerNameSet.Insert(singleContainerName)
	}

	var (
		productInfo      *types.Product
		serviceRender    *template.ServiceRender
		mergedValuesYaml string
		servicePath      string
		chartPath        string
		helmClient       helmclient.Client
	)

	p.Log.Infof("start helm deploy, productName %s serviceName %s containerName %v envName: %s namespace %s", p.Task.ProductName,
		p.Task.ServiceName, containerNameSet.List(), p.Task.EnvName, p.Task.Namespace)

	productInfo, err = GetProductInfo(ctx, &EnvArgs{EnvName: p.Task.EnvName, ProductName: p.Task.ProductName})
	if err != nil {
		err = errors.WithMessagef(
			err,
			"failed to get product %s/%s",
			p.Task.Namespace, p.Task.ServiceName)
		return
	}
	if productInfo.IsSleeping() {
		err = fmt.Errorf("product %s/%s is sleeping", p.Task.ProductName, p.Task.EnvName)
		return
	}

	serviceRevisionInProduct := int64(0)
	involvedImagePaths := make(map[string]*commonmodels.ImagePathSpec)
	targetContainers := make(map[string]*commonmodels.Container, 0)

	for _, service := range productInfo.GetServiceMap() {
		if service.ServiceName != p.Task.ServiceName {
			continue
		}
		serviceRender = service.Render
		serviceRevisionInProduct = service.Revision
		for _, container := range service.Containers {
			if container.ImagePath == nil {
				err = errors.WithMessagef(err, "image path of %s/%s is nil", service.ServiceName, container.Name)
				return
			}
			targetContainers[container.Name] = container
			if !containerNameSet.Has(container.Name) {
				continue
			}
			involvedImagePaths[container.Name] = container.ImagePath
		}
		break
	}

	if len(involvedImagePaths) == 0 {
		err = errors.Errorf("failed to find containers from service %s", p.Task.ServiceName)
		return
	}

	if serviceRender == nil {
		serviceRender = &template.ServiceRender{
			OverrideYaml: &template.CustomYaml{},
		}
	}

	// use revision of service currently applied in environment instead of the latest revision
	path, errDownload := p.downloadService(pipelineTask.ProductName, p.Task.ServiceName, pipelineTask.StorageURI, serviceRevisionInProduct)
	if errDownload != nil {
		p.Log.Warnf("failed to get chart of revision: %d for service: %s, use latest version",
			serviceRevisionInProduct, p.Task.ServiceName)
		path, errDownload = p.downloadService(pipelineTask.ProductName, p.Task.ServiceName,
			pipelineTask.StorageURI, 0)
		if errDownload != nil {
			err = errors.WithMessagef(
				errDownload,
				"failed to download service %s/%s",
				p.Task.Namespace, p.Task.ServiceName)
			return
		}
	}

	chartPath, err = fsutil.RelativeToCurrentPath(path)
	if err != nil {
		err = errors.WithMessagef(
			err,
			"failed to get relative path %s",
			servicePath,
		)
		return
	}

	imageKVS := make([]*helmtool.KV, 0)
	replaceValuesMaps := make([]map[string]interface{}, 0)

	for _, sPlugin := range p.ContentPlugins {
		containerName := strings.TrimSuffix(sPlugin.Task.ContainerName, "_"+p.Task.ServiceName)
		if _, ok := involvedImagePaths[containerName]; ok {
			targetContainers[containerName].Image = sPlugin.Task.Image
		}
	}

	for _, targetContainer := range targetContainers {
		// prepare image replace info
		replaceValuesMap, errAssignData := commonutil.AssignImageData(targetContainer.Image, commonutil.GetValidMatchData(targetContainer.ImagePath))
		if err != nil {
			err = errors.WithMessagef(
				errAssignData,
				"failed to assign image into values yaml %s/%s",
				p.Task.Namespace, p.Task.ServiceName)
			return
		}
		p.Log.Infof("assing image data for image: %s, assign data: %v", targetContainer.Image, replaceValuesMap)
		replaceValuesMaps = append(replaceValuesMaps, replaceValuesMap)
	}

	for _, imageSecs := range replaceValuesMaps {
		for key, value := range imageSecs {
			imageKVS = append(imageKVS, &helmtool.KV{
				Key:   key,
				Value: value,
			})
		}
	}

	// merge override values and kvs into service's yaml
	mergedValuesYaml, err = helmtool.MergeOverrideValues("", productInfo.DefaultValues, serviceRender.GetOverrideYaml(), serviceRender.OverrideValues, imageKVS)
	if err != nil {
		err = errors.WithMessagef(
			err,
			"failed to merge override values %s",
			serviceRender.OverrideValues,
		)
		return
	}
	p.Log.Infof("final minimum merged values.yaml: \n%s", mergedValuesYaml)

	helmClient, err = helmtool.NewClientFromNamespace(pipelineTask.ConfigPayload.DeployClusterID, p.Task.Namespace)
	if err != nil {
		err = errors.WithMessagef(
			err,
			"failed to create helm client %s/%s",
			p.Task.Namespace, p.Task.ServiceName)
		return
	}

	releaseName := p.Task.ReleaseName

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
		return
	}

	timeOut := p.TaskTimeout()
	chartSpec := helmclient.ChartSpec{
		ReleaseName: releaseName,
		ChartName:   chartPath,
		Namespace:   p.Task.Namespace,
		ReuseValues: true,
		Version:     serviceRender.ChartVersion,
		ValuesYaml:  mergedValuesYaml,
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
				p.Task.Namespace, p.Task.ServiceName)
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
		return
	}
}

// download chart info of specific version, use the latest version if fails
func (p *HelmDeployTaskPlugin) downloadService(productName, serviceName, storageURI string, revision int64) (string, error) {
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

// Wait ...
func (p *HelmDeployTaskPlugin) Wait(ctx context.Context) {
	// skip waiting for reset image task
	if p.Task.SkipWaiting {
		p.SetStatus(config.StatusPassed)
		return
	}

	// for services deployed by helm, use --wait option to ensure related resources are updated
	p.SetStatus(config.StatusPassed)
	return
}

func (p *HelmDeployTaskPlugin) Complete(ctx context.Context, pipelineTask *task.Task, serviceName string) {
}

func (p *HelmDeployTaskPlugin) SetTask(t map[string]interface{}) error {
	if p.Task == nil {
		task, err := ToDeployTask(t)
		if err != nil {
			return err
		}
		p.Task = task
	}
	return nil
}

// GetTask ...
func (p *HelmDeployTaskPlugin) GetTask() interface{} {
	return p.Task
}

// IsTaskDone ...
func (p *HelmDeployTaskPlugin) IsTaskDone() bool {
	if p.Task.TaskStatus != config.StatusCreated && p.Task.TaskStatus != config.StatusRunning {
		return true
	}
	return false
}

// IsTaskFailed ...
func (p *HelmDeployTaskPlugin) IsTaskFailed() bool {
	if p.Task.TaskStatus == config.StatusFailed || p.Task.TaskStatus == config.StatusTimeout || p.Task.TaskStatus == config.StatusCancelled {
		return true
	}
	return false
}

// SetStartTime ...
func (p *HelmDeployTaskPlugin) SetStartTime() {
	p.Task.StartTime = time.Now().Unix()
	for _, subPlugin := range p.ContentPlugins {
		subPlugin.Task.StartTime = p.Task.StartTime
	}
}

// SetEndTime ...
func (p *HelmDeployTaskPlugin) SetEndTime() {
	p.Task.EndTime = time.Now().Unix()
	for _, subPlugin := range p.ContentPlugins {
		subPlugin.Task.EndTime = p.Task.EndTime
	}
}

// IsTaskEnabled ...
func (p *HelmDeployTaskPlugin) IsTaskEnabled() bool {
	return p.Task.Enabled
}

// ResetError ...
func (p *HelmDeployTaskPlugin) ResetError() {
	p.Task.Error = ""
	for _, subPlugin := range p.ContentPlugins {
		subPlugin.Task.Error = ""
	}
}
