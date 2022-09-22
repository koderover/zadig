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
	"path/filepath"
	"regexp"
	"strings"
	"time"

	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	helmtool "github.com/koderover/zadig/pkg/tool/helmclient"
	s3tool "github.com/koderover/zadig/pkg/tool/s3"
	"github.com/koderover/zadig/pkg/util/converter"
	fsutil "github.com/koderover/zadig/pkg/util/fs"
	yamlutil "github.com/koderover/zadig/pkg/util/yaml"
	helmclient "github.com/mittwald/go-helm-client"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/releaseutil"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/rest"
	crClient "sigs.k8s.io/controller-runtime/pkg/client"

	configbase "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	templatemodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/s3"
	"github.com/koderover/zadig/pkg/setting"
)

const (
	imageUrlParseRegexString = `(?P<repo>.+/)?(?P<image>[^:]+){1}(:)?(?P<tag>.+)?`
)

var (
	imageParseRegex = regexp.MustCompile(imageUrlParseRegexString)
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
	env, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    c.workflowCtx.ProjectName,
		EnvName: c.jobTaskSpec.Env,
	})
	if err != nil {
		msg := fmt.Sprintf("find project %s error: %v", c.workflowCtx.ProjectName, err)
		logError(c.job, msg, c.logger)
		return
	}
	c.namespace = env.Namespace
	c.jobTaskSpec.ClusterID = env.ClusterID

	c.restConfig, err = kubeclient.GetRESTConfig(config.HubServerAddress(), c.jobTaskSpec.ClusterID)
	if err != nil {
		msg := fmt.Sprintf("can't get k8s rest config: %v", err)
		logError(c.job, msg, c.logger)
		return
	}

	c.kubeClient, err = kubeclient.GetKubeClient(config.HubServerAddress(), c.jobTaskSpec.ClusterID)
	if err != nil {
		msg := fmt.Sprintf("can't init k8s client: %v", err)
		logError(c.job, msg, c.logger)
		return
	}

	// all involved containers
	containerNameSet := sets.NewString()
	for _, svcAndContainer := range c.jobTaskSpec.ImageAndModules {
		singleContainerName := strings.TrimSuffix(svcAndContainer.ServiceModule, "_"+c.jobTaskSpec.ServiceName)
		containerNameSet.Insert(singleContainerName)
	}

	var (
		productInfo              *commonmodels.Product
		renderChart              *templatemodels.RenderChart
		replacedValuesYaml       string
		mergedValuesYaml         string
		replacedMergedValuesYaml string
		servicePath              string
		chartPath                string
		replaceValuesMap         map[string]interface{}
		renderInfo               *commonmodels.RenderSet
		helmClient               helmclient.Client
	)

	c.logger.Infof("start helm deploy, productName %s serviceName %s containerName %v namespace %s", c.workflowCtx.ProjectName,
		c.jobTaskSpec.ServiceName, containerNameSet.List(), c.namespace)

	productInfo, err = commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{Name: c.workflowCtx.ProjectName, EnvName: c.jobTaskSpec.Env})
	if err != nil {
		err = errors.WithMessagef(
			err,
			"failed to get product %s/%s",
			c.namespace, c.jobTaskSpec.ServiceName)
		logError(c.job, err.Error(), c.logger)
		return
	}

	renderInfo, err = commonrepo.NewRenderSetColl().Find(&commonrepo.RenderSetFindOption{Name: productInfo.Render.Name, Revision: productInfo.Render.Revision})
	if err != nil {
		err = errors.WithMessagef(
			err,
			"failed to get getRenderSet %s/%d",
			productInfo.Render.Name, productInfo.Render.Revision)
		logError(c.job, err.Error(), c.logger)
		return
	}

	serviceRevisionInProduct := int64(0)
	involvedImagePaths := make(map[string]*commonmodels.ImagePathSpec)
	for _, service := range productInfo.GetServiceMap() {
		if service.ServiceName != c.jobTaskSpec.ServiceName {
			continue
		}
		serviceRevisionInProduct = service.Revision
		for _, container := range service.Containers {
			if !containerNameSet.Has(container.Name) {
				continue
			}
			if container.ImagePath == nil {
				err = errors.WithMessagef(err, "image path of %s/%s is nil", service.ServiceName, container.Name)
				logError(c.job, err.Error(), c.logger)
				return
			}
			involvedImagePaths[container.Name] = container.ImagePath
		}
		break
	}

	if len(involvedImagePaths) == 0 {
		err = errors.Errorf("failed to find containers from service %s", c.jobTaskSpec.ServiceName)
		logError(c.job, err.Error(), c.logger)
		return
	}

	for _, chartInfo := range renderInfo.ChartInfos {
		if chartInfo.ServiceName == c.jobTaskSpec.ServiceName {
			renderChart = chartInfo
			break
		}
	}

	if renderChart == nil {
		err = errors.Errorf("failed to update container image in %s/%s,chart not found",
			c.namespace, c.jobTaskSpec.ServiceName)
		logError(c.job, err.Error(), c.logger)
		return
	}

	defaultS3, err := s3.FindDefaultS3()
	if err != nil {
		logError(c.job, err.Error(), c.logger)
		return
	}

	defaultURL, err := defaultS3.GetEncryptedURL()
	if err != nil {
		logError(c.job, err.Error(), c.logger)
		return
	}

	// use revision of service currently applied in environment instead of the latest revision
	path, errDownload := c.downloadService(c.workflowCtx.ProjectName, c.jobTaskSpec.ServiceName, defaultURL, serviceRevisionInProduct)
	if errDownload != nil {
		c.logger.Warnf("failed to get chart of revision: %d for service: %s, use latest version",
			serviceRevisionInProduct, c.jobTaskSpec.ServiceName)
		path, errDownload = c.downloadService(c.workflowCtx.ProjectName, c.jobTaskSpec.ServiceName,
			defaultURL, 0)
		if errDownload != nil {
			err = errors.WithMessagef(
				errDownload,
				"failed to download service %s/%s",
				c.namespace, c.jobTaskSpec.ServiceName)
			logError(c.job, err.Error(), c.logger)
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
		logError(c.job, err.Error(), c.logger)
		return
	}

	serviceValuesYaml := renderChart.ValuesYaml
	replaceValuesMap = make(map[string]interface{})

	for _, svcAndContainer := range c.jobTaskSpec.ImageAndModules {
		containerName := strings.TrimSuffix(svcAndContainer.ServiceModule, "_"+c.jobTaskSpec.ServiceName)
		if imagePath, ok := involvedImagePaths[containerName]; ok {
			validMatchData := getValidMatchData(imagePath)
			singleReplaceValuesMap, errAssign := assignImageData(svcAndContainer.Image, validMatchData)
			if errAssign != nil {
				err = errors.WithMessagef(
					errAssign,
					"failed to pase image uri %s/%s",
					c.namespace, c.jobTaskSpec.ServiceName)
				logError(c.job, err.Error(), c.logger)
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
			c.namespace, c.jobTaskSpec.ServiceName)
		logError(c.job, err.Error(), c.logger)
		return
	}
	if replacedValuesYaml == "" {
		err = errors.Errorf("failed to set new image uri into service's values.yaml %s/%s",
			c.namespace, c.jobTaskSpec.ServiceName)
		logError(c.job, err.Error(), c.logger)
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
		logError(c.job, err.Error(), c.logger)
		return
	}

	// replace image into final merged values.yaml
	replacedMergedValuesYaml, err = replaceImage(mergedValuesYaml, replaceValuesMap)
	if err != nil {
		err = errors.WithMessagef(
			err,
			"failed to replace image uri into helm values %s/%s",
			c.namespace, c.jobTaskSpec.ServiceName)
		logError(c.job, err.Error(), c.logger)
		return
	}
	if replacedMergedValuesYaml == "" {
		err = errors.Errorf("failed to set image uri into mreged values.yaml in %s/%s",
			c.namespace, c.jobTaskSpec.ServiceName)
		logError(c.job, err.Error(), c.logger)
		return
	}

	c.logger.Infof("final replaced merged values: \n%s", replacedMergedValuesYaml)

	helmClient, err = helmtool.NewClientFromNamespace(c.jobTaskSpec.ClusterID, c.namespace)
	if err != nil {
		err = errors.WithMessagef(
			err,
			"failed to create helm client %s/%s",
			c.namespace, c.jobTaskSpec.ServiceName)
		logError(c.job, err.Error(), c.logger)
		return
	}

	releaseName := c.jobTaskSpec.ReleaseName

	ensureUpgrade := func() error {
		hrs, errHistory := helmClient.ListReleaseHistory(releaseName, 10)
		if errHistory != nil {
			// list history should not block deploy operation, error will be logged instead of returned
			c.logger.Errorf("failed to list release history, release: %s, err: %s", releaseName, errHistory)
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
		logError(c.job, err.Error(), c.logger)
		return
	}

	timeOut := c.timeout()
	chartSpec := helmclient.ChartSpec{
		ReleaseName: releaseName,
		ChartName:   chartPath,
		Namespace:   c.namespace,
		ReuseValues: true,
		Version:     renderChart.ChartVersion,
		ValuesYaml:  replacedMergedValuesYaml,
		SkipCRDs:    false,
		UpgradeCRDs: true,
		Timeout:     time.Second * time.Duration(timeOut),
		Wait:        !c.jobTaskSpec.SkipCheckRunStatus,
		Replace:     true,
		MaxHistory:  10,
	}
	c.logger.Infof("start to upgrade helm chart, release name: %s, chart name: %s, version: %s", chartSpec.ReleaseName, chartSpec.ChartName, chartSpec.Version)
	done := make(chan bool)
	go func(chan bool) {
		if _, err = helmClient.InstallOrUpgradeChart(ctx, &chartSpec, nil); err != nil {
			err = errors.WithMessagef(
				err,
				"failed to upgrade helm chart %s/%s",
				c.namespace, c.jobTaskSpec.ServiceName)
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
		logError(c.job, err.Error(), c.logger)
		return
	}

	//替换环境变量中的chartInfos
	for _, chartInfo := range renderInfo.ChartInfos {
		if chartInfo.ServiceName == c.jobTaskSpec.ServiceName {
			chartInfo.ValuesYaml = replacedValuesYaml
			break
		}
	}

	// TODO too dangerous to override entire renderset!
	if err := commonrepo.NewRenderSetColl().Update(&commonmodels.RenderSet{
		Name:          renderInfo.Name,
		Revision:      renderInfo.Revision,
		DefaultValues: renderInfo.DefaultValues,
		ChartInfos:    renderInfo.ChartInfos,
	}); err != nil {
		err = errors.WithMessagef(
			err,
			"failed to update renderset info %s/%s, renderset %s",
			c.namespace, c.jobTaskSpec.ServiceName, renderInfo.Name)
		logError(c.job, err.Error(), c.logger)
		return
	}
	c.job.Status = config.StatusPassed
}

func (c *HelmDeployJobCtl) timeout() int {
	if c.jobTaskSpec.Timeout == 0 {
		c.jobTaskSpec.Timeout = setting.DeployTimeout
	}
	return c.jobTaskSpec.Timeout
}

// download chart info of specific version, use the latest version if fails
func (c *HelmDeployJobCtl) downloadService(productName, serviceName, storageURI string, revision int64) (string, error) {
	logger := c.logger

	fileName := serviceName
	if revision > 0 {
		fileName = fmt.Sprintf("%s-%d", serviceName, revision)
	}
	tarball := fmt.Sprintf("%s.tar.gz", fileName)
	localBase := configbase.LocalWorkflowServicePath(productName, serviceName)
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

	s3Storage.Subfolder = filepath.Join(s3Storage.Subfolder, config.ObjectStorageServicePath(productName, serviceName))
	forcedPathStyle := true
	if s3Storage.Provider == setting.ProviderSourceAli {
		forcedPathStyle = false
	}
	s3Client, err := s3tool.NewClient(s3Storage.Endpoint, s3Storage.Ak, s3Storage.Sk, s3Storage.Insecure, forcedPathStyle)
	if err != nil {
		c.logger.Errorf("failed to create s3 client, err: %s", err)
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

func getValidMatchData(spec *commonmodels.ImagePathSpec) map[string]string {
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
