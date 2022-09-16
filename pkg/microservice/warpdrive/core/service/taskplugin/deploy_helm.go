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
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/releaseutil"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	configbase "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/warpdrive/config"
	"github.com/koderover/zadig/pkg/microservice/warpdrive/core/service/taskplugin/s3"
	"github.com/koderover/zadig/pkg/microservice/warpdrive/core/service/types"
	"github.com/koderover/zadig/pkg/microservice/warpdrive/core/service/types/task"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	helmtool "github.com/koderover/zadig/pkg/tool/helmclient"
	"github.com/koderover/zadig/pkg/tool/httpclient"
	krkubeclient "github.com/koderover/zadig/pkg/tool/kube/client"
	s3tool "github.com/koderover/zadig/pkg/tool/s3"
	"github.com/koderover/zadig/pkg/util/converter"
	fsutil "github.com/koderover/zadig/pkg/util/fs"
	yamlutil "github.com/koderover/zadig/pkg/util/yaml"
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
	} else {
		if !p.Task.IsRestart {
			p.Task.Timeout = p.Task.Timeout * 60
		}
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
		productInfo              *types.Product
		renderChart              *types.RenderChart
		replacedValuesYaml       string
		mergedValuesYaml         string
		replacedMergedValuesYaml string
		servicePath              string
		chartPath                string
		replaceValuesMap         map[string]interface{}
		renderInfo               *types.RenderSet
		helmClient               helmclient.Client
	)

	p.Log.Infof("start helm deploy, productName %s serviceName %s containerName %v envName: %s namespace %s", p.Task.ProductName,
		p.Task.ServiceName, containerNameSet.List(), p.Task.EnvName, p.Task.Namespace)

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

	serviceRevisionInProduct := int64(0)
	involvedImagePaths := make(map[string]*types.ImagePathSpec)
	for _, service := range productInfo.GetServiceMap() {
		if service.ServiceName != p.Task.ServiceName {
			continue
		}
		serviceRevisionInProduct = service.Revision
		for _, container := range service.Containers {
			if !containerNameSet.Has(container.Name) {
				continue
			}
			if container.ImagePath == nil {
				err = errors.WithMessagef(err, "image path of %s/%s is nil", service.ServiceName, container.Name)
				return
			}
			involvedImagePaths[container.Name] = container.ImagePath
		}
		break
	}

	if len(involvedImagePaths) == 0 {
		err = errors.Errorf("failed to find containers from service %s", p.Task.ServiceName)
		return
	}

	for _, chartInfo := range renderInfo.ChartInfos {
		if chartInfo.ServiceName == p.Task.ServiceName {
			renderChart = chartInfo
			break
		}
	}

	if renderChart == nil {
		err = errors.Errorf("failed to update container image in %s/%s，chart not found",
			p.Task.Namespace, p.Task.ServiceName)
		return
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

	serviceValuesYaml := renderChart.ValuesYaml
	replaceValuesMap = make(map[string]interface{})

	for _, sPlugin := range p.ContentPlugins {
		containerName := strings.TrimSuffix(sPlugin.Task.ContainerName, "_"+p.Task.ServiceName)
		if imagePath, ok := involvedImagePaths[containerName]; ok {
			validMatchData := getValidMatchData(imagePath)
			singleReplaceValuesMap, errAssign := assignImageData(sPlugin.Task.Image, validMatchData)
			if errAssign != nil {
				err = errors.WithMessagef(
					errAssign,
					"failed to pase image uri %s/%s",
					p.Task.Namespace, p.Task.ServiceName)
				return
			}
			for k, v := range singleReplaceValuesMap {
				replaceValuesMap[k] = v
			}
		}
	}

	// replace image into service's values.yaml
	replacedValuesYaml, err = replaceImage(serviceValuesYaml, replaceValuesMap)
	if err != nil {
		err = errors.WithMessagef(
			err,
			"failed to replace image uri %s/%s",
			p.Task.Namespace, p.Task.ServiceName)
		return
	}
	if replacedValuesYaml == "" {
		err = errors.Errorf("failed to set new image uri into service's values.yaml %s/%s",
			p.Task.Namespace, p.Task.ServiceName)
		return
	}

	// merge override values and kvs into service's yaml
	mergedValuesYaml, err = helmtool.MergeOverrideValues(serviceValuesYaml, renderInfo.DefaultValues, renderChart.GetOverrideYaml(), renderChart.OverrideValues)
	if err != nil {
		err = errors.WithMessagef(
			err,
			"failed to merge override values %s",
			renderChart.OverrideValues,
		)
		return
	}

	// replace image into final merged values.yaml
	replacedMergedValuesYaml, err = replaceImage(mergedValuesYaml, replaceValuesMap)
	if err != nil {
		err = errors.WithMessagef(
			err,
			"failed to replace image uri into helm values %s/%s",
			p.Task.Namespace, p.Task.ServiceName)
		return
	}
	if replacedMergedValuesYaml == "" {
		err = errors.Errorf("failed to set image uri into mreged values.yaml in %s/%s",
			p.Task.Namespace, p.Task.ServiceName)
		return
	}

	p.Log.Infof("final replaced merged values: \n%s", replacedMergedValuesYaml)

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
		Version:     renderChart.ChartVersion,
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

	//替换环境变量中的chartInfos
	for _, chartInfo := range renderInfo.ChartInfos {
		if chartInfo.ServiceName == p.Task.ServiceName {
			chartInfo.ValuesYaml = replacedValuesYaml
			break
		}
	}

	// TODO too dangerous to override entire renderset!
	err = p.updateRenderSet(ctx, &types.RenderSet{
		Name:          renderInfo.Name,
		Revision:      renderInfo.Revision,
		DefaultValues: renderInfo.DefaultValues,
		ChartInfos:    renderInfo.ChartInfos,
	})
	if err != nil {
		err = errors.WithMessagef(
			err,
			"failed to update renderset info %s/%s, renderset %s",
			p.Task.Namespace, p.Task.ServiceName, renderInfo.Name)
	}
}

func (p *HelmDeployTaskPlugin) getProductInfo(ctx context.Context, args *EnvArgs) (*types.Product, error) {
	url := fmt.Sprintf("/api/environment/environments/%s/productInfo", args.EnvName)
	prod := &types.Product{}
	_, err := p.httpClient.Get(url, httpclient.SetResult(prod), httpclient.SetQueryParam("projectName", args.ProductName), httpclient.SetQueryParam("ifPassFilter", "true"))
	if err != nil {
		return nil, err
	}
	return prod, nil
}

// download chart info of specific version, use the latest version if fails
func (p *HelmDeployTaskPlugin) downloadService(productName, serviceName, storageURI string, revision int64) (string, error) {
	logger := p.Log

	fileName := serviceName
	if revision > 0 {
		fileName = fmt.Sprintf("%s-%d", serviceName, revision)
	}
	tarball := fmt.Sprintf("%s.tar.gz", fileName)
	localBase := configbase.LocalServicePath(productName, serviceName)
	tarFilePath := filepath.Join(localBase, tarball)

	exists, err := fsutil.FileExists(tarFilePath)
	if err != nil {
		return "", err
	}
	if exists {
		return tarFilePath, nil
	}

	s3Storage, err := s3.NewS3StorageFromEncryptedURI(storageURI)
	if err != nil {
		return "", err
	}

	s3Storage.Subfolder = filepath.Join(s3Storage.Subfolder, configbase.ObjectStorageServicePath(productName, serviceName))
	forcedPathStyle := true
	if s3Storage.Provider == setting.ProviderSourceAli {
		forcedPathStyle = false
	}
	s3Client, err := s3tool.NewClient(s3Storage.Endpoint, s3Storage.Ak, s3Storage.Sk, s3Storage.Insecure, forcedPathStyle)
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

func (p *HelmDeployTaskPlugin) getRenderSet(ctx context.Context, name string, revision int64) (*types.RenderSet, error) {
	url := fmt.Sprintf("/api/project/renders/render/%s/revision/%d", name, revision)
	rs := &types.RenderSet{}
	_, err := p.httpClient.Get(url, httpclient.SetResult(rs), httpclient.SetQueryParam("ifPassFilter", "true"))
	if err != nil {
		return nil, err
	}
	return rs, nil
}

func getValidMatchData(spec *types.ImagePathSpec) map[string]string {
	ret := make(map[string]string)
	if spec.Repo != "" {
		ret[setting.PathSearchComponentRepo] = spec.Repo
	}
	if spec.Image != "" {
		ret[setting.PathSearchComponentImage] = spec.Image
	}
	if spec.Tag != "" {
		ret[setting.PathSearchComponentTag] = spec.Tag
	}
	return ret
}

// replace image defines in yaml by new version
func replaceImage(sourceYaml string, imageValuesMap map[string]interface{}) (string, error) {
	nestedMap, err := converter.Expand(imageValuesMap)
	if err != nil {
		return "", err
	}
	bs, err := yaml.Marshal(nestedMap)
	if err != nil {
		return "", err
	}
	mergedBs, err := yamlutil.Merge([][]byte{[]byte(sourceYaml), bs})
	if err != nil {
		return "", err
	}
	return string(mergedBs), nil
}

// parse image url to map: repo=>xxx/xx/xx image=>xx tag=>xxx
func resolveImageUrl(imageUrl string) map[string]string {
	subMatchAll := imageParseRegex.FindStringSubmatch(imageUrl)
	result := make(map[string]string)
	exNames := imageParseRegex.SubexpNames()
	for i, matchedStr := range subMatchAll {
		if i != 0 && matchedStr != "" && matchedStr != ":" {
			result[exNames[i]] = matchedStr
		}
	}
	return result
}

// assignImageData assign image url data into match data
// matchData: image=>absolute-path repo=>absolute-path tag=>absolute-path
// return: absolute-image-path=>image-value  absolute-repo-path=>repo-value absolute-tag-path=>tag-value
func assignImageData(imageUrl string, matchData map[string]string) (map[string]interface{}, error) {
	ret := make(map[string]interface{})
	// total image url assigned into one single value
	if len(matchData) == 1 {
		for _, v := range matchData {
			ret[v] = imageUrl
		}
		return ret, nil
	}

	resolvedImageUrl := resolveImageUrl(imageUrl)

	// image url assigned into repo/image+tag
	if len(matchData) == 3 {
		ret[matchData[setting.PathSearchComponentRepo]] = strings.TrimSuffix(resolvedImageUrl[setting.PathSearchComponentRepo], "/")
		ret[matchData[setting.PathSearchComponentImage]] = resolvedImageUrl[setting.PathSearchComponentImage]
		ret[matchData[setting.PathSearchComponentTag]] = resolvedImageUrl[setting.PathSearchComponentTag]
		return ret, nil
	}

	if len(matchData) == 2 {
		// image url assigned into repo/image + tag
		if tagPath, ok := matchData[setting.PathSearchComponentTag]; ok {
			ret[tagPath] = resolvedImageUrl[setting.PathSearchComponentTag]
			for k, imagePath := range matchData {
				if k == setting.PathSearchComponentTag {
					continue
				}
				ret[imagePath] = fmt.Sprintf("%s%s", resolvedImageUrl[setting.PathSearchComponentRepo], resolvedImageUrl[setting.PathSearchComponentImage])
				break
			}
			return ret, nil
		}
		// image url assigned into repo + image(tag)
		ret[matchData[setting.PathSearchComponentRepo]] = strings.TrimSuffix(resolvedImageUrl[setting.PathSearchComponentRepo], "/")
		ret[matchData[setting.PathSearchComponentImage]] = fmt.Sprintf("%s:%s", resolvedImageUrl[setting.PathSearchComponentImage], resolvedImageUrl[setting.PathSearchComponentTag])
		return ret, nil
	}

	return nil, errors.Errorf("match data illegal, expect length: 1-3, actual length: %d", len(matchData))
}

func (p *HelmDeployTaskPlugin) updateRenderSet(ctx context.Context, args *types.RenderSet) error {
	url := "/api/project/renders"
	_, err := p.httpClient.Put(url, httpclient.SetBody(args), httpclient.SetQueryParam("ifPassFilter", "true"))
	return err
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
