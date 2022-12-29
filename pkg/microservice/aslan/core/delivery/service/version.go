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

package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/blang/semver/v4"
	"github.com/chartmuseum/helm-push/pkg/helm"
	"github.com/hashicorp/go-multierror"
	"github.com/otiai10/copy"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
	chartloader "helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/repo"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/yaml"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/task"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/base"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/kube"
	workflowservice "github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/service/workflow"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
	helmtool "github.com/koderover/zadig/pkg/tool/helmclient"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/types"
	"github.com/koderover/zadig/pkg/util"
	"github.com/koderover/zadig/pkg/util/converter"
	fsutil "github.com/koderover/zadig/pkg/util/fs"
	yamlutil "github.com/koderover/zadig/pkg/util/yaml"
)

const (
	VerbosityBrief    string = "brief"    // brief delivery data
	VerbosityDetailed string = "detailed" // detailed delivery version with total data
)

type DeliveryVersionFilter struct {
	ServiceName string
}

type CreateHelmDeliveryVersionOption struct {
	EnableOfflineDist bool   `json:"enableOfflineDist"`
	S3StorageID       string `json:"s3StorageID"`
}

type ImageData struct {
	ImageName string `json:"imageName"`
	ImageTag  string `json:"imageTag"`
	Selected  bool   `json:"selected"`
}

type CreateHelmDeliveryVersionChartData struct {
	ServiceName       string       `json:"serviceName"`
	Version           string       `json:"version,omitempty"`
	ValuesYamlContent string       `json:"valuesYamlContent"`
	ImageData         []*ImageData `json:"imageData"`
}

type CreateHelmDeliveryVersionArgs struct {
	CreateBy      string   `json:"-"`
	ProductName   string   `json:"productName"`
	Retry         bool     `json:"retry"`
	Version       string   `json:"version"`
	Desc          string   `json:"desc"`
	EnvName       string   `json:"envName"`
	Labels        []string `json:"labels"`
	ImageRepoName string   `json:"imageRepoName"`
	*DeliveryVersionChartData
}

type DeliveryVersionChartData struct {
	GlobalVariables string                                `json:"globalVariables"`
	ChartRepoName   string                                `json:"chartRepoName"`
	ImageRegistryID string                                `json:"imageRegistryID"`
	ChartDatas      []*CreateHelmDeliveryVersionChartData `json:"chartDatas"`
	Options         *CreateHelmDeliveryVersionOption      `json:"options"`
}

type DeliveryChartData struct {
	ChartData      *CreateHelmDeliveryVersionChartData
	ServiceObj     *commonmodels.Service
	ProductService *commonmodels.ProductService
	RenderChart    *template.ServiceRender
	RenderSet      *commonmodels.RenderSet
	ValuesInEnv    map[string]interface{}
}

type DeliveryChartResp struct {
	FileInfos []*types.FileInfo `json:"fileInfos"`
}

type DeliveryChartFilePathArgs struct {
	Dir         string `json:"dir"`
	ProjectName string `json:"projectName"`
	ChartName   string `json:"chartName"`
	Version     string `json:"version"`
}

type DeliveryChartFileContentArgs struct {
	FilePath    string `json:"filePath"`
	FileName    string `json:"fileName"`
	ProjectName string `json:"projectName"`
	ChartName   string `json:"chartName"`
	Version     string `json:"version"`
}

type DeliveryVariablesApplyArgs struct {
	GlobalVariables string                                `json:"globalVariables,omitempty"`
	ChartDatas      []*CreateHelmDeliveryVersionChartData `json:"chartDatas"`
}

type ListDeliveryVersionArgs struct {
	Page         int    `form:"page"`
	PerPage      int    `form:"per_page"`
	TaskId       int    `form:"taskId"`
	ServiceName  string `form:"serviceName"`
	Verbosity    string `form:"verbosity"`
	ProjectName  string `form:"projectName"`
	WorkflowName string `form:"workflowName"`
}

type ReleaseInfo struct {
	VersionInfo    *commonmodels.DeliveryVersion      `json:"versionInfo"`
	BuildInfo      []*commonmodels.DeliveryBuild      `json:"buildInfo,omitempty"`
	DeployInfo     []*commonmodels.DeliveryDeploy     `json:"deployInfo,omitempty"`
	TestInfo       []*commonmodels.DeliveryTest       `json:"testInfo,omitempty"`
	DistributeInfo []*commonmodels.DeliveryDistribute `json:"distributeInfo,omitempty"`
	SecurityInfo   []*DeliverySecurityStats           `json:"securityStatsInfo,omitempty"`
}

type DeliverySecurityStatsInfo struct {
	Total      int `json:"total"`
	Unknown    int `json:"unkown"`
	Negligible int `json:"negligible"`
	Low        int `json:"low"`
	Medium     int `json:"medium"`
	High       int `json:"high"`
	Critical   int `json:"critical"`
}

type DeliverySecurityStats struct {
	ImageName                 string                    `json:"imageName"`
	ImageID                   string                    `json:"imageId"`
	DeliverySecurityStatsInfo DeliverySecurityStatsInfo `json:"deliverySecurityStatsInfo"`
}

type ImageUrlDetail struct {
	ImageUrl  string
	Name      string
	Registry  string
	Tag       string
	CustomTag string
}

type ServiceImageDetails struct {
	ServiceName string
	Images      []*ImageUrlDetail
	Registries  []string
}

type ChartVersionResp struct {
	ChartName        string `json:"chartName"`
	ChartVersion     string `json:"chartVersion"`
	NextChartVersion string `json:"nextChartVersion"`
	Url              string `json:"url"`
}

type DeliveryVersionPayloadImage struct {
	ServiceModule string `json:"service_module"`
	Image         string `json:"image"`
}

type DeliveryVersionPayloadChart struct {
	ChartName    string                         `json:"chart_name"`
	ChartVersion string                         `json:"chart_version"`
	ChartUrl     string                         `json:"chart_url"`
	Images       []*DeliveryVersionPayloadImage `json:"images"`
}

type DeliveryVersionHookPayload struct {
	ProjectName string                         `json:"project_name"`
	Version     string                         `json:"version"`
	Status      string                         `json:"status"`
	Error       string                         `json:"error"`
	StartTime   int64                          `json:"start_time"`
	EndTime     int64                          `json:"end_time"`
	Charts      []*DeliveryVersionPayloadChart `json:"charts"`
}

func GetDeliveryVersion(args *commonrepo.DeliveryVersionArgs, log *zap.SugaredLogger) (*commonmodels.DeliveryVersion, error) {
	versionData, err := commonrepo.NewDeliveryVersionColl().Get(args)
	if err != nil {
		log.Errorf("get deliveryVersion error: %v", err)
		return nil, e.ErrGetDeliveryVersion
	}
	return versionData, err
}

func GetDetailReleaseData(args *commonrepo.DeliveryVersionArgs, log *zap.SugaredLogger) (*ReleaseInfo, error) {
	versionData, err := commonrepo.NewDeliveryVersionColl().Get(args)
	if err != nil {
		log.Errorf("get deliveryVersion error: %v", err)
		return nil, e.ErrGetDeliveryVersion
	}
	return buildListReleaseResp(VerbosityDetailed, versionData, nil, log)
}

func FindDeliveryVersion(args *commonrepo.DeliveryVersionArgs, log *zap.SugaredLogger) ([]*commonmodels.DeliveryVersion, error) {
	resp, err := commonrepo.NewDeliveryVersionColl().Find(args)
	if err != nil {
		log.Errorf("find deliveryVersion error: %v", err)
		return resp, e.ErrFindDeliveryVersion
	}
	return resp, err
}

func DeleteDeliveryVersion(args *commonrepo.DeliveryVersionArgs, log *zap.SugaredLogger) error {
	err := commonrepo.NewDeliveryVersionColl().Delete(args.ID)
	if err != nil {
		log.Errorf("delete deliveryVersion error: %v", err)
		return e.ErrDeleteDeliveryVersion
	}
	return nil
}

func filterReleases(filter *DeliveryVersionFilter, deliveryVersion *commonmodels.DeliveryVersion, logger *zap.SugaredLogger) bool {
	if filter == nil {
		return true
	}
	if filter.ServiceName != "" {
		deliveryDeployArgs := new(commonrepo.DeliveryDeployArgs)
		deliveryDeployArgs.ReleaseID = deliveryVersion.ID.Hex()
		deliveryDeploys, err := FindDeliveryDeploy(deliveryDeployArgs, logger)
		if err != nil {
			return true
		}
		match := false
		for _, deliveryDeploy := range deliveryDeploys {
			if deliveryDeploy.ServiceName == filter.ServiceName {
				match = true
				break
			}
		}
		if !match {
			return false
		}
	}
	return true
}

func buildBriefRelease(deliveryVersion *commonmodels.DeliveryVersion, _ *zap.SugaredLogger) (*ReleaseInfo, error) {
	return &ReleaseInfo{
		VersionInfo: deliveryVersion,
	}, nil
}

func buildDetailedRelease(deliveryVersion *commonmodels.DeliveryVersion, filterOpt *DeliveryVersionFilter, logger *zap.SugaredLogger) (*ReleaseInfo, error) {
	releaseInfo := new(ReleaseInfo)
	//versionInfo
	releaseInfo.VersionInfo = deliveryVersion

	//deployInfo
	deliveryDeployArgs := new(commonrepo.DeliveryDeployArgs)
	deliveryDeployArgs.ReleaseID = deliveryVersion.ID.Hex()
	deliveryDeploys, err := FindDeliveryDeploy(deliveryDeployArgs, logger)
	if err != nil {
		return nil, err
	}
	if filterOpt != nil {
		if !filterReleases(filterOpt, deliveryVersion, logger) {
			return nil, nil
		}
	}
	// 将serviceName替换为服务名/服务组件的形式，用于前端展示
	for _, deliveryDeploy := range deliveryDeploys {
		if deliveryDeploy.ContainerName != "" {
			deliveryDeploy.ServiceName = deliveryDeploy.ServiceName + "/" + deliveryDeploy.ContainerName
		}
	}
	releaseInfo.DeployInfo = deliveryDeploys

	//buildInfo
	deliveryBuildArgs := new(commonrepo.DeliveryBuildArgs)
	deliveryBuildArgs.ReleaseID = deliveryVersion.ID.Hex()
	deliveryBuilds, err := FindDeliveryBuild(deliveryBuildArgs, logger)
	if err != nil {
		return nil, err
	}
	releaseInfo.BuildInfo = deliveryBuilds

	//testInfo
	deliveryTestArgs := new(commonrepo.DeliveryTestArgs)
	deliveryTestArgs.ReleaseID = deliveryVersion.ID.Hex()
	deliveryTests, err := FindDeliveryTest(deliveryTestArgs, logger)
	if err != nil {
		return nil, err
	}
	releaseInfo.TestInfo = deliveryTests

	//securityStatsInfo
	deliverySecurityStatss := make([]*DeliverySecurityStats, 0)
	if pipelineTask, err := workflowservice.GetPipelineTaskV2(int64(deliveryVersion.TaskID), deliveryVersion.WorkflowName, config.WorkflowType, logger); err == nil {
		for _, subStage := range pipelineTask.Stages {
			if subStage.TaskType == config.TaskSecurity {
				subSecurityTaskMap := subStage.SubTasks
				for _, subTask := range subSecurityTaskMap {
					securityInfo, _ := base.ToSecurityTask(subTask)

					deliverySecurityStats := new(DeliverySecurityStats)
					deliverySecurityStats.ImageName = securityInfo.ImageName
					deliverySecurityStats.ImageID = securityInfo.ImageID
					deliverySecurityStatsMap, err := FindDeliverySecurityStatistics(securityInfo.ImageID, logger)
					if err != nil {
						return nil, err
					}
					var transErr error
					b, err := json.Marshal(deliverySecurityStatsMap)
					if err != nil {
						transErr = fmt.Errorf("marshal task error: %v", err)
					}
					if err := json.Unmarshal(b, &deliverySecurityStats.DeliverySecurityStatsInfo); err != nil {
						transErr = fmt.Errorf("unmarshal task error: %v", err)
					}
					if transErr != nil {
						return nil, transErr
					}

					deliverySecurityStatss = append(deliverySecurityStatss, deliverySecurityStats)
				}
				break
			}
		}
		releaseInfo.SecurityInfo = deliverySecurityStatss
	}

	//distributeInfo
	deliveryDistributeArgs := new(commonrepo.DeliveryDistributeArgs)
	deliveryDistributeArgs.ReleaseID = deliveryVersion.ID.Hex()
	deliveryDistributes, _ := FindDeliveryDistribute(deliveryDistributeArgs, logger)
	releaseInfo.DistributeInfo = deliveryDistributes

	// fill some data for helm delivery releases
	processReleaseRespData(releaseInfo)

	return releaseInfo, nil
}

func buildListReleaseResp(verbosity string, deliveryVersion *commonmodels.DeliveryVersion, filterOpt *DeliveryVersionFilter, logger *zap.SugaredLogger) (*ReleaseInfo, error) {
	switch verbosity {
	case VerbosityBrief:
		return buildBriefRelease(deliveryVersion, logger)
	case VerbosityDetailed:
		return buildDetailedRelease(deliveryVersion, filterOpt, logger)
	default:
		return buildDetailedRelease(deliveryVersion, filterOpt, logger)
	}
}

func ListDeliveryVersion(args *ListDeliveryVersionArgs, logger *zap.SugaredLogger) ([]*ReleaseInfo, error) {
	versionListArgs := new(commonrepo.DeliveryVersionArgs)
	versionListArgs.ProductName = args.ProjectName
	versionListArgs.WorkflowName = args.WorkflowName
	versionListArgs.TaskID = args.TaskId
	versionListArgs.PerPage = args.PerPage
	versionListArgs.Page = args.Page
	deliveryVersions, err := FindDeliveryVersion(versionListArgs, logger)
	if err != nil {
		return nil, err
	}
	names := []string{}
	for _, version := range deliveryVersions {
		if version.WorkflowName == "" {
			continue
		}
		names = append(names, version.WorkflowName)
	}
	workflows, err := commonrepo.NewWorkflowColl().List(&commonrepo.ListWorkflowOption{Projects: []string{args.ProjectName}, Names: names})
	if err != nil {
		return nil, err
	}
	displayNameMap := map[string]string{}
	for _, workflow := range workflows {
		displayNameMap[workflow.Name] = workflow.DisplayName
	}
	for _, version := range deliveryVersions {
		if name, ok := displayNameMap[version.WorkflowName]; ok {
			version.WorkflowDisplayName = name
		}
	}
	releaseInfos := make([]*ReleaseInfo, 0)
	for _, deliveryVersion := range deliveryVersions {
		releaseInfo, err := buildListReleaseResp(args.Verbosity, deliveryVersion, &DeliveryVersionFilter{ServiceName: args.ServiceName}, logger)
		if err != nil {
			return nil, err
		}
		if releaseInfo == nil {
			continue
		}
		releaseInfos = append(releaseInfos, releaseInfo)
	}
	return releaseInfos, nil
}

// fill release
func processReleaseRespData(release *ReleaseInfo) {
	if release.VersionInfo.Type != setting.DeliveryVersionTypeChart {
		return
	}

	distributeImageMap := make(map[string][]*commonmodels.DeliveryDistribute)
	for _, distributeImage := range release.DistributeInfo {
		if distributeImage.DistributeType != config.Image {
			continue
		}
		distributeImageMap[distributeImage.ChartName] = append(distributeImageMap[distributeImage.ChartName], distributeImage)
	}

	chartDistributeCount := 0
	distributes := make([]*commonmodels.DeliveryDistribute, 0)
	for _, distribute := range release.DistributeInfo {
		if distribute.DistributeType == config.Image {
			continue
		}
		switch distribute.DistributeType {
		case config.Chart:
			chartDistributeCount++
			distribute.SubDistributes = distributeImageMap[distribute.ChartName]
		case config.File:
			s3Storage, err := commonrepo.NewS3StorageColl().Find(distribute.S3StorageID)
			if err != nil {
				log.Errorf("failed to query s3 storageID: %s, err: %s", distribute.S3StorageID, err)
			} else {
				distribute.StorageURL = s3Storage.Endpoint
				distribute.StorageBucket = s3Storage.Bucket
			}
		}
		distributes = append(distributes, distribute)
	}
	release.DistributeInfo = distributes

	release.VersionInfo.Progress = buildDeliveryProgressInfo(release.VersionInfo, chartDistributeCount)
}

func buildDeliveryProgressInfo(deliveryVersion *commonmodels.DeliveryVersion, successfulChartCount int) *commonmodels.DeliveryVersionProgress {
	if deliveryVersion.Type != setting.DeliveryVersionTypeChart {
		return nil
	}

	_, err := checkVersionStatus(deliveryVersion)
	if err != nil {
		updateVersionStatus(deliveryVersion.Version, deliveryVersion.ProductName, setting.DeliveryVersionStatusFailed, err.Error())
	}

	progress := &commonmodels.DeliveryVersionProgress{
		SuccessChartCount:   successfulChartCount,
		TotalChartCount:     0,
		PackageUploadStatus: "",
		Error:               "",
	}
	if deliveryVersion.Status == setting.DeliveryVersionStatusSuccess {
		progress.TotalChartCount = successfulChartCount
		progress.PackageUploadStatus = setting.DeliveryVersionPackageStatusSuccess
		return progress
	}

	argsBytes, err := json.Marshal(deliveryVersion.CreateArgument)
	if err != nil {
		log.Errorf("failed to marshal arguments, versionName: %s err %s", deliveryVersion.Version, err)
		return progress
	}
	createArgs := new(DeliveryVersionChartData)
	err = json.Unmarshal(argsBytes, createArgs)
	if err != nil {
		log.Errorf("failed to unMarshal arguments, versionName: %s err %s", deliveryVersion.Version, err)
		return progress
	}

	progress.TotalChartCount = len(createArgs.ChartDatas)

	if deliveryVersion.Status == setting.DeliveryVersionStatusFailed {
		progress.PackageUploadStatus = setting.DeliveryVersionPackageStatusFailed
		progress.Error = deliveryVersion.Error
		return progress
	}

	if len(createArgs.ChartDatas) > successfulChartCount {
		progress.PackageUploadStatus = setting.DeliveryVersionPackageStatusWaiting
		return progress
	}

	progress.PackageUploadStatus = setting.DeliveryVersionPackageStatusUploading
	return progress

}

func getChartTGZDir(productName, versionName string) string {
	tmpDir := os.TempDir()
	return filepath.Join(tmpDir, "chart-tgz", productName, versionName)
}

func getChartExpandDir(productName, versionName string) string {
	tmpDir := os.TempDir()
	return filepath.Join(tmpDir, "chart", productName, versionName)
}

func getProductEnvInfo(productName, envName string) (*commonmodels.Product, error) {
	productInfo, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    productName,
		EnvName: envName,
	})
	if err != nil {
		log.Errorf("failed to query product info, productName: %s envName: %s err: %s", productName, envName, err)
		return nil, fmt.Errorf("failed to query product info, productName: %s envName: %s", productName, envName)
	}
	return productInfo, nil
}

func getChartRepoData(repoName string) (*commonmodels.HelmRepo, error) {
	return commonrepo.NewHelmRepoColl().Find(&commonrepo.HelmRepoFindOption{RepoName: repoName})
}

// ensure chart files exist
func ensureChartFiles(chartData *DeliveryChartData, prod *commonmodels.Product) (string, error) {
	serviceObj := chartData.ServiceObj
	revisionBasePath := config.LocalDeliveryChartPathWithRevision(serviceObj.ProductName, serviceObj.ServiceName, serviceObj.Revision)
	deliveryChartPath := filepath.Join(revisionBasePath, serviceObj.ServiceName)
	if exists, _ := fsutil.DirExists(deliveryChartPath); exists {
		return deliveryChartPath, nil
	}

	serviceName, revision := serviceObj.ServiceName, serviceObj.Revision
	basePath := config.LocalServicePathWithRevision(serviceObj.ProductName, serviceName, revision)
	if err := commonservice.PreloadServiceManifestsByRevision(basePath, serviceObj); err != nil {
		log.Warnf("failed to get chart of revision: %d for service: %s, use latest version", revision, serviceName)
		// use the latest version when it fails to download the specific version
		basePath = config.LocalServicePath(serviceObj.ProductName, serviceName)
		if err = commonservice.PreLoadServiceManifests(basePath, serviceObj); err != nil {
			log.Errorf("failed to load chart info for service %v", serviceObj.ServiceName)
			return "", err
		}
	}

	fullPath := filepath.Join(basePath, serviceObj.ServiceName)
	err := copy.Copy(fullPath, deliveryChartPath)
	if err != nil {
		return "", err
	}

	restConfig, err := kube.GetRESTConfig(prod.ClusterID)
	if err != nil {
		log.Errorf("get rest config error: %s", err)
		return "", err
	}
	helmClient, err := helmtool.NewClientFromRestConf(restConfig, prod.Namespace)
	if err != nil {
		log.Errorf("[%s][%s] init helm client error: %s", prod.ProductName, prod.Namespace, err)
		return "", err
	}

	releaseName := util.GeneReleaseName(serviceObj.GetReleaseNaming(), prod.ProductName, prod.Namespace, prod.EnvName, serviceObj.ServiceName)
	valuesMap, err := helmClient.GetReleaseValues(releaseName, true)
	if err != nil {
		log.Errorf("failed to get values map data, err: %s", err)
		return "", err
	}

	currentValuesYaml, err := yaml.Marshal(valuesMap)
	if err != nil {
		return "", err
	}

	err = os.WriteFile(filepath.Join(deliveryChartPath, setting.ValuesYaml), currentValuesYaml, 0644)
	if err != nil {
		return "", errors.Wrapf(err, "failed to write values.yaml")
	}

	return deliveryChartPath, nil
}

// handleImageRegistry update image registry to target registry
func handleImageRegistry(valuesYaml []byte, chartData *DeliveryChartData, targetRegistry *commonmodels.RegistryNamespace,
	registryMap map[string]*commonmodels.RegistryNamespace, imageData []*ImageData) ([]byte, *ServiceImageDetails, error) {

	flatMap, err := converter.YamlToFlatMap(valuesYaml)
	if err != nil {
		return nil, nil, err
	}

	imageTagMap := make(map[string]string)
	for _, it := range imageData {
		if it.Selected {
			imageTagMap[it.ImageName] = it.ImageTag
		}
	}

	retValuesYaml := string(valuesYaml)

	serviceObj := chartData.ServiceObj
	imagePathSpecs := make([]map[string]string, 0)
	for _, container := range serviceObj.Containers {
		imageSearchRule := &template.ImageSearchingRule{
			Repo:  container.ImagePath.Repo,
			Image: container.ImagePath.Image,
			Tag:   container.ImagePath.Tag,
		}
		pattern := imageSearchRule.GetSearchingPattern()
		imagePathSpecs = append(imagePathSpecs, pattern)
	}

	imageDetail := &ServiceImageDetails{
		ServiceName: serviceObj.ServiceName,
		Images:      make([]*ImageUrlDetail, 0),
	}

	registrySet := sets.NewString()
	for _, spec := range imagePathSpecs {
		imageUrl, err := commonservice.GeneImageURI(spec, flatMap)
		if err != nil {
			return nil, nil, err
		}

		imageName := commonservice.ExtractImageName(imageUrl)
		imageTag := commonservice.ExtractImageTag(imageUrl)

		customTag := ""
		if ct, ok := imageTagMap[imageName]; ok {
			customTag = ct
		} else {
			continue
		}

		if customTag == "" {
			customTag = imageTag
		}

		registryUrl, err := commonservice.ExtractImageRegistry(imageUrl)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "failed to parse registry from image uri: %s", imageUrl)
		}
		registryUrl = strings.TrimSuffix(registryUrl, "/")

		registryID := ""
		// used source registry
		if registry, ok := registryMap[registryUrl]; ok {
			registryID = registry.ID.Hex()
			registrySet.Insert(registryID)
		}

		imageDetail.Images = append(imageDetail.Images, &ImageUrlDetail{
			ImageUrl:  imageUrl,
			Name:      imageName,
			Tag:       imageTag,
			Registry:  registryID,
			CustomTag: customTag,
		})

		// assign image to values.yaml
		targetImageUrl := util.ReplaceRepo(imageUrl, targetRegistry.RegAddr, targetRegistry.Namespace)
		targetImageUrl = util.ReplaceTag(targetImageUrl, customTag)
		replaceValuesMap, err := commonservice.AssignImageData(targetImageUrl, spec)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to pase image uri %s, err %s", targetImageUrl, err)
		}

		// replace image into final merged values.yaml
		retValuesYaml, err = commonservice.ReplaceImage(retValuesYaml, replaceValuesMap)
		if err != nil {
			return nil, nil, err
		}
	}

	imageDetail.Registries = registrySet.List()
	return []byte(retValuesYaml), imageDetail, nil
}

func handleSingleChart(chartData *DeliveryChartData, product *commonmodels.Product, chartRepo *commonmodels.HelmRepo, dir string, globalVariables string,
	targetRegistry *commonmodels.RegistryNamespace, registryMap map[string]*commonmodels.RegistryNamespace) (*ServiceImageDetails, error) {
	serviceObj := chartData.ServiceObj

	deliveryChartPath, err := ensureChartFiles(chartData, product)
	if err != nil {
		return nil, err
	}

	valuesYamlData := make(map[string]interface{})
	valuesFilePath := filepath.Join(deliveryChartPath, setting.ValuesYaml)
	valueYamlContent, err := os.ReadFile(valuesFilePath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read values.yaml for service %s", serviceObj.ServiceName)
	}

	// write values.yaml file before load
	if len(chartData.ChartData.ValuesYamlContent) > 0 { // values.yaml was edited directly
		if err = yaml.Unmarshal([]byte(chartData.ChartData.ValuesYamlContent), map[string]interface{}{}); err != nil {
			log.Errorf("invalid yaml content, serviceName: %s, yamlContent: %s", serviceObj.ServiceName, chartData.ChartData.ValuesYamlContent)
			return nil, errors.Wrapf(err, "invalid yaml content for service: %s", serviceObj.ServiceName)
		}
		valueYamlContent = []byte(chartData.ChartData.ValuesYamlContent)
	} else if len(globalVariables) > 0 { // merge global variables
		valueYamlContent, err = yamlutil.Merge([][]byte{valueYamlContent, []byte(globalVariables)})
		if err != nil {
			return nil, errors.Wrapf(err, "failed to merge global variables for service: %s", serviceObj.ServiceName)
		}
	}

	// replace image url(registryUrl and imageTag)
	valueYamlContent, imageDetail, err := handleImageRegistry(valueYamlContent, chartData, targetRegistry, registryMap, chartData.ChartData.ImageData)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to handle image registry for service: %s", serviceObj.ServiceName)
	}

	err = yaml.Unmarshal(valueYamlContent, &valuesYamlData)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal values.yaml for service %s", serviceObj.ServiceName)
	}

	// hold the currently running yaml data
	chartData.ValuesInEnv = valuesYamlData

	err = os.WriteFile(valuesFilePath, valueYamlContent, 0644)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to write values.yaml file for service %s", serviceObj.ServiceName)
	}

	//load chart info from local storage
	chartRequested, err := chartloader.Load(deliveryChartPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load chart info, path %s", deliveryChartPath)
	}

	//set metadata
	chartRequested.Metadata.Name = chartData.ChartData.ServiceName
	chartRequested.Metadata.Version = chartData.ChartData.Version
	chartRequested.Metadata.AppVersion = chartData.ChartData.Version

	//create local chart package
	chartPackagePath, err := helm.CreateChartPackage(&helm.Chart{Chart: chartRequested}, dir)
	if err != nil {
		return nil, err
	}

	client, err := helmtool.NewClient()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create chart repo client, repoName: %s", chartRepo.RepoName)
	}

	log.Infof("pushing chart %s to %s...", filepath.Base(chartPackagePath), chartRepo.URL)
	err = client.PushChart(commonservice.GeneHelmRepo(chartRepo), chartPackagePath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to push chart: %s", chartPackagePath)
	}
	return imageDetail, nil
}

func makeChartTGZFileDir(productName, versionName string) (string, error) {
	dirPath := getChartTGZDir(productName, versionName)
	if err := os.RemoveAll(dirPath); err != nil {
		if !os.IsExist(err) {
			return "", errors.Wrapf(err, "failed to claer dir for chart tgz files")
		}
	}
	err := os.MkdirAll(dirPath, 0777)
	if err != nil {
		return "", errors.Wrapf(err, "failed to create chart tgz dir for version: %s", versionName)
	}
	return dirPath, nil
}

func CreateHelmDeliveryVersion(args *CreateHelmDeliveryVersionArgs, logger *zap.SugaredLogger) error {
	if args.Retry {
		return RetryCreateHelmDeliveryVersion(args.ProductName, args.Version, logger)
	} else {
		return CreateNewHelmDeliveryVersion(args, logger)
	}
}

// validate chartInfo, make sure service is in environment
// prepare data set for chart delivery
func prepareChartData(chartDatas []*CreateHelmDeliveryVersionChartData, productInfo *commonmodels.Product) (map[string]*DeliveryChartData, error) {

	renderSet, err := commonrepo.NewRenderSetColl().Find(&commonrepo.RenderSetFindOption{
		Revision:    productInfo.Render.Revision,
		Name:        productInfo.Render.Name,
		EnvName:     productInfo.EnvName,
		ProductTmpl: productInfo.ProductName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to find renderSet: %s, revision: %d", productInfo.Render.Name, productInfo.Render.Revision)
	}
	chartMap := make(map[string]*template.ServiceRender)
	for _, rChart := range renderSet.ChartInfos {
		chartMap[rChart.ServiceName] = rChart
	}

	chartDataMap := make(map[string]*DeliveryChartData)
	serviceMap := productInfo.GetServiceMap()
	for _, chartData := range chartDatas {
		if productService, ok := serviceMap[chartData.ServiceName]; ok {
			serviceObj, err := commonrepo.NewServiceColl().Find(&commonrepo.ServiceFindOption{
				ServiceName: chartData.ServiceName,
				Revision:    productService.Revision,
				Type:        setting.HelmDeployType,
				ProductName: productInfo.ProductName,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to query service: %s", chartData.ServiceName)
			}
			renderChart, ok := chartMap[chartData.ServiceName]
			if !ok {
				return nil, fmt.Errorf("can't find renderChart for service: %s", chartData.ServiceName)
			}
			chartDataMap[chartData.ServiceName] = &DeliveryChartData{
				ChartData:      chartData,
				RenderChart:    renderChart,
				ServiceObj:     serviceObj,
				ProductService: productService,
				RenderSet:      renderSet,
			}
		} else {
			return nil, fmt.Errorf("service %s not found in environment", chartData.ServiceName)
		}
	}
	return chartDataMap, nil
}

func buildRegistryMap() (map[string]*commonmodels.RegistryNamespace, error) {
	registries, err := commonservice.ListRegistryNamespaces("", true, log.SugaredLogger())
	if err != nil {
		return nil, fmt.Errorf("failed to query registries")
	}
	ret := make(map[string]*commonmodels.RegistryNamespace)
	for _, singleRegistry := range registries {
		fullUrl := fmt.Sprintf("%s/%s", singleRegistry.RegAddr, singleRegistry.Namespace)
		fullUrl = strings.TrimSuffix(fullUrl, "/")
		u, _ := url.Parse(fullUrl)
		if len(u.Scheme) > 0 {
			fullUrl = strings.TrimPrefix(fullUrl, fmt.Sprintf("%s://", u.Scheme))
		}
		ret[fullUrl] = singleRegistry
	}
	return ret, nil
}

func buildArtifactTaskArgs(projectName, envName string, imagesMap *sync.Map) *commonmodels.ArtifactPackageTaskArgs {
	imageArgs := make([]*commonmodels.ImagesByService, 0)
	sourceRegistry := sets.NewString()
	imagesMap.Range(func(key, value interface{}) bool {
		imageDetail := value.(*ServiceImageDetails)
		imagesByService := &commonmodels.ImagesByService{
			ServiceName: imageDetail.ServiceName,
			Images:      make([]*commonmodels.ImageData, 0),
		}
		for _, image := range imageDetail.Images {
			imagesByService.Images = append(imagesByService.Images, &commonmodels.ImageData{
				ImageUrl:   image.ImageUrl,
				ImageName:  image.Name,
				ImageTag:   image.Tag,
				CustomTag:  image.CustomTag,
				RegistryID: image.Registry,
			})
		}
		imageArgs = append(imageArgs, imagesByService)
		sourceRegistry.Insert(imageDetail.Registries...)
		return true
	})

	ret := &commonmodels.ArtifactPackageTaskArgs{
		ProjectName:      projectName,
		EnvName:          envName,
		Images:           imageArgs,
		SourceRegistries: sourceRegistry.List(),
	}
	return ret
}

// insert delivery distribution data for single chart, include image and chart
func insertDeliveryDistributions(result *task.ServicePackageResult, chartVersion string, deliveryVersion *commonmodels.DeliveryVersion, args *DeliveryVersionChartData) error {
	for _, image := range result.ImageData {
		err := commonrepo.NewDeliveryDistributeColl().Insert(&commonmodels.DeliveryDistribute{
			ReleaseID:      deliveryVersion.ID,
			ServiceName:    image.ImageName, // image name
			ChartName:      result.ServiceName,
			DistributeType: config.Image,
			RegistryName:   image.ImageUrl,
			Namespace:      commonservice.ExtractRegistryNamespace(image.ImageUrl),
			CreatedAt:      time.Now().Unix(),
		})
		if err != nil {
			log.Errorf("failed to insert image distribute data, chartName: %s, err: %s", result.ServiceName, err)
			return fmt.Errorf("failed to insert image distribute data, chartName: %s", result.ServiceName)
		}
	}

	err := commonrepo.NewDeliveryDistributeColl().Insert(&commonmodels.DeliveryDistribute{
		ReleaseID:      deliveryVersion.ID,
		DistributeType: config.Chart,
		ChartName:      result.ServiceName,
		ChartVersion:   chartVersion,
		ChartRepoName:  args.ChartRepoName,
		SubDistributes: nil,
		CreatedAt:      time.Now().Unix(),
	})
	if err != nil {
		log.Errorf("failed to insert chart distribute data, chartName: %s, err: %s", result.ServiceName, err)
		return fmt.Errorf("failed to insert chart distribute data, chartName: %s", result.ServiceName)
	}
	return nil
}

func buildDeliveryCharts(chartDataMap map[string]*DeliveryChartData, deliveryVersion *commonmodels.DeliveryVersion, args *DeliveryVersionChartData, logger *zap.SugaredLogger) (err error) {
	defer func() {
		if err != nil {
			deliveryVersion.Status = setting.DeliveryVersionStatusFailed
			deliveryVersion.Error = err.Error()
		}
		updateVersionStatus(deliveryVersion.Version, deliveryVersion.ProductName, deliveryVersion.Status, deliveryVersion.Error)
	}()

	var errLock sync.Mutex
	errorList := &multierror.Error{}

	appendError := func(err error) {
		errLock.Lock()
		defer errLock.Unlock()
		errorList = multierror.Append(errorList, err)
	}

	dir, err := makeChartTGZFileDir(deliveryVersion.ProductName, deliveryVersion.Version)
	if err != nil {
		return err
	}
	repoInfo, err := getChartRepoData(args.ChartRepoName)
	if err != nil {
		log.Errorf("failed to query chart-repo info, productName: %s, err: %s", deliveryVersion.ProductName, err)
		return fmt.Errorf("failed to query chart-repo info, productName: %s, repoName: %s", deliveryVersion.ProductName, args.ChartRepoName)
	}

	registryMap, err := buildRegistryMap()
	if err != nil {
		return fmt.Errorf("failed to build registry map")
	}

	var targetRegistry *commonmodels.RegistryNamespace
	for _, registry := range registryMap {
		if registry.ID.Hex() == args.ImageRegistryID {
			targetRegistry = registry
			break
		}
	}

	imagesDataMap := &sync.Map{}

	wg := sync.WaitGroup{}
	for _, chartData := range chartDataMap {
		wg.Add(1)
		go func(cData *DeliveryChartData) {
			defer wg.Done()
			// generate new chart data, push to chart repo, extract related images
			imageData, err := handleSingleChart(cData, deliveryVersion.ProductEnvInfo, repoInfo, dir, args.GlobalVariables, targetRegistry, registryMap)
			if err != nil {
				logger.Errorf("failed to build chart package, serviceName: %s err: %s", cData.ChartData.ServiceName, err)
				appendError(err)
				return
			}
			imagesDataMap.Store(cData.ServiceObj.ServiceName, imageData)
		}(chartData)
	}
	wg.Wait()

	if errorList.ErrorOrNil() != nil {
		err = errorList.ErrorOrNil()
		return
	}

	// create task to deal with images
	// offline docker images are not supported
	taskArgs := buildArtifactTaskArgs(deliveryVersion.ProductName, deliveryVersion.ProductEnvInfo.EnvName, imagesDataMap)
	taskArgs.TargetRegistries = []string{args.ImageRegistryID}
	taskID, err := workflowservice.CreateArtifactPackageTask(taskArgs, deliveryVersion.Version, logger)
	if err != nil {
		return err
	}
	deliveryVersion.TaskID = int(taskID)
	err = commonrepo.NewDeliveryVersionColl().UpdateTaskID(deliveryVersion.Version, deliveryVersion.ProductName, int32(deliveryVersion.TaskID))
	if err != nil {
		logger.Errorf("failed to update delivery version task_id, version: %s, task_id: %s, err: %s", deliveryVersion, deliveryVersion.ProductName, deliveryVersion.TaskID)
	}
	// start a new routine to check task results
	go waitVersionDone(deliveryVersion)

	return
}

// send hook
func sendVersionDeliveryHook(deliveryVersion *commonmodels.DeliveryVersion, host, urlPath string) error {
	projectName, version := deliveryVersion.ProductName, deliveryVersion.Version
	ret := &DeliveryVersionHookPayload{
		ProjectName: projectName,
		Version:     version,
		Status:      setting.DeliveryVersionStatusSuccess,
		Error:       "",
		StartTime:   deliveryVersion.CreatedAt,
		EndTime:     time.Now().Unix(),
		Charts:      make([]*DeliveryVersionPayloadChart, 0),
	}

	//distributes image + chart
	deliveryDistributes, err := FindDeliveryDistribute(&commonrepo.DeliveryDistributeArgs{
		ReleaseID: deliveryVersion.ID.Hex(),
	}, log.SugaredLogger())
	if err != nil {
		return err
	}

	distributeImageMap := make(map[string][]*DeliveryVersionPayloadImage)
	for _, distributeImage := range deliveryDistributes {
		if distributeImage.DistributeType != config.Image {
			continue
		}
		distributeImageMap[distributeImage.ChartName] = append(distributeImageMap[distributeImage.ChartName], &DeliveryVersionPayloadImage{
			ServiceModule: distributeImage.ServiceName,
			Image:         distributeImage.RegistryName,
		})
	}

	chartRepoName := ""
	for _, distribute := range deliveryDistributes {
		if distribute.DistributeType != config.Chart {
			continue
		}
		ret.Charts = append(ret.Charts, &DeliveryVersionPayloadChart{
			ChartName:    distribute.ChartName,
			ChartVersion: distribute.ChartVersion,
			Images:       distributeImageMap[distribute.ChartName],
		})
		chartRepoName = distribute.ChartRepoName
	}
	err = fillChartUrl(ret.Charts, chartRepoName)
	if err != nil {
		return err
	}

	targetPath := fmt.Sprintf("%s/%s", host, strings.TrimPrefix(urlPath, "/"))

	// validate url
	_, err = url.Parse(targetPath)
	if err != nil {
		return err
	}

	reqBody, err := json.Marshal(ret)
	if err != nil {
		log.Errorf("marshal json args error: %s", err)
		return err
	}

	request, err := http.NewRequest("POST", targetPath, bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")

	client := http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		return err
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("hook request send to url: %s got error resp code : %d", targetPath, resp.StatusCode)
	}

	return nil
}

func updateVersionStatus(versionName, projectName, status, errStr string) {
	// send hook info when version build finished
	if status == setting.DeliveryVersionStatusSuccess {

		versionInfo, err := commonrepo.NewDeliveryVersionColl().Get(&commonrepo.DeliveryVersionArgs{
			ProductName: projectName,
			Version:     versionName,
		})
		if err != nil {
			log.Errorf("failed to find version: %s of project %s, err: %s", versionName, projectName, err)
			return
		}
		if versionInfo.Status == status {
			return
		}

		templateProduct, err := templaterepo.NewProductColl().Find(projectName)
		if err != nil {
			log.Errorf("updateVersionStatus failed to find template product: %s, err: %s", projectName, err)
		} else {
			hookConfig := templateProduct.DeliveryVersionHook
			if hookConfig != nil && hookConfig.Enable {
				err = sendVersionDeliveryHook(versionInfo, hookConfig.HookHost, hookConfig.Path)
				if err != nil {
					log.Errorf("updateVersionStatus failed to send version delivery hook, projectName: %s, err: %s", projectName, err)
				}
			}
		}
	}

	err := commonrepo.NewDeliveryVersionColl().UpdateStatusByName(versionName, projectName, status, errStr)
	if err != nil {
		log.Errorf("failed to update version status, name: %s, err: %s", versionName, err)
	}
}

func taskFinished(status config.Status) bool {
	return status == config.StatusPassed || status == config.StatusFailed || status == config.StatusTimeout || status == config.StatusCancelled
}

func waitVersionDone(deliveryVersion *commonmodels.DeliveryVersion) {
	waitTimeout := time.After(60 * time.Minute * 1)
	for {
		select {
		case <-waitTimeout:
			updateVersionStatus(deliveryVersion.Version, deliveryVersion.ProductName, setting.DeliveryVersionStatusFailed, "timeout")
			return
		default:
			done, err := checkVersionStatus(deliveryVersion)
			if err != nil {
				updateVersionStatus(deliveryVersion.Version, deliveryVersion.ProductName, setting.DeliveryVersionStatusFailed, err.Error())
				return
			}
			if done {
				return
			}
		}
		time.Sleep(time.Second * 5)
	}
}

func checkVersionStatus(deliveryVersion *commonmodels.DeliveryVersion) (bool, error) {
	if deliveryVersion.Status == setting.DeliveryVersionStatusSuccess || deliveryVersion.Status == setting.DeliveryVersionStatusFailed {
		return true, nil
	}
	pipelineName := fmt.Sprintf("%s-%s-%s", deliveryVersion.ProductName, deliveryVersion.ProductEnvInfo.EnvName, "artifact")
	taskData, err := commonrepo.NewTaskColl().Find(int64(deliveryVersion.TaskID), pipelineName, config.ArtifactType)
	if err != nil {
		return false, fmt.Errorf("failed to query taskData, id: %d, pipelineName: %s", deliveryVersion.TaskID, pipelineName)
	}

	if len(taskData.Stages) != 1 {
		return false, fmt.Errorf("invalid task data, stage length not leagal")
	}

	argsBytes, err := json.Marshal(deliveryVersion.CreateArgument)
	if err != nil {
		return false, errors.Wrapf(err, "failed to marshal arguments, versionName: %s err: %s", deliveryVersion.Version, err)
	}
	createArgs := new(DeliveryVersionChartData)
	err = json.Unmarshal(argsBytes, createArgs)
	if err != nil {
		return false, errors.Wrapf(err, "failed to unMarshal arguments, versionName: %s err: %s", deliveryVersion.Version, err)
	}

	distributes, err := commonrepo.NewDeliveryDistributeColl().Find(&commonrepo.DeliveryDistributeArgs{
		DistributeType: config.Chart,
		ReleaseID:      deliveryVersion.ID.Hex(),
	})
	if err != nil {
		return false, errors.Wrapf(err, "failed to query distrubutes, versionName: %s", deliveryVersion.Version)
	}

	// for charts has been successfully handled, download charts directly
	successCharts := sets.NewString()
	for _, distribute := range distributes {
		successCharts.Insert(distribute.ChartName)
	}

	stage := taskData.Stages[0]
	errorList := &multierror.Error{}

	allTaskDone := true
	for _, taskData := range stage.SubTasks {
		artifactPackageArgs, err := base.ToArtifactPackageImageTask(taskData)
		if err != nil {
			return false, errors.Wrapf(err, "failed to generate origin artifact task data")
		}

		progressData, err := artifactPackageArgs.GetProgress()
		if err != nil {
			return false, errors.Wrapf(err, "failed to get progress data from data")
		}
		progressDataMap := make(map[string]*task.ServicePackageResult)
		for _, singleResult := range progressData {
			progressDataMap[singleResult.ServiceName] = singleResult
		}

		for _, chartData := range createArgs.ChartDatas {
			// service artifact has been marked as success
			if successCharts.Has(chartData.ServiceName) {
				continue
			}
			if singleResult, ok := progressDataMap[chartData.ServiceName]; ok {
				if singleResult.Result != "success" {
					errorList = multierror.Append(errorList, fmt.Errorf("failed to build image distribute for service:%s, err: %s ", singleResult.ServiceName, singleResult.ErrorMsg))
					continue
				}
				err = insertDeliveryDistributions(singleResult, chartData.Version, deliveryVersion, createArgs)
				if err != nil {
					errorList = multierror.Append(errorList, fmt.Errorf("failed to insert distribute data for service:%s ", singleResult.ServiceName))
					continue
				}
				successCharts.Insert(chartData.ServiceName)
			}
		}

		if !taskFinished(artifactPackageArgs.TaskStatus) {
			allTaskDone = false
		}
		if len(artifactPackageArgs.Error) > 0 {
			errorList = multierror.Append(errorList, fmt.Errorf(artifactPackageArgs.Error))
		}
	}

	if allTaskDone {
		if successCharts.Len() == len(createArgs.ChartDatas) {
			deliveryVersion.Status = setting.DeliveryVersionStatusSuccess
		} else {
			deliveryVersion.Status = setting.DeliveryVersionStatusFailed
		}
	}
	if errorList.ErrorOrNil() != nil {
		deliveryVersion.Error = errorList.Error()
	}
	updateVersionStatus(deliveryVersion.Version, deliveryVersion.ProductName, deliveryVersion.Status, deliveryVersion.Error)
	return allTaskDone, nil
}

func CreateNewHelmDeliveryVersion(args *CreateHelmDeliveryVersionArgs, logger *zap.SugaredLogger) error {
	// need appoint chart info
	if len(args.ChartDatas) == 0 {
		return e.ErrCreateDeliveryVersion.AddDesc("no chart info appointed")
	}
	// validate necessary params
	if len(args.ChartRepoName) == 0 {
		return e.ErrCreateDeliveryVersion.AddDesc("chart repo not appointed")
	}
	if len(args.ImageRegistryID) == 0 {
		return e.ErrCreateDeliveryVersion.AddDesc("image registry not appointed")
	}
	// prepare data
	productInfo, err := getProductEnvInfo(args.ProductName, args.EnvName)
	if err != nil {
		log.Infof("failed to query product info, productName: %s envName %s, err: %s", args.ProductName, args.EnvName, err)
		return e.ErrCreateDeliveryVersion.AddDesc(fmt.Sprintf("failed to query product info, procutName: %s envName %s", args.ProductName, args.EnvName))
	}
	chartDataMap, err := prepareChartData(args.ChartDatas, productInfo)
	if err != nil {
		return e.ErrCreateDeliveryVersion.AddErr(err)
	}

	productInfo.ID, _ = primitive.ObjectIDFromHex("")

	versionObj := &commonmodels.DeliveryVersion{
		Version:        args.Version,
		ProductName:    args.ProductName,
		Type:           setting.DeliveryVersionTypeChart,
		Desc:           args.Desc,
		Labels:         args.Labels,
		ProductEnvInfo: productInfo,
		Status:         setting.DeliveryVersionStatusCreating,
		CreateArgument: args.DeliveryVersionChartData,
		CreatedBy:      args.CreateBy,
		CreatedAt:      time.Now().Unix(),
		DeletedAt:      0,
	}

	err = commonrepo.NewDeliveryVersionColl().Insert(versionObj)
	if err != nil {
		logger.Errorf("failed to insert version data, err: %s", err)
		return e.ErrCreateDeliveryVersion.AddErr(fmt.Errorf("failed to insert delivery version: %s", versionObj.Version))
	}

	err = buildDeliveryCharts(chartDataMap, versionObj, args.DeliveryVersionChartData, logger)
	if err != nil {
		return err
	}

	return nil
}

func RetryCreateHelmDeliveryVersion(projectName, versionName string, logger *zap.SugaredLogger) error {
	deliveryVersion, err := commonrepo.NewDeliveryVersionColl().Get(&commonrepo.DeliveryVersionArgs{
		ProductName: projectName,
		Version:     versionName,
	})
	if err != nil {
		logger.Errorf("failed to query delivery version data, verisonName: %s, error: %s", versionName, err)
		return fmt.Errorf("failed to query delivery version data, verisonName: %s", versionName)
	}

	if deliveryVersion.Status != setting.DeliveryVersionStatusFailed {
		return fmt.Errorf("can't reCreate version with status:%s", deliveryVersion.Status)
	}

	argsBytes, err := json.Marshal(deliveryVersion.CreateArgument)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal arguments, versionName: %s err: %s", deliveryVersion.Version, err)
	}
	createArgs := new(DeliveryVersionChartData)
	err = json.Unmarshal(argsBytes, createArgs)
	if err != nil {
		return errors.Wrapf(err, "failed to unMarshal arguments, versionName: %s err: %s", deliveryVersion.Version, err)
	}

	productInfoSnap := deliveryVersion.ProductEnvInfo

	distributes, err := commonrepo.NewDeliveryDistributeColl().Find(&commonrepo.DeliveryDistributeArgs{
		DistributeType: config.Chart,
		ReleaseID:      deliveryVersion.ID.Hex(),
	})
	if err != nil {
		return errors.Wrapf(err, "failed to query distrubutes, versionName: %s", deliveryVersion.Version)
	}

	// for charts has been successfully handled, download charts directly
	successCharts := sets.NewString()
	for _, distribute := range distributes {
		if distribute.DistributeType != config.Chart {
			continue
		}
		_, err := downloadChart(deliveryVersion, distribute)
		if err != nil {
			log.Errorf("failed to download chart from chart repo, chartName: %s, err: %s", distribute.ChartName, err)
			continue
		}
		successCharts.Insert(distribute.ChartName)
	}

	chartsToBeHandled := make([]*CreateHelmDeliveryVersionChartData, 0)
	for _, chartConfig := range createArgs.ChartDatas {
		if successCharts.Has(chartConfig.ServiceName) {
			continue
		}
		chartsToBeHandled = append(chartsToBeHandled, chartConfig)
	}

	chartDataMap, err := prepareChartData(chartsToBeHandled, productInfoSnap)
	if err != nil {
		return e.ErrCreateDeliveryVersion.AddErr(err)
	}

	err = buildDeliveryCharts(chartDataMap, deliveryVersion, createArgs, logger)
	if err != nil {
		return err
	}

	// update status
	deliveryVersion.Status = setting.DeliveryVersionStatusRetrying
	updateVersionStatus(deliveryVersion.Version, deliveryVersion.ProductName, deliveryVersion.Status, deliveryVersion.Error)

	return nil
}

func ListDeliveryServiceNames(productName string, log *zap.SugaredLogger) ([]string, error) {
	serviceNames := sets.String{}

	version := new(commonrepo.DeliveryVersionArgs)
	version.ProductName = productName
	deliveryVersions, err := FindDeliveryVersion(version, log)
	if err != nil {
		log.Errorf("FindDeliveryVersion failed, err:%v", err)
		return serviceNames.List(), err
	}

	for _, deliveryVersion := range deliveryVersions {
		deliveryDeployArgs := new(commonrepo.DeliveryDeployArgs)
		deliveryDeployArgs.ReleaseID = deliveryVersion.ID.Hex()
		deliveryDeploys, err := FindDeliveryDeploy(deliveryDeployArgs, log)
		if err != nil {
			log.Errorf("FindDeliveryDeploy failed, ReleaseID:%s, err:%v", deliveryVersion.ID, err)
			continue
		}
		for _, deliveryDeploy := range deliveryDeploys {
			serviceNames.Insert(deliveryDeploy.ServiceName)
		}
	}

	return serviceNames.UnsortedList(), nil
}

func downloadChart(deliveryVersion *commonmodels.DeliveryVersion, chartInfo *commonmodels.DeliveryDistribute) (string, error) {
	productName, versionName := deliveryVersion.ProductName, deliveryVersion.Version
	chartTGZName := fmt.Sprintf("%s-%s.tgz", chartInfo.ChartName, chartInfo.ChartVersion)
	chartTGZFileParent, err := makeChartTGZFileDir(productName, versionName)
	if err != nil {
		return "", err
	}

	chartTGZFilePath := filepath.Join(chartTGZFileParent, chartTGZName)
	if _, err := os.Stat(chartTGZFilePath); err == nil {
		// local cache exists
		return chartTGZFilePath, nil
	}

	chartRepo, err := getChartRepoData(chartInfo.ChartRepoName)
	if err != nil {
		return "", err
	}
	hClient, err := helmtool.NewClient()
	if err != nil {
		return "", err
	}

	chartRef := fmt.Sprintf("%s/%s", chartRepo.RepoName, chartInfo.ChartName)
	return chartTGZFilePath, hClient.DownloadChart(commonservice.GeneHelmRepo(chartRepo), chartRef, chartInfo.ChartVersion, chartTGZFileParent, false)
}

func getChartDistributeInfo(releaseID, chartName string, log *zap.SugaredLogger) (*commonmodels.DeliveryDistribute, error) {
	distributes, _ := FindDeliveryDistribute(&commonrepo.DeliveryDistributeArgs{
		ReleaseID:      releaseID,
		ChartName:      chartName,
		DistributeType: config.Chart,
	}, log)

	if len(distributes) != 1 {
		log.Warnf("find chart %s failed, expect count %d, found count %d, release_id: %s", chartName, 1, len(distributes), releaseID)
		return nil, fmt.Errorf("can't find target charts")
	}

	chartInfo := distributes[0]
	return chartInfo, nil
}

func DownloadDeliveryChart(projectName, version string, chartName string, log *zap.SugaredLogger) ([]byte, string, error) {

	filePath, err := preDownloadChart(projectName, version, chartName, log)
	if err != nil {
		return nil, "", err
	}

	fileBytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, "", err
	}

	return fileBytes, filepath.Base(filePath), err
}

func preDownloadChart(projectName, versionName, chartName string, log *zap.SugaredLogger) (string, error) {
	deliveryInfo, err := GetDeliveryVersion(&commonrepo.DeliveryVersionArgs{
		ProductName: projectName,
		Version:     versionName,
	}, log)
	if err != nil {
		return "", fmt.Errorf("failed to query delivery info")
	}

	chartInfo, err := getChartDistributeInfo(deliveryInfo.ID.Hex(), chartName, log)
	if err != nil {
		return "", err
	}
	// prepare chart data
	filePath, err := downloadChart(deliveryInfo, chartInfo)
	if err != nil {
		return "", err
	}
	return filePath, err
}

func getIndexInfoFromChartRepo(chartRepoName string) (*repo.IndexFile, error) {
	chartRepo, err := getChartRepoData(chartRepoName)
	if err != nil {
		return nil, err
	}
	hClient, err := helmtool.NewClient()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create chart repo client")
	}
	return hClient.FetchIndexYaml(commonservice.GeneHelmRepo(chartRepo))
}

func fillChartUrl(charts []*DeliveryVersionPayloadChart, chartRepoName string) error {
	index, err := getIndexInfoFromChartRepo(chartRepoName)
	if err != nil {
		return err
	}
	chartMap := make(map[string]*DeliveryVersionPayloadChart)
	for _, chart := range charts {
		chartMap[chart.ChartName] = chart
	}

	for name, entries := range index.Entries {
		chart, ok := chartMap[name]
		if !ok {
			continue
		}
		if len(entries) == 0 {
			continue
		}

		for _, entry := range entries {
			if entry.Version == chart.ChartVersion {
				if len(entry.URLs) > 0 {
					chart.ChartUrl = entry.URLs[0]
				}
				break
			}
		}
	}
	return nil
}

func GetChartVersion(chartName, chartRepoName string) ([]*ChartVersionResp, error) {

	index, err := getIndexInfoFromChartRepo(chartRepoName)
	if err != nil {
		return nil, err
	}

	chartNameList := strings.Split(chartName, ",")
	chartNameSet := sets.NewString(chartNameList...)
	existedChartSet := sets.NewString()

	ret := make([]*ChartVersionResp, 0)

	for name, entry := range index.Entries {
		if !chartNameSet.Has(name) {
			continue
		}
		if len(entry) == 0 {
			continue
		}
		latestEntry := entry[0]

		// generate suggested next chart version
		nextVersion := latestEntry.Version
		t, err := semver.Make(latestEntry.Version)
		if err != nil {
			log.Errorf("failed to parse current version: %s, err: %s", latestEntry.Version, err)
		} else {
			t.Patch = t.Patch + 1
			nextVersion = t.String()
		}

		ret = append(ret, &ChartVersionResp{
			ChartName:        name,
			ChartVersion:     latestEntry.Version,
			NextChartVersion: nextVersion,
		})
		existedChartSet.Insert(name)
	}

	for _, singleChartName := range chartNameList {
		if existedChartSet.Has(singleChartName) {
			continue
		}
		ret = append(ret, &ChartVersionResp{
			ChartName:        singleChartName,
			ChartVersion:     "",
			NextChartVersion: "1.0.0",
		})
	}

	return ret, nil
}

func preDownloadAndUncompressChart(projectName, versionName, chartName string, log *zap.SugaredLogger) (string, error) {
	deliveryInfo, err := GetDeliveryVersion(&commonrepo.DeliveryVersionArgs{
		ProductName: projectName,
		Version:     versionName,
	}, log)
	if err != nil {
		return "", fmt.Errorf("failed to query delivery info")
	}

	chartDistribute, err := getChartDistributeInfo(deliveryInfo.ID.Hex(), chartName, log)
	if err != nil {
		return "", err
	}
	dstDir := getChartExpandDir(projectName, versionName)
	dstDir = filepath.Join(dstDir, fmt.Sprintf("%s-%s", chartDistribute.ChartName, chartDistribute.ChartVersion))

	filePath, err := preDownloadChart(projectName, versionName, chartName, log)
	if err != nil {
		return "", err
	}

	file, err := os.Open(filePath)
	if err != nil {
		return "", errors.Wrap(err, "unable to open tarball")
	}
	defer func() { _ = file.Close() }()

	err = chartutil.Expand(dstDir, file)
	if err != nil {
		log.Errorf("failed to uncompress file: %s", filePath)
		return "", errors.Wrapf(err, "failed to uncompress file")
	}
	return dstDir, nil
}

func PreviewDeliveryChart(projectName, version, chartName string, log *zap.SugaredLogger) (*DeliveryChartResp, error) {
	dstDir, err := preDownloadAndUncompressChart(projectName, version, chartName, log)
	if err != nil {
		return nil, err
	}

	ret := &DeliveryChartResp{
		FileInfos: make([]*types.FileInfo, 0),
	}

	var fis []*types.FileInfo
	files, err := os.ReadDir(filepath.Join(dstDir, chartName))
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		info, _ := file.Info()
		if info == nil {
			continue
		}
		fi := &types.FileInfo{
			Parent:  "",
			Name:    file.Name(),
			Size:    info.Size(),
			Mode:    file.Type(),
			ModTime: info.ModTime().Unix(),
			IsDir:   file.IsDir(),
		}

		fis = append(fis, fi)
	}
	ret.FileInfos = fis
	return ret, nil
}

// load chart file infos
func loadChartFileInfos(fileDir, chartName string, dir string) ([]*types.FileInfo, error) {
	var fis []*types.FileInfo
	files, err := os.ReadDir(filepath.Join(fileDir, chartName, dir))
	if err != nil {
		return nil, e.ErrFilePath.AddDesc(err.Error())
	}

	for _, file := range files {
		info, _ := file.Info()
		if info == nil {
			continue
		}
		fi := &types.FileInfo{
			Parent:  dir,
			Name:    file.Name(),
			Size:    info.Size(),
			Mode:    file.Type(),
			ModTime: info.ModTime().Unix(),
			IsDir:   file.IsDir(),
		}
		fis = append(fis, fi)
	}
	return fis, nil
}

func GetDeliveryChartFilePath(args *DeliveryChartFilePathArgs, log *zap.SugaredLogger) ([]*types.FileInfo, error) {
	projectName, version, chartName := args.ProjectName, args.Version, args.ChartName
	dstDir, err := preDownloadAndUncompressChart(projectName, version, chartName, log)
	if err != nil {
		return nil, nil
	}

	fileInfos, err := loadChartFileInfos(dstDir, chartName, args.Dir)
	if err != nil {
		return nil, err
	}
	return fileInfos, nil
}

func GetDeliveryChartFileContent(args *DeliveryChartFileContentArgs, log *zap.SugaredLogger) (string, error) {
	projectName, version, chartName := args.ProjectName, args.Version, args.ChartName
	dstDir, err := preDownloadAndUncompressChart(projectName, version, chartName, log)
	if err != nil {
		return "", nil
	}

	file := filepath.Join(dstDir, chartName, args.FilePath, args.FileName)
	fileContent, err := os.ReadFile(file)
	if err != nil {
		log.Errorf("Failed to read file %s, err: %s", file, err)
		return "", e.ErrFileContent.AddDesc(err.Error())
	}

	return string(fileContent), nil
}

func ApplyDeliveryGlobalVariables(args *DeliveryVariablesApplyArgs, logger *zap.SugaredLogger) (interface{}, error) {
	ret := new(DeliveryVariablesApplyArgs)
	for _, chartData := range args.ChartDatas {
		mergedYaml, err := yamlutil.Merge([][]byte{[]byte(chartData.ValuesYamlContent), []byte(args.GlobalVariables)})
		if err != nil {
			logger.Errorf("failed to merge gobal variables for service: %s", chartData.ServiceName)
			return nil, errors.Wrapf(err, "failed to merge global variables for service: %s", chartData.ServiceName)
		}
		ret.ChartDatas = append(ret.ChartDatas, &CreateHelmDeliveryVersionChartData{
			ServiceName:       chartData.ServiceName,
			ValuesYamlContent: string(mergedYaml),
		})
	}
	return ret, nil
}
