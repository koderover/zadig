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
	"cmp"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/blang/semver/v4"
	"github.com/chartmuseum/helm-push/pkg/helm"
	"github.com/hashicorp/go-multierror"
	"github.com/otiai10/copy"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	chartloader "helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/repo"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/yaml"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models/task"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models/template"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/repository"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	workflowservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow"
	"github.com/koderover/zadig/v2/pkg/setting"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	helmtool "github.com/koderover/zadig/v2/pkg/tool/helmclient"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
	stepspec "github.com/koderover/zadig/v2/pkg/types/step"
	"github.com/koderover/zadig/v2/pkg/util"
	"github.com/koderover/zadig/v2/pkg/util/converter"
	fsutil "github.com/koderover/zadig/v2/pkg/util/fs"
	yamlutil "github.com/koderover/zadig/v2/pkg/util/yaml"
)

const (
	VerbosityBrief                            string = "brief"    // brief delivery data
	VerbosityDetailed                         string = "detailed" // detailed delivery version with total data
	deliveryVersionWorkflowV4NamingConvention string = "zadig-delivery-%s"
)

type DeliveryVersionFilter struct {
	ServiceName string
}

type CreateHelmDeliveryVersionOption struct {
	EnableOfflineDist bool   `json:"enableOfflineDist"`
	S3StorageID       string `json:"s3StorageID"`
}

type ImageData struct {
	ContainerName string `json:"containerName"`
	Image         string `json:"image"`
	ImageName     string `json:"imageName"`
	ImageTag      string `json:"imageTag"`
	Selected      bool   `json:"selected"`
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
	Production    bool     `json:"production"`
	Labels        []string `json:"labels"`
	ImageRepoName string   `json:"imageRepoName"`
	*DeliveryVersionChartData
}

type CreateK8SDeliveryVersionYamlData struct {
	ServiceName string       `json:"serviceName"`
	YamlContent string       `json:"yamlContent"`
	ImageDatas  []*ImageData `json:"imageDatas"`
}

type CreateK8SDeliveryVersionArgs struct {
	CreateBy    string   `json:"-"`
	ProductName string   `json:"productName"`
	Retry       bool     `json:"retry"`
	Version     string   `json:"version"`
	Desc        string   `json:"desc"`
	EnvName     string   `json:"envName"`
	Production  bool     `json:"production"`
	Labels      []string `json:"labels"`
	*DeliveryVersionYamlData
}

type DeliveryVersionYamlData struct {
	ImageRegistryID string                              `json:"imageRegistryID"`
	YamlDatas       []*CreateK8SDeliveryVersionYamlData `json:"yamlDatas"`
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
	ImageUrl         string
	Name             string
	SourceRegistryID string
	TargetRegistryID string
	Tag              string
	CustomTag        string
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

func FindDeliveryVersion(args *commonrepo.DeliveryVersionArgs, log *zap.SugaredLogger) ([]*commonmodels.DeliveryVersion, int, error) {
	resp, total, err := commonrepo.NewDeliveryVersionColl().Find(args)
	if err != nil {
		log.Errorf("find deliveryVersion error: %v", err)
		return resp, 0, e.ErrFindDeliveryVersion.AddErr(err)
	}
	return resp, total, err
}

func DeleteDeliveryVersion(args *commonrepo.DeliveryVersionArgs, log *zap.SugaredLogger) error {
	err := commonrepo.NewDeliveryVersionColl().Delete(args.ID)
	if err != nil {
		log.Errorf("delete deliveryVersion error: %v", err)
		return e.ErrDeleteDeliveryVersion
	}
	return nil
}

func filterReleases(filter *DeliveryVersionFilter, deliveryVersion *commonmodels.DeliveryVersion, deliveryDeploys []*commonmodels.DeliveryDeploy, logger *zap.SugaredLogger) bool {
	if filter == nil {
		return true
	}
	if filter.ServiceName != "" {
		deliveryDeployArgs := new(commonrepo.DeliveryDeployArgs)
		deliveryDeployArgs.ReleaseID = deliveryVersion.ID.Hex()
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
	deliveryVersion.ProductEnvInfo = nil
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
		if !filterReleases(filterOpt, deliveryVersion, deliveryDeploys, logger) {
			return nil, nil
		}
	}

	production := false
	if deliveryVersion.ProductEnvInfo != nil {
		production = deliveryVersion.ProductEnvInfo.Production
	}

	// order deploys by service name
	productTemplate, err := templaterepo.NewProductColl().Find(deliveryVersion.ProductName)
	if err != nil {
		return nil, fmt.Errorf("failed to find product template %s, err: %v", deliveryVersion.ProductName, err)
	}

	servicesOrder := productTemplate.Services
	if production {
		servicesOrder = productTemplate.ProductionServices
	}

	i := 0
	serviceOrderMap := make(map[string]int)
	for _, serviceGroup := range servicesOrder {
		for _, service := range serviceGroup {
			serviceOrderMap[service] = i
			i++
		}
	}
	slices.SortStableFunc(deliveryDeploys, func(i, j *commonmodels.DeliveryDeploy) int {
		return cmp.Compare(serviceOrderMap[i.ServiceName], serviceOrderMap[j.ServiceName])
	})

	// 将serviceName替换为服务名/服务组件的形式，用于前端展示
	for _, deliveryDeploy := range deliveryDeploys {
		if deliveryDeploy.ContainerName != "" {
			deliveryDeploy.RealServiceName = deliveryDeploy.ServiceName
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

	//distributeInfo
	deliveryDistributeArgs := new(commonrepo.DeliveryDistributeArgs)
	deliveryDistributeArgs.ReleaseID = deliveryVersion.ID.Hex()
	deliveryDistributes, _ := FindDeliveryDistribute(deliveryDistributeArgs, logger)

	releaseInfo.DistributeInfo = deliveryDistributes

	// fill some data for helm delivery releases
	processReleaseRespData(releaseInfo)

	// helm chart version uses distribute info to store version info.
	for _, distribute := range releaseInfo.DistributeInfo {
		// modify each service module info for frontend
		for _, module := range distribute.SubDistributes {
			if module.DistributeType == config.Image {
				module.Image = module.RegistryName
				module.ImageName = util.ExtractImageName(module.RegistryName)
				module.ServiceModule = util.ExtractImageName(module.RegistryName)
			}
		}
	}

	// k8s yaml version uses deploy info to store version info.
	for _, deploy := range releaseInfo.DeployInfo {
		deploy.ImageName = util.ExtractImageName(deploy.Image)
		deploy.ServiceModule = deploy.ContainerName
	}

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

func ListDeliveryVersion(args *ListDeliveryVersionArgs, logger *zap.SugaredLogger) ([]*ReleaseInfo, int, error) {
	versionListArgs := new(commonrepo.DeliveryVersionArgs)
	versionListArgs.ProductName = args.ProjectName
	versionListArgs.WorkflowName = args.WorkflowName
	versionListArgs.TaskID = args.TaskId
	versionListArgs.PerPage = args.PerPage
	versionListArgs.Page = args.Page
	deliveryVersions, total, err := FindDeliveryVersion(versionListArgs, logger)
	if err != nil {
		return nil, 0, err
	}
	names := []string{}
	for _, version := range deliveryVersions {
		if version.WorkflowName == "" {
			continue
		}
		names = append(names, version.WorkflowName)
	}

	// workflows, err := commonrepo.NewWorkflowColl().List(&commonrepo.ListWorkflowOption{Projects: []string{args.ProjectName}, Names: names})
	// if err != nil {
	// 	return nil, 0, err
	// }
	// displayNameMap := map[string]string{}
	// for _, workflow := range workflows {
	// 	displayNameMap[workflow.Name] = workflow.DisplayName
	// }
	// for _, version := range deliveryVersions {
	// 	if name, ok := displayNameMap[version.WorkflowName]; ok {
	// 		version.WorkflowDisplayName = name
	// 	}
	// }

	releaseInfos := make([]*ReleaseInfo, 0)
	for _, deliveryVersion := range deliveryVersions {
		releaseInfo, err := buildListReleaseResp(args.Verbosity, deliveryVersion, &DeliveryVersionFilter{ServiceName: args.ServiceName}, logger)
		if err != nil {
			return nil, 0, err
		}
		if releaseInfo == nil {
			continue
		}
		releaseInfos = append(releaseInfos, releaseInfo)
	}
	return releaseInfos, total, nil
}

// fill release
func processReleaseRespData(release *ReleaseInfo) {
	if release.VersionInfo.Type == setting.DeliveryVersionTypeChart {
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
	} else if release.VersionInfo.Type == setting.DeliveryVersionTypeYaml {
		release.VersionInfo.Progress = buildDeliveryProgressInfo(release.VersionInfo, 0)
	}
}

func buildDeliveryProgressInfo(deliveryVersion *commonmodels.DeliveryVersion, successfulChartCount int) *commonmodels.DeliveryVersionProgress {
	progress := &commonmodels.DeliveryVersionProgress{
		SuccessCount: 0,
		TotalCount:   0,
		UploadStatus: "",
		Error:        "",
	}

	if deliveryVersion.Type == setting.DeliveryVersionTypeChart {
		_, err := checkHelmChartVersionStatus(deliveryVersion)
		if err != nil {
			updateVersionStatus(deliveryVersion.Version, deliveryVersion.ProductName, setting.DeliveryVersionStatusFailed, err.Error())
		}
	} else if deliveryVersion.Type == setting.DeliveryVersionTypeYaml {
		_, err := checkK8SImageVersionStatus(deliveryVersion)
		if err != nil {
			updateVersionStatus(deliveryVersion.Version, deliveryVersion.ProductName, setting.DeliveryVersionStatusFailed, err.Error())
		}
	}

	workflowTaskExist := true
	workflowTask, err := commonrepo.NewworkflowTaskv4Coll().Find(deliveryVersion.WorkflowName, int64(deliveryVersion.TaskID))
	if err != nil {
		if err == mongo.ErrNoDocuments {
			workflowTaskExist = false
		} else {
			err = fmt.Errorf("failed to find workflow task %s, task id %d, err: %s", deliveryVersion.WorkflowName, int64(deliveryVersion.TaskID), err)
			log.Error(err)
			progress.Error = err.Error()
			return progress
		}
	}

	if workflowTaskExist {
		if progress.DeliveryVersionWorkflowStatus == nil {
			progress.DeliveryVersionWorkflowStatus = []commonmodels.DeliveryVersionWorkflowStatus{}
		}

		for _, stage := range workflowTask.Stages {
			for _, job := range stage.Jobs {
				if job.JobType != string(config.JobZadigDistributeImage) {
					continue
				}

				taskJobSpec := &commonmodels.JobTaskFreestyleSpec{}
				if err := commonmodels.IToi(job.Spec, taskJobSpec); err != nil {
					err = fmt.Errorf("failed to convert job spec interface to JobTaskFreestyleSpec, err: %s", err)
					log.Error(err)
					progress.Error = err.Error()
					return progress
				}
				for _, step := range taskJobSpec.Steps {
					if step.StepType == config.StepDistributeImage {
						stepSpec := &stepspec.StepImageDistributeSpec{}
						if err := commonmodels.IToi(step.Spec, stepSpec); err != nil {
							err = fmt.Errorf("failed to convert step spec interface to StepImageDistributeSpec, err: %s", err)
							log.Error(err)
							progress.Error = err.Error()
							return progress
						}

						for _, target := range stepSpec.DistributeTarget {
							progress.DeliveryVersionWorkflowStatus = append(progress.DeliveryVersionWorkflowStatus, commonmodels.DeliveryVersionWorkflowStatus{
								JobName:       job.Name,
								ServiceName:   target.ServiceName,
								ServiceModule: target.ServiceModule,
								TargetImage:   target.TargetImage,
								Status:        job.Status,
							})
						}

						if job.Status == config.StatusPassed {
							progress.SuccessCount += len(stepSpec.DistributeTarget)
						}
						progress.TotalCount += len(stepSpec.DistributeTarget)
					}
				}
			}
		}
	} else {
		progress.SuccessCount = successfulChartCount
		progress.TotalCount = successfulChartCount
	}

	if deliveryVersion.Status == setting.DeliveryVersionStatusSuccess {
		progress.UploadStatus = setting.DeliveryVersionPackageStatusSuccess
		return progress
	}

	if deliveryVersion.Status == setting.DeliveryVersionStatusFailed {
		progress.UploadStatus = setting.DeliveryVersionPackageStatusFailed
		progress.Error = deliveryVersion.Error
		return progress
	}

	if progress.SuccessCount < progress.TotalCount {
		progress.UploadStatus = setting.DeliveryVersionPackageStatusWaiting
		return progress
	}

	progress.UploadStatus = setting.DeliveryVersionPackageStatusUploading
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

func getProductEnvInfo(productName, envName string, production bool) (*commonmodels.Product, error) {
	productInfo, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:       productName,
		EnvName:    envName,
		Production: &production,
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
	basePath := config.LocalTestServicePathWithRevision(serviceObj.ProductName, serviceName, fmt.Sprint(revision))
	if err := commonutil.PreloadServiceManifestsByRevision(basePath, serviceObj, prod.Production); err != nil {
		log.Warnf("failed to get chart of revision: %d for service: %s, use latest version", revision, serviceName)
		// use the latest version when it fails to download the specific version
		basePath = config.LocalTestServicePath(serviceObj.ProductName, serviceName)
		if err = commonutil.PreLoadServiceManifests(basePath, serviceObj, prod.Production); err != nil {
			log.Errorf("failed to load chart info for service %v", serviceObj.ServiceName)
			return "", err
		}
	}

	fullPath := filepath.Join(basePath, serviceObj.ServiceName)
	err := copy.Copy(fullPath, deliveryChartPath)
	if err != nil {
		return "", err
	}

	helmClient, err := helmtool.NewClientFromNamespace(prod.ClusterID, prod.Namespace)
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
			Repo:      container.ImagePath.Repo,
			Namespace: container.ImagePath.Namespace,
			Image:     container.ImagePath.Image,
			Tag:       container.ImagePath.Tag,
		}
		pattern := imageSearchRule.GetSearchingPattern()
		imagePathSpecs = append(imagePathSpecs, pattern)
	}

	imageDetail := &ServiceImageDetails{
		ServiceName: serviceObj.ServiceName,
		Images:      make([]*ImageUrlDetail, 0),
	}

	registrySet := sets.NewString()
	prodSvc := chartData.ProductService
	containerMap := make(map[string]*commonmodels.Container)
	for _, container := range prodSvc.Containers {
		containerMap[container.ImageName] = container
	}
	for _, spec := range imagePathSpecs {
		imageUrl, err := commonutil.GeneImageURI(spec, flatMap)
		if err != nil {
			return nil, nil, err
		}

		imageName := commonutil.ExtractImageName(imageUrl)
		container := containerMap[imageName]
		if container == nil {
			return nil, nil, fmt.Errorf("container not found, imageName: %s", imageName)
		}

		prodImageUrl := container.Image
		imageTag := commonservice.ExtractImageTag(prodImageUrl)

		customTag := ""
		if ct, ok := imageTagMap[imageName]; ok {
			customTag = ct
		} else {
			continue
		}

		if customTag == "" {
			customTag = imageTag
		}

		registryUrl, err := commonservice.ExtractImageRegistry(prodImageUrl)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "failed to parse registry from image uri: %s", prodImageUrl)
		}
		registryUrl = strings.TrimSuffix(registryUrl, "/")

		sourceRegistryID := ""
		// used source registry
		if registry, ok := registryMap[registryUrl]; ok {
			sourceRegistryID = registry.ID.Hex()
			registrySet.Insert(sourceRegistryID)
		} else {
			return nil, nil, fmt.Errorf("registry not found, registryUrl: %s", registryUrl)
		}

		imageDetail.Images = append(imageDetail.Images, &ImageUrlDetail{
			ImageUrl:         prodImageUrl,
			Name:             imageName,
			Tag:              imageTag,
			SourceRegistryID: sourceRegistryID,
			TargetRegistryID: targetRegistry.ID.Hex(),
			CustomTag:        customTag,
		})

		// assign image to values.yaml
		targetImageUrl := util.ReplaceRepo(prodImageUrl, targetRegistry.RegAddr, targetRegistry.Namespace)
		targetImageUrl = util.ReplaceTag(targetImageUrl, customTag)
		replaceValuesMap, err := commonutil.AssignImageData(targetImageUrl, spec)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to pase image uri %s, err %s", targetImageUrl, err)
		}

		// replace image into final merged values.yaml
		retValuesYaml, err = commonutil.ReplaceImage(retValuesYaml, replaceValuesMap)
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

	client, err := commonutil.NewHelmClient(chartRepo)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create chart repo client, repoName: %s", chartRepo.RepoName)
	}

	proxy, err := commonutil.GenHelmChartProxy(chartRepo)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to generate helm chart proxy, repoName: %s", chartRepo.RepoName)
	}

	log.Debugf("pushing chart %s to %s...", filepath.Base(chartPackagePath), chartRepo.URL)
	err = client.PushChart(commonutil.GeneHelmRepo(chartRepo), chartPackagePath, proxy)
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

func CreateK8SDeliveryVersion(args *CreateK8SDeliveryVersionArgs, logger *zap.SugaredLogger) error {
	if args.Retry {
		return RetryCreateK8SDeliveryVersion(args.ProductName, args.Version, logger)
	} else {
		return CreateNewK8SDeliveryVersion(args, logger)
	}
}

func CreateHelmDeliveryVersion(args *CreateHelmDeliveryVersionArgs, logger *zap.SugaredLogger) error {
	if args.Retry {
		return RetryCreateHelmDeliveryVersion(args.ProductName, args.Version, logger)
	} else {
		return CreateNewHelmDeliveryVersion(args, logger)
	}
}

func CheckDeliveryVersion(projectName, deliveryVersionName string) error {
	if len(deliveryVersionName) == 0 {
		return e.ErrCheckDeliveryVersion.AddDesc("版本不能为空")
	}
	if len(projectName) == 0 {
		return e.ErrCheckDeliveryVersion.AddDesc("项目名称不能为空")
	}

	_, err := commonrepo.NewDeliveryVersionColl().Get(&commonrepo.DeliveryVersionArgs{
		ProductName: projectName,
		Version:     deliveryVersionName,
	})
	if !mongodb.IsErrNoDocuments(err) {
		return e.ErrCheckDeliveryVersion.AddErr(fmt.Errorf("版本 %s 已存在", deliveryVersionName))
	}

	return nil
}

// validate yamlInfo, make sure service is in environment
// prepare data set for yaml delivery
func prepareYamlData(yamlDatas []*CreateK8SDeliveryVersionYamlData, productInfo *commonmodels.Product) (map[string]string, error) {
	serviceMap := productInfo.GetServiceMap()
	result := map[string]string{}

	for _, yamlData := range yamlDatas {
		if productService, ok := serviceMap[yamlData.ServiceName]; ok {
			yaml, err := kube.RenderEnvService(productInfo, productService.GetServiceRender(), productService)
			if err != nil {
				return nil, fmt.Errorf("failed to render yaml for service: %s", yamlData.ServiceName)
			}
			result[yamlData.ServiceName] = yaml
		} else {
			return nil, fmt.Errorf("service %s not found in environment", yamlData.ServiceName)
		}
	}
	return result, nil
}

// validate chartInfo, make sure service is in environment
// prepare data set for chart delivery
func prepareChartData(chartDatas []*CreateHelmDeliveryVersionChartData, productInfo *commonmodels.Product) (map[string]*DeliveryChartData, error) {
	serviceMap := productInfo.GetServiceMap()
	chartMap := productInfo.GetChartRenderMap()
	chartDataMap := make(map[string]*DeliveryChartData)

	for _, chartData := range chartDatas {
		if productService, ok := serviceMap[chartData.ServiceName]; ok {
			serviceObj, err := repository.QueryTemplateService(&commonrepo.ServiceFindOption{
				ServiceName: chartData.ServiceName,
				Revision:    productService.Revision,
				Type:        setting.HelmDeployType,
				ProductName: productInfo.ProductName,
			}, productInfo.Production)
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

func buildArtifactTaskArgs(projectName, envName string, imagesMap *sync.Map) (*commonmodels.WorkflowV4, bool, error) {
	name := generateDeliveryWorkflowName(projectName)
	resp := &commonmodels.WorkflowV4{
		Name:             name,
		DisplayName:      name,
		Stages:           nil,
		Project:          projectName,
		CreatedBy:        "system",
		ConcurrencyLimit: 1,
	}

	stage := make([]*commonmodels.WorkflowStage, 0)
	jobs := make([]*commonmodels.Job, 0)

	i := 0
	imagesMap.Range(func(key, value interface{}) bool {
		imageDetail := value.(*ServiceImageDetails)
		for _, image := range imageDetail.Images {
			imageName := commonutil.ExtractImageName(image.ImageUrl)
			targets := []*commonmodels.DistributeTarget{}
			target := &commonmodels.DistributeTarget{
				ServiceName: imageDetail.ServiceName,
				ImageName:   imageName,
				SourceTag:   image.Tag,
				TargetTag:   image.CustomTag,
			}
			targets = append(targets, target)

			jobs = append(jobs, &commonmodels.Job{
				Name:    fmt.Sprintf("distribute-image-%d", i),
				JobType: config.JobZadigDistributeImage,
				Skipped: false,
				Spec: &commonmodels.ZadigDistributeImageJobSpec{
					Source:           config.SourceRuntime,
					JobName:          "",
					SourceRegistryID: image.SourceRegistryID,
					TargetRegistryID: image.TargetRegistryID,
					Targets:          targets,
				},
				RunPolicy:      "",
				ServiceModules: nil,
			})
			i++
		}
		return true
	})

	stage = append(stage, &commonmodels.WorkflowStage{
		Name:     "distribute-image",
		Parallel: false,
		Jobs:     jobs,
	})

	resp.Stages = stage

	return resp, len(jobs) > 0, nil
}

// insert delivery distribution data for single chart, include image and chart
func insertDeliveryDistributions(imageDatas []*task.ImageData, serviceName, chartVersion string, deliveryVersion *commonmodels.DeliveryVersion, args *DeliveryVersionChartData) error {
	for _, image := range imageDatas {
		err := commonrepo.NewDeliveryDistributeColl().Insert(&commonmodels.DeliveryDistribute{
			ReleaseID:      deliveryVersion.ID,
			ServiceName:    image.ImageName, // image name
			ChartName:      serviceName,
			DistributeType: config.Image,
			RegistryName:   image.ImageUrl,
			Namespace:      commonservice.ExtractRegistryNamespace(image.ImageUrl),
			CreatedAt:      time.Now().Unix(),
		})
		if err != nil {
			log.Errorf("failed to insert image distribute data, chartName: %s, err: %s", serviceName, err)
			return fmt.Errorf("failed to insert image distribute data, chartName: %s", serviceName)
		}
	}

	err := commonrepo.NewDeliveryDistributeColl().Insert(&commonmodels.DeliveryDistribute{
		ReleaseID:      deliveryVersion.ID,
		DistributeType: config.Chart,
		ChartName:      serviceName,
		ChartVersion:   chartVersion,
		ChartRepoName:  args.ChartRepoName,
		SubDistributes: nil,
		CreatedAt:      time.Now().Unix(),
	})
	if err != nil {
		log.Errorf("failed to insert chart distribute data, chartName: %s, err: %s", serviceName, err)
		return fmt.Errorf("failed to insert chart distribute data, chartName: %s", serviceName)
	}
	return nil
}

func buildDeliveryImages(productInfo *commonmodels.Product, targetRegistry *commonmodels.RegistryNamespace, registryMap map[string]*commonmodels.RegistryNamespace, deliveryVersion *commonmodels.DeliveryVersion, args *DeliveryVersionYamlData, logger *zap.SugaredLogger) (err error) {
	defer func() {
		if err != nil {
			deliveryVersion.Status = setting.DeliveryVersionStatusFailed
			deliveryVersion.Error = err.Error()
		}
		updateVersionStatus(deliveryVersion.Version, deliveryVersion.ProductName, deliveryVersion.Status, deliveryVersion.Error)
	}()

	for _, yamlData := range args.YamlDatas {
		for _, imageData := range yamlData.ImageDatas {
			deliveryDeploy := new(commonmodels.DeliveryDeploy)
			deliveryDeploy.ReleaseID = deliveryVersion.ID
			deliveryDeploy.StartTime = time.Now().Unix()
			deliveryDeploy.EndTime = time.Now().Unix()
			deliveryDeploy.ServiceName = yamlData.ServiceName
			deliveryDeploy.ContainerName = imageData.ContainerName
			deliveryDeploy.RegistryID = args.ImageRegistryID

			if targetRegistry == nil {
				deliveryDeploy.Image = imageData.Image
			} else {
				regAddr, err := targetRegistry.GetRegistryAddress()
				if err != nil {
					return fmt.Errorf("failed to get registry address, err: %s", err)
				}

				if targetRegistry.RegProvider == config.RegistryProviderECR {
					image := fmt.Sprintf("%s/%s:%s", regAddr, imageData.ImageName, imageData.ImageTag)
					deliveryDeploy.Image = image
				} else {
					image := fmt.Sprintf("%s/%s/%s:%s", regAddr, targetRegistry.Namespace, imageData.ImageName, imageData.ImageTag)
					deliveryDeploy.Image = image
				}
			}

			deliveryDeploy.YamlContents = []string{yamlData.YamlContent}
			//orderedServices
			deliveryDeploy.OrderedServices = productInfo.GetGroupServiceNames()
			deliveryDeploy.CreatedAt = time.Now().Unix()
			deliveryDeploy.DeletedAt = 0
			err = commonrepo.NewDeliveryDeployColl().Insert(deliveryDeploy)
			if err != nil {
				return fmt.Errorf("failed to insert deliveryDeploy, serviceName: %s, imageName: %s, image: %s, err: %v", deliveryDeploy.ServiceName, deliveryDeploy.ContainerName, deliveryDeploy.Image, err)
			}
		}
	}

	// create workflow task to deal with images
	deliveryVersionWorkflowV4, err := generateCustomWorkflowFromDeliveryVersion(productInfo, deliveryVersion, targetRegistry, registryMap, args)
	if err != nil {
		return fmt.Errorf("failed to generate workflow from delivery version, versionName: %s, err: %s", deliveryVersion.Version, err)
	}

	if len(deliveryVersionWorkflowV4.Stages) != 0 {
		createResp, err := workflowservice.CreateWorkflowTaskV4(&workflowservice.CreateWorkflowTaskV4Args{
			Name: "system",
			Type: config.WorkflowTaskTypeDelivery,
		}, deliveryVersionWorkflowV4, logger)
		if err != nil {
			return fmt.Errorf("failed to create delivery version custom workflow task, versionName: %s, err: %s", deliveryVersion.Version, err)
		}

		deliveryVersion.WorkflowName = createResp.WorkflowName
		deliveryVersion.TaskID = int(createResp.TaskID)

		err = commonrepo.NewDeliveryVersionColl().UpdateWorkflowTask(deliveryVersion.Version, deliveryVersion.ProductName, deliveryVersion.WorkflowName, int32(deliveryVersion.TaskID))
		if err != nil {
			logger.Errorf("failed to update delivery version task_id, version: %s, task_id: %s, err: %s", deliveryVersion, deliveryVersion.ProductName, deliveryVersion.TaskID)
		}
	}

	// start a new routine to check task results
	go waitK8SImageVersionDone(deliveryVersion)

	return
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
	workflowV4, notEmpty, err := buildArtifactTaskArgs(deliveryVersion.ProductName, deliveryVersion.ProductEnvInfo.EnvName, imagesDataMap)
	if err != nil {
		return fmt.Errorf("failed to build helm custom artifact task , err: %s", err)

	}
	if notEmpty {
		createResp, err := workflowservice.CreateWorkflowTaskV4(&workflowservice.CreateWorkflowTaskV4Args{
			Name: "system",
			Type: config.WorkflowTaskTypeDelivery,
		}, workflowV4, logger)
		if err != nil {
			return fmt.Errorf("failed to create helm delivery version custom workflow task, versionName: %s, err: %s", deliveryVersion.Version, err)
		}
		deliveryVersion.TaskID = int(createResp.TaskID)
	}

	deliveryVersion.WorkflowName = workflowV4.Name
	err = commonrepo.NewDeliveryVersionColl().UpdateWorkflowTask(deliveryVersion.Version, deliveryVersion.ProductName, deliveryVersion.WorkflowName, int32(deliveryVersion.TaskID))
	if err != nil {
		logger.Errorf("failed to update delivery version task_id, version: %s, task_id: %s, err: %s", deliveryVersion, deliveryVersion.ProductName, deliveryVersion.TaskID)
	}

	// start a new routine to check task results
	go waitHelmChartVersionDone(deliveryVersion)

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

		if versionInfo.Type == setting.DeliveryVersionTypeChart {
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
	}

	err := commonrepo.NewDeliveryVersionColl().UpdateStatusByName(versionName, projectName, status, errStr)
	if err != nil {
		log.Errorf("failed to update version status, name: %s, err: %s", versionName, err)
	}
}

func taskFinished(status config.Status) bool {
	return status == config.StatusPassed || status == config.StatusFailed || status == config.StatusTimeout || status == config.StatusCancelled
}

func waitK8SImageVersionDone(deliveryVersion *commonmodels.DeliveryVersion) {
	waitTimeout := time.After(60 * time.Minute * 1)
	for {
		select {
		case <-waitTimeout:
			updateVersionStatus(deliveryVersion.Version, deliveryVersion.ProductName, setting.DeliveryVersionStatusFailed, "timeout")
			return
		default:
			done, err := checkK8SImageVersionStatus(deliveryVersion)
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

func waitHelmChartVersionDone(deliveryVersion *commonmodels.DeliveryVersion) {
	waitTimeout := time.After(60 * time.Minute * 1)
	for {
		select {
		case <-waitTimeout:
			updateVersionStatus(deliveryVersion.Version, deliveryVersion.ProductName, setting.DeliveryVersionStatusFailed, "timeout")
			return
		default:
			done, err := checkHelmChartVersionStatus(deliveryVersion)
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

func checkK8SImageVersionStatus(deliveryVersion *commonmodels.DeliveryVersion) (bool, error) {
	if deliveryVersion.Status == setting.DeliveryVersionStatusSuccess || deliveryVersion.Status == setting.DeliveryVersionStatusFailed {
		return true, nil
	}

	workflowTaskExist := true
	workflowTask, err := commonrepo.NewworkflowTaskv4Coll().Find(deliveryVersion.WorkflowName, int64(deliveryVersion.TaskID))
	if err != nil {

		if err == mongo.ErrNoDocuments {
			workflowTaskExist = false
		} else {
			return false, fmt.Errorf("failed to find workflow task, workflowName: %s, taskID: %d", deliveryVersion.WorkflowName, deliveryVersion.TaskID)
		}
	}

	done := false
	if workflowTaskExist {
		if len(workflowTask.Stages) != 1 {
			return false, fmt.Errorf("invalid task data, stage length not leagal")
		}
		if workflowTask.Status == config.StatusPassed {
			deliveryVersion.Status = setting.DeliveryVersionStatusSuccess
			done = true
		} else if workflowTask.Status == config.StatusFailed || workflowTask.Status == config.StatusTimeout || workflowTask.Status == config.StatusCancelled {
			deliveryVersion.Status = setting.DeliveryVersionStatusFailed
			done = true
		}
	} else {
		done = true
		deliveryVersion.Status = setting.DeliveryVersionStatusSuccess
	}
	if done {
		updateVersionStatus(deliveryVersion.Version, deliveryVersion.ProductName, deliveryVersion.Status, deliveryVersion.Error)
	}

	return done, nil
}

func checkHelmChartVersionStatus(deliveryVersion *commonmodels.DeliveryVersion) (bool, error) {
	if deliveryVersion.Status == setting.DeliveryVersionStatusSuccess || deliveryVersion.Status == setting.DeliveryVersionStatusFailed {
		return true, nil
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
	chartDataMap := map[string]*CreateHelmDeliveryVersionChartData{}
	for _, chartData := range createArgs.ChartDatas {
		chartDataMap[chartData.ServiceName] = chartData
	}

	// successfully handled charts
	chartDistributes, err := commonrepo.NewDeliveryDistributeColl().Find(&commonrepo.DeliveryDistributeArgs{
		DistributeType: config.Chart,
		ReleaseID:      deliveryVersion.ID.Hex(),
	})
	if err != nil {
		return false, errors.Wrapf(err, "failed to query chart distrubutes, versionName: %s", deliveryVersion.Version)
	}
	insertCharts := sets.NewString()
	for _, distribute := range chartDistributes {
		insertCharts.Insert(distribute.ChartName)
	}

	// successfully handled images
	imageDistributes, err := commonrepo.NewDeliveryDistributeColl().Find(&commonrepo.DeliveryDistributeArgs{
		DistributeType: config.Image,
		ReleaseID:      deliveryVersion.ID.Hex(),
	})
	if err != nil {
		return false, errors.Wrapf(err, "failed to query image distrubutes, versionName: %s", deliveryVersion.Version)
	}
	insertedImages := sets.NewString()
	for _, distribute := range imageDistributes {
		insertedImages.Insert(distribute.ServiceName) // image name
	}

	errorList := &multierror.Error{}

	workflowTaskExist := true
	workflowTask, err := commonrepo.NewworkflowTaskv4Coll().Find(deliveryVersion.WorkflowName, int64(deliveryVersion.TaskID))
	if err != nil {
		if err == mongo.ErrNoDocuments {
			workflowTaskExist = false
		} else {
			return false, fmt.Errorf("failed to find workflow task, workflowName: %s, taskID: %d", deliveryVersion.WorkflowName, deliveryVersion.TaskID)
		}
	}

	// Images
	taskDone := true
	taskSuccess := true
	if workflowTaskExist {
		if !lo.Contains(config.CompletedStatus(), workflowTask.Status) {
			taskDone = false
		}
		if workflowTask.Status != config.StatusPassed {
			taskSuccess = false
		}
		for _, stage := range workflowTask.Stages {
			for _, job := range stage.Jobs {
				taskJobSpec := &commonmodels.JobTaskFreestyleSpec{}
				if err := commonmodels.IToi(job.Spec, taskJobSpec); err != nil {
					continue
				}

				for _, step := range taskJobSpec.Steps {
					if step.StepType == config.StepDistributeImage {
						stepSpec := &stepspec.StepImageDistributeSpec{}
						commonmodels.IToi(step.Spec, stepSpec)

						for _, target := range stepSpec.DistributeTarget {
							if job.Status == "" {
								target.SetTargetImage(stepSpec.TargetRegistry)
							}
							imageName := commonutil.ExtractImageName(target.TargetImage)

							if job.Status == config.StatusPassed {
								if insertedImages.Has(imageName) {
									continue
								}

								err := commonrepo.NewDeliveryDistributeColl().Insert(&commonmodels.DeliveryDistribute{
									DistributeType: config.Image,
									ReleaseID:      deliveryVersion.ID,
									ServiceName:    imageName, // image name
									ChartName:      target.ServiceName,
									RegistryName:   target.TargetImage,
									Namespace:      commonservice.ExtractRegistryNamespace(target.TargetImage),
									CreatedAt:      time.Now().Unix(),
								})
								if err != nil {
									err = fmt.Errorf("failed to insert image distribute data, chartName: %s, err: %s", target.ServiceName, err)
									log.Error(err)
									errorList = multierror.Append(errorList, err)
									continue
								}
								insertedImages.Insert(imageName)
							} else {
								if lo.Contains(config.FailedStatus(), job.Status) {
									errorList = multierror.Append(errorList, fmt.Errorf("failed to build image distribute for service: %s, status: %s, err: %s ", target.ServiceName, job.Status, job.Error))
								}

								if !insertedImages.Has(imageName) {
									err := commonrepo.NewDeliveryDistributeColl().Insert(&commonmodels.DeliveryDistribute{
										DistributeType: config.Image,
										ReleaseID:      deliveryVersion.ID,
										ServiceName:    imageName, // image name
										ChartName:      target.ServiceName,
										RegistryName:   target.TargetImage,
										Namespace:      commonservice.ExtractRegistryNamespace(target.TargetImage),
										CreatedAt:      time.Now().Unix(),
									})
									if err != nil {
										err = fmt.Errorf("failed to insert image distribute data, chartName: %s, err: %s", target.ServiceName, err)
										log.Error(err)
										errorList = multierror.Append(errorList, err)
										continue
									}
									insertedImages.Insert(imageName)
								}
							}
						}
					}
				}
			}
		}
	}

	// Charts
	for _, chartData := range createArgs.ChartDatas {
		if !insertCharts.Has(chartData.ServiceName) {
			err := commonrepo.NewDeliveryDistributeColl().Insert(&commonmodels.DeliveryDistribute{
				ReleaseID:      deliveryVersion.ID,
				DistributeType: config.Chart,
				ChartName:      chartData.ServiceName,
				ChartVersion:   chartData.Version,
				ChartRepoName:  createArgs.ChartRepoName,
				SubDistributes: nil,
				CreatedAt:      time.Now().Unix(),
			})
			if err != nil {
				errorList = multierror.Append(errorList, fmt.Errorf("failed to insert distribute data for service:%s ", chartData.ServiceName))
				continue
			}
			insertCharts.Insert(chartData.ServiceName)
		}
	}

	if taskDone {
		if insertCharts.Len() == len(createArgs.ChartDatas) && taskSuccess {
			deliveryVersion.Status = setting.DeliveryVersionStatusSuccess
		} else {
			deliveryVersion.Status = setting.DeliveryVersionStatusFailed
		}
	}

	if errorList.ErrorOrNil() != nil {
		deliveryVersion.Error = errorList.Error()
	}
	updateVersionStatus(deliveryVersion.Version, deliveryVersion.ProductName, deliveryVersion.Status, deliveryVersion.Error)
	return taskDone, nil
}

func CreateNewK8SDeliveryVersion(args *CreateK8SDeliveryVersionArgs, logger *zap.SugaredLogger) error {
	// prepare data
	productInfo, err := getProductEnvInfo(args.ProductName, args.EnvName, args.Production)
	if err != nil {
		log.Infof("failed to query product info, productName: %s envName %s, err: %s", args.ProductName, args.EnvName, err)
		return e.ErrCreateDeliveryVersion.AddDesc(fmt.Sprintf("failed to query product info, procutName: %s envName %s", args.ProductName, args.EnvName))
	}

	registryMap, err := buildRegistryMap()
	if err != nil {
		return fmt.Errorf("failed to build registry map")
	}

	var targetRegistry *commonmodels.RegistryNamespace
	if len(args.ImageRegistryID) != 0 {
		for _, registry := range registryMap {
			if registry.ID.Hex() == args.ImageRegistryID {
				targetRegistry = registry
				break
			}
		}
		targetRegistryProjectSet := sets.NewString()
		for _, project := range targetRegistry.Projects {
			targetRegistryProjectSet.Insert(project)
		}
		if !targetRegistryProjectSet.Has(productInfo.ProductName) && !targetRegistryProjectSet.Has(setting.AllProjects) {
			return fmt.Errorf("registry %s/%s not support project %s", targetRegistry.RegAddr, targetRegistry.Namespace, productInfo.ProductName)
		}
	}

	for _, yamlData := range args.YamlDatas {
		for _, imageData := range yamlData.ImageDatas {
			if !imageData.Selected {
				continue
			}
			_, err := getImageSourceRegistry(imageData, registryMap)
			if err != nil {
				return fmt.Errorf("failed to check registry, err: %v", err)
			}
		}
	}

	productInfo.ID, _ = primitive.ObjectIDFromHex("")

	workflowName := generateDeliveryWorkflowName(args.ProductName)
	versionObj := &commonmodels.DeliveryVersion{
		Version:        args.Version,
		ProductName:    args.ProductName,
		WorkflowName:   workflowName,
		Type:           setting.DeliveryVersionTypeYaml,
		Desc:           args.Desc,
		Labels:         args.Labels,
		ProductEnvInfo: productInfo,
		Status:         setting.DeliveryVersionStatusCreating,
		CreateArgument: args.DeliveryVersionYamlData,
		CreatedBy:      args.CreateBy,
		CreatedAt:      time.Now().Unix(),
		DeletedAt:      0,
	}

	err = commonrepo.NewDeliveryVersionColl().Insert(versionObj)
	if err != nil {
		logger.Errorf("failed to insert version data, err: %s", err)
		if mongo.IsDuplicateKeyError(err) {
			return e.ErrCreateDeliveryVersion.AddErr(fmt.Errorf("failed to insert delivery version %s, version already exist", versionObj.Version))
		}
		return e.ErrCreateDeliveryVersion.AddErr(fmt.Errorf("failed to insert delivery version: %s, %v", versionObj.Version, err))
	}

	err = buildDeliveryImages(productInfo, targetRegistry, registryMap, versionObj, args.DeliveryVersionYamlData, logger)
	if err != nil {
		return err
	}

	return nil
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
	productInfo, err := getProductEnvInfo(args.ProductName, args.EnvName, args.Production)
	if err != nil {
		log.Infof("failed to query product info, productName: %s envName %s, err: %s", args.ProductName, args.EnvName, err)
		return e.ErrCreateDeliveryVersion.AddDesc(fmt.Sprintf("failed to query product info, procutName: %s envName %s", args.ProductName, args.EnvName))
	}
	chartDataMap, err := prepareChartData(args.ChartDatas, productInfo)
	if err != nil {
		return e.ErrCreateDeliveryVersion.AddErr(err)
	}

	registryMap, err := buildRegistryMap()
	if err != nil {
		return fmt.Errorf("failed to build registry map")
	}

	var targetRegistry *commonmodels.RegistryNamespace
	if len(args.ImageRegistryID) != 0 {
		for _, registry := range registryMap {
			if registry.ID.Hex() == args.ImageRegistryID {
				targetRegistry = registry
				break
			}
		}
		targetRegistryProjectSet := sets.NewString()
		for _, project := range targetRegistry.Projects {
			targetRegistryProjectSet.Insert(project)
		}
		if !targetRegistryProjectSet.Has(productInfo.ProductName) && !targetRegistryProjectSet.Has(setting.AllProjects) {
			return fmt.Errorf("registry %s/%s not support project %s", targetRegistry.RegAddr, targetRegistry.Namespace, productInfo.ProductName)
		}
	}

	for _, chartData := range args.ChartDatas {
		for _, imageData := range chartData.ImageData {
			if !imageData.Selected {
				continue
			}
			_, err := getImageSourceRegistry(imageData, registryMap)
			if err != nil {
				return fmt.Errorf("failed to check registry, err: %v", err)
			}
		}
	}

	productInfo.ID, _ = primitive.ObjectIDFromHex("")

	workflowName := generateDeliveryWorkflowName(args.ProductName)
	versionObj := &commonmodels.DeliveryVersion{
		Version:        args.Version,
		ProductName:    args.ProductName,
		WorkflowName:   workflowName,
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
		if mongo.IsDuplicateKeyError(err) {
			return e.ErrCreateDeliveryVersion.AddErr(fmt.Errorf("failed to insert delivery version %s, version already exist", versionObj.Version))
		}
		return e.ErrCreateDeliveryVersion.AddErr(fmt.Errorf("failed to insert delivery version: %s, err: %v", versionObj.Version, err))
	}

	err = buildDeliveryCharts(chartDataMap, versionObj, args.DeliveryVersionChartData, logger)
	if err != nil {
		return err
	}

	return nil
}

func RetryCreateK8SDeliveryVersion(projectName, versionName string, logger *zap.SugaredLogger) error {
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

	if deliveryVersion.TaskID != 0 {
		err = workflowservice.RetryWorkflowTaskV4(deliveryVersion.WorkflowName, int64(deliveryVersion.TaskID), logger)
		if err != nil {
			return fmt.Errorf("failed to retry workflow task, workflowName: %s, taskID: %d, err: %s", deliveryVersion.WorkflowName, deliveryVersion.TaskID, err)
		}

		// update status
		deliveryVersion.Status = setting.DeliveryVersionStatusRetrying
		updateVersionStatus(deliveryVersion.Version, deliveryVersion.ProductName, deliveryVersion.Status, deliveryVersion.Error)
	} else {
		return fmt.Errorf("no workflow task found for version: %s", deliveryVersion.Version)
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
		return fmt.Errorf("failed to query delivery version data, verisonName: %s, error: %s", versionName, err)
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
	hClient, err := commonutil.NewHelmClient(chartRepo)
	if err != nil {
		return "", err
	}

	chartRef := fmt.Sprintf("%s/%s", chartRepo.RepoName, chartInfo.ChartName)
	return chartTGZFilePath, hClient.DownloadChart(commonutil.GeneHelmRepo(chartRepo), chartRef, chartInfo.ChartVersion, chartTGZFileParent, false)
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
	hClient, err := commonutil.NewHelmClient(chartRepo)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create chart repo client")
	}
	return hClient.FetchIndexYaml(commonutil.GeneHelmRepo(chartRepo))
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

func generateCustomWorkflowFromDeliveryVersion(productInfo *commonmodels.Product, deliveryVersion *commonmodels.DeliveryVersion, targetRegistry *commonmodels.RegistryNamespace, registryMap map[string]*commonmodels.RegistryNamespace, args *DeliveryVersionYamlData) (*commonmodels.WorkflowV4, error) {
	name := generateDeliveryWorkflowName(deliveryVersion.ProductName)
	resp := &commonmodels.WorkflowV4{
		Name:             name,
		DisplayName:      name,
		Stages:           nil,
		Project:          deliveryVersion.ProductName,
		CreatedBy:        "system",
		ConcurrencyLimit: 1,
	}

	stage := make([]*commonmodels.WorkflowStage, 0)
	jobs := make([]*commonmodels.Job, 0)

	registryDatasMap := map[*commonmodels.RegistryNamespace]map[string][]*ImageData{}
	for _, yamlData := range args.YamlDatas {
		for _, imageData := range yamlData.ImageDatas {
			if !imageData.Selected {
				continue
			}
			sourceRegistry, err := getImageSourceRegistry(imageData, registryMap)
			if err != nil {
				return nil, fmt.Errorf("failed to check registry, err: %v", err)
			}

			if registryDatasMap[sourceRegistry] == nil {
				registryDatasMap[sourceRegistry] = map[string][]*ImageData{yamlData.ServiceName: {}}
			}
			if registryDatasMap[sourceRegistry][yamlData.ServiceName] == nil {
				registryDatasMap[sourceRegistry][yamlData.ServiceName] = []*ImageData{}
			}
			registryDatasMap[sourceRegistry][yamlData.ServiceName] = append(registryDatasMap[sourceRegistry][yamlData.ServiceName], imageData)
		}
	}

	serviceMap := productInfo.GetServiceMap()
	serviceNameContainerMap := map[string]map[string]*commonmodels.Container{}
	for serviceName, prodService := range serviceMap {
		for _, container := range prodService.Containers {
			if serviceNameContainerMap[serviceName] == nil {
				serviceNameContainerMap[serviceName] = map[string]*commonmodels.Container{}
			}
			if serviceNameContainerMap[serviceName][container.Name] == nil {
				serviceNameContainerMap[serviceName][container.Name] = container
			}
		}
	}

	i := 0
	for sourceRegistry, serviceNameImageDatasMap := range registryDatasMap {
		for serviceName, imageDatas := range serviceNameImageDatasMap {
			for _, imageData := range imageDatas {
				if !imageData.Selected {
					continue
				}

				if targetRegistry == nil {
					return nil, fmt.Errorf("target registry not appointed")
				}
				sourceContainter := serviceNameContainerMap[serviceName][imageData.ContainerName]
				if sourceContainter == nil {
					return nil, fmt.Errorf("can't find source container: %s", imageData.ContainerName)
				}
				sourceImage := sourceContainter.Image
				sourceTagStr := strings.Split(sourceImage, ":")
				sourceTag := "latest"
				if len(sourceTagStr) == 2 {
					sourceTag = sourceTagStr[1]
				} else if len(sourceTagStr) == 3 {
					sourceTag = sourceTagStr[2]
				}

				targets := []*commonmodels.DistributeTarget{}
				target := &commonmodels.DistributeTarget{
					ServiceName:   serviceName,
					ServiceModule: imageData.ContainerName,
					ImageName:     imageData.ImageName,
					SourceTag:     sourceTag,
					TargetTag:     imageData.ImageTag,
				}
				targets = append(targets, target)

				jobs = append(jobs, &commonmodels.Job{
					Name:    fmt.Sprintf("distribute-image-%d", i),
					JobType: config.JobZadigDistributeImage,
					Skipped: false,
					Spec: &commonmodels.ZadigDistributeImageJobSpec{
						Source:           config.SourceRuntime,
						JobName:          "",
						SourceRegistryID: sourceRegistry.ID.Hex(),
						TargetRegistryID: targetRegistry.ID.Hex(),
						Targets:          targets,
					},
					RunPolicy:      "",
					ServiceModules: nil,
				})
				i++
			}
		}
	}

	if len(jobs) > 0 {
		stage = append(stage, &commonmodels.WorkflowStage{
			Name:     "distribute-image",
			Parallel: false,
			Jobs:     jobs,
		})
		resp.Stages = stage
	}

	return resp, nil
}

func getImageSourceRegistry(imageData *ImageData, registryMap map[string]*commonmodels.RegistryNamespace) (*commonmodels.RegistryNamespace, error) {
	sourceImageTag := ""
	registryURL := strings.TrimSuffix(imageData.Image, fmt.Sprintf("/%s", imageData.ImageName))
	tmpArr := strings.Split(imageData.Image, ":")
	if len(tmpArr) == 2 {
		sourceImageTag = tmpArr[1]
		registryURL = strings.TrimSuffix(imageData.Image, fmt.Sprintf("/%s:%s", imageData.ImageName, sourceImageTag))
	} else if len(tmpArr) == 3 {
		sourceImageTag = tmpArr[2]
		registryURL = strings.TrimSuffix(imageData.Image, fmt.Sprintf("/%s:%s", imageData.ImageName, sourceImageTag))
	} else if len(tmpArr) == 1 {
		// no need to trim
	} else {
		return nil, fmt.Errorf("invalid image: %s", imageData.Image)
	}
	sourceRegistry, ok := registryMap[registryURL]
	if !ok {
		return nil, fmt.Errorf("can't find source registry for image: %s", imageData.Image)
	}
	return sourceRegistry, nil
}

func generateDeliveryWorkflowName(productName string) string {
	return fmt.Sprintf(deliveryVersionWorkflowV4NamingConvention, productName)
}
