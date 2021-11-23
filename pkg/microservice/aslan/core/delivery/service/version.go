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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	cm "github.com/chartmuseum/helm-push/pkg/chartmuseum"
	"github.com/chartmuseum/helm-push/pkg/helm"
	"github.com/hashicorp/go-multierror"
	"github.com/otiai10/copy"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
	chartloader "helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/yaml"

	configbase "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	fsservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/fs"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/types"
)

type CreateHelmDeliveryVersionOption struct {
	EnableOfflineDist bool   `json:"enableOfflineDist"`
	S3StorageID       string `json:"s3StorageID"`
}

type CreateHelmDeliveryVersionChartData struct {
	ServiceName       string `json:"serviceName"`
	Version           string `json:"version"`
	ValuesYamlContent string `json:"valuesYamlContent"`
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
	ChartRepoName   string                                `json:"chartRepoName"`
	ImageRegistryID string                                `json:"imageRegistryID"`
	ChartDatas      []*CreateHelmDeliveryVersionChartData `json:"chartDatas"`
	Options         *CreateHelmDeliveryVersionOption      `json:"options"`
}

type DeliveryChartData struct {
	ChartData  *CreateHelmDeliveryVersionChartData
	ServiceObj *commonmodels.Service
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

func GetDeliveryVersion(args *commonrepo.DeliveryVersionArgs, log *zap.SugaredLogger) (*commonmodels.DeliveryVersion, error) {
	resp, err := commonrepo.NewDeliveryVersionColl().Get(args)
	if err != nil {
		log.Errorf("get deliveryVersion error: %v", err)
		return nil, e.ErrGetDeliveryVersion
	}
	return resp, err
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

func FillDeliveryProgressInfo(deliveryVersion *commonmodels.DeliveryVersion) *commonmodels.DeliveryVersionProgress {
	if deliveryVersion.Type != setting.DeliveryVersionTypeChart {
		return nil
	}
	successfulChartCount := len(deliveryVersion.Charts)

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

func createChartRepoClient(repo *commonmodels.HelmRepo) (*cm.Client, error) {
	client, err := cm.NewClient(
		cm.URL(repo.URL),
		cm.Username(repo.Username),
		cm.Password(repo.Password),
		// need support more auth types
	)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create chart repo client, repoName: %s", repo.RepoName)
	}
	return client, nil
}

func handleSingleChart(chartData *DeliveryChartData, chartRepo *commonmodels.HelmRepo, dir string) error {
	serviceObj := chartData.ServiceObj
	serviceName, revision := serviceObj.ServiceName, serviceObj.Revision
	base := config.LocalServicePathWithRevision(serviceObj.ProductName, serviceName, revision)
	if err := commonservice.PreloadServiceManifestsByRevision(base, serviceObj); err != nil {
		log.Warnf("failed to get chart of revision: %d for service: %s, use latest version", revision, serviceName)
		// use the latest version when it fails to download the specific version
		base = config.LocalServicePath(serviceObj.ProductName, serviceName)
		if err = commonservice.PreLoadServiceManifests(base, serviceObj); err != nil {
			log.Errorf("failed to load chart info for service %v", serviceObj.ServiceName)
			return err
		}
	}

	// push 镜像
	//commonrepo.NewRegistryNamespaceColl().Find(&commonrepo.FindRegOps{})

	fullPath := filepath.Join(base, serviceObj.ServiceName)
	revisionBasePath := config.LocalDeliveryChartPathWithRevision(serviceObj.ProductName, serviceObj.ServiceName, serviceObj.Revision)
	deliveryChartPath := filepath.Join(revisionBasePath, serviceObj.ServiceName)
	err := copy.Copy(fullPath, deliveryChartPath)
	if err != nil {
		return nil
	}

	//load chart info from local storage
	chartRequested, err := chartloader.Load(deliveryChartPath)
	if err != nil {
		return errors.Wrapf(err, "failed to load chart info, path %s", deliveryChartPath)
	}

	//set version and values.yaml content
	chartRequested.Metadata.Name = chartData.ChartData.ServiceName
	chartRequested.Metadata.Version = chartData.ChartData.Version
	chartRequested.Metadata.AppVersion = chartData.ChartData.Version
	if len(chartData.ChartData.ValuesYamlContent) > 0 {
		valuesInfo := make(map[string]interface{})
		if err = yaml.Unmarshal([]byte(chartData.ChartData.ValuesYamlContent), map[string]interface{}{}); err != nil {
			log.Errorf("invalid yaml content, serviceName: %s, yamlContent: %s", serviceObj.ServiceName, chartData.ChartData.ValuesYamlContent)
			return errors.Wrapf(err, "invalid yaml content for service: %s", serviceObj.ServiceName)
		}
		chartRequested.Values = valuesInfo
	}

	chartPackagePath, err := helm.CreateChartPackage(&helm.Chart{Chart: chartRequested}, dir)
	if err != nil {
		return err
	}

	client, err := createChartRepoClient(chartRepo)
	if err != nil {
		return err
	}

	log.Infof("pushing %s to %s...\n", filepath.Base(chartPackagePath), chartRepo.URL)
	resp, err := client.UploadChartPackage(chartPackagePath, false)
	if err != nil {
		return errors.Wrapf(err, "failed to prepare pushing chart: %s", chartPackagePath)
	}
	err = handlePushResponse(resp)
	if err != nil {
		return errors.Wrapf(err, "failed to push chart: %s ", chartPackagePath)
	}
	return nil
}

func handlePushResponse(resp *http.Response) error {
	if resp.StatusCode != 201 && resp.StatusCode != 202 {
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return getChartmuseumError(b, resp.StatusCode)
	}
	log.Infof("push chart to chart repo done")
	return nil
}

func getChartmuseumError(b []byte, code int) error {
	var er struct {
		Error string `json:"error"`
	}
	err := json.Unmarshal(b, &er)
	if err != nil || er.Error == "" {
		return errors.Errorf("%d: could not properly parse response JSON: %s", code, string(b))
	}
	return errors.Errorf("%d: %s", code, er.Error)
}

func makeChartTGZFileDir(productName, versionName string) (string, error) {
	path := getChartTGZDir(productName, versionName)
	if err := os.RemoveAll(path); err != nil {
		if !os.IsExist(err) {
			return "", errors.Wrapf(err, "failed to claer dir for chart tgz files")
		}
	}
	err := os.MkdirAll(path, 0777)
	if err != nil {
		return "", errors.Wrapf(err, "failed to create chart tgz dir for version: %s", versionName)
	}
	return path, nil
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
	chartDataMap := make(map[string]*DeliveryChartData)
	serviceMap := productInfo.GetServiceMap()
	for _, chartDta := range chartDatas {
		if productService, ok := serviceMap[chartDta.ServiceName]; ok {
			serviceObj, err := commonrepo.NewServiceColl().Find(&commonrepo.ServiceFindOption{
				ServiceName: chartDta.ServiceName,
				Revision:    productService.Revision,
				Type:        setting.HelmDeployType,
				ProductName: productInfo.ProductName,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to query service: %s", chartDta.ServiceName)
			}
			chartDataMap[chartDta.ServiceName] = &DeliveryChartData{
				ChartData: chartDta,
				//ProductService: productService,
				ServiceObj: serviceObj,
			}
		} else {
			return nil, fmt.Errorf("service %s not found in environment", chartDta.ServiceName)
		}
	}
	return chartDataMap, nil
}

func buildDeliveryCharts(chartDataMap map[string]*DeliveryChartData, deliveryVersion *commonmodels.DeliveryVersion, args *DeliveryVersionChartData, logger *zap.SugaredLogger) (err error) {
	defer func() {
		if err != nil {
			deliveryVersion.Status = setting.DeliveryVersionStatusFailed
			deliveryVersion.Error = err.Error()
		} else {
			deliveryVersion.Status = setting.DeliveryVersionStatusSuccess
			deliveryVersion.Error = ""
		}
		err = commonrepo.NewDeliveryVersionColl().UpdateStatusByName(deliveryVersion.Version, deliveryVersion.Status, deliveryVersion.Error)
		if err != nil {
			logger.Errorf("failed to update delivery version data, name: %s error: %s", deliveryVersion.Version, err)
		}
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
		log.Infof("failed to query chart-repo info, productName: %s, err: %s", deliveryVersion.ProductName, err)
		return fmt.Errorf("failed to query chart-repo info, productName: %s, repoName: %s", deliveryVersion.ProductName, args.ChartRepoName)
	}

	// push charts to repo
	wg := sync.WaitGroup{}
	for _, chartData := range chartDataMap {
		wg.Add(1)
		go func(cData *DeliveryChartData) {
			defer wg.Done()
			err := handleSingleChart(cData, repoInfo, dir)
			if err != nil {
				logger.Errorf("failed to handle single chart data, serviceName: %s err: %s", cData.ChartData.ServiceName, err)
				appendError(err)
			} else {
				err = commonrepo.NewDeliveryVersionColl().AddDeliveryChart(deliveryVersion.Version, &commonmodels.DeliveryChart{
					Name:    cData.ChartData.ServiceName,
					Version: cData.ChartData.Version,
					Repo:    args.ChartRepoName,
				})
				if err != nil {
					appendError(errors.Wrapf(err, "failed to save delivery chart: %s", cData.ChartData.ServiceName))
				}
			}
		}(chartData)
	}
	wg.Wait()

	if errorList.ErrorOrNil() != nil {
		err = errorList.ErrorOrNil()
		return
	}

	// no need to upload chart packages
	if args.Options == nil || !args.Options.EnableOfflineDist {
		return
	}

	//tar all chart files and send to s3 store
	fsTree := os.DirFS(dir)
	ServiceS3Base := configbase.ObjectStorageDeliveryVersionPath(deliveryVersion.ProductName)
	if err = fsservice.ArchiveAndUploadFilesToSpecifiedS3(fsTree, deliveryVersion.Version, ServiceS3Base, nil, args.Options.S3StorageID, logger); err != nil {
		logger.Errorf("failed to upload chart package files for project %s, err: %s", deliveryVersion.ProductName, err)
		err = errors.Wrapf(err, "failed to upload package file")
		return
	}
	return
}

func CreateNewHelmDeliveryVersion(args *CreateHelmDeliveryVersionArgs, logger *zap.SugaredLogger) error {
	// need appoint chart info
	if len(args.ChartDatas) == 0 {
		return e.ErrCreateDeliveryVersion.AddDesc("no chart info appointed")
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

	//go func() {
	//	var err error
	//	defer func() {
	//		if err != nil {
	//			versionObj.Status = setting.DeliveryVersionStatusFailed
	//			versionObj.Error = err.Error()
	//		} else {
	//			versionObj.Status = setting.DeliveryVersionStatusSuccess
	//			versionObj.Error = ""
	//		}
	//		err = commonrepo.NewDeliveryVersionColl().UpdateStatusByName(versionObj.Version, versionObj.Status, versionObj.Error)
	//		if err != nil {
	//			logger.Errorf("failed to update delivery version data, name: %s error: %s", versionObj.Version, err)
	//		}
	//	}()
	//
	//	var errLock sync.Mutex
	//	errorList := &multierror.Error{}
	//
	//	appendError := func(err error) {
	//		errLock.Lock()
	//		defer errLock.Unlock()
	//		errorList = multierror.Append(errorList, err)
	//	}
	//
	//	// push charts to repo
	//	wg := sync.WaitGroup{}
	//	for _, chartData := range chartDataMap {
	//		wg.Add(1)
	//		go func(cData *DeliveryChartData) {
	//			defer wg.Done()
	//			err := handleSingleChart(cData, repoInfo, dir)
	//			if err != nil {
	//				logger.Errorf("failed to handle single chart data, serviceName: %s err: %s", cData.ChartData.ServiceName, err)
	//				appendError(err)
	//			} else {
	//				err = commonrepo.NewDeliveryVersionColl().AddDeliveryChart(versionObj.Version, &commonmodels.DeliveryChart{
	//					Name:    cData.ChartData.ServiceName,
	//					Version: cData.ChartData.Version,
	//					Repo:    args.ChartRepoName,
	//				})
	//				if err != nil {
	//					appendError(errors.Wrapf(err, "failed to save delivery chart: %s", cData.ChartData.ServiceName))
	//				}
	//			}
	//		}(chartData)
	//	}
	//	wg.Wait()
	//
	//	if errorList.ErrorOrNil() != nil {
	//		err = errorList.ErrorOrNil()
	//		return
	//	}
	//
	//	// no need to upload chart packages
	//	if args.Options == nil || !args.Options.EnableOfflineDist {
	//		return
	//	}
	//
	//	//tar all chart files and send to s3 store
	//	fsTree := os.DirFS(dir)
	//	ServiceS3Base := configbase.ObjectStorageDeliveryVersionPath(args.ProductName)
	//	if err = fsservice.ArchiveAndUploadFilesToSpecifiedS3(fsTree, versionObj.Version, ServiceS3Base, nil, args.Options.S3StorageID, logger); err != nil {
	//		logger.Errorf("failed to upload chart package files for project %s, err: %s", args.ProductName, err)
	//		err = errors.Wrapf(err, "failed to upload package file")
	//		return
	//	}
	//}()

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
		return errors.Wrapf(err, "failed to marshal arguments, versionName: %s err %s", deliveryVersion.Version)
	}
	createArgs := new(DeliveryVersionChartData)
	err = json.Unmarshal(argsBytes, createArgs)
	if err != nil {
		return errors.Wrapf(err, "failed to unMarshal arguments, versionName: %s err %s", deliveryVersion.Version)
	}

	productInfoSnap := deliveryVersion.ProductEnvInfo

	// for charts has been successfully handled, download charts directly
	successCharts := sets.NewString()
	for _, singleChart := range deliveryVersion.Charts {
		// prepare chart data
		_, err := downloadChart(projectName, versionName, singleChart)
		if err != nil {
			log.Errorf("failed to download chart from chart repo, chartName: %s, err: %s", singleChart.Name, err)
			continue
		}
		successCharts.Insert(singleChart.Name)
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
	err = commonrepo.NewDeliveryVersionColl().UpdateStatusByName(deliveryVersion.Version, deliveryVersion.Status, "")
	if err != nil {
		logger.Errorf("failed to update delivery status, name: %s, err: %s", deliveryVersion.Version, err)
		return fmt.Errorf("failed to update delivery status, name: %s", deliveryVersion.Version)
	}

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

func downloadChart(productName, versionName string, chartInfo *commonmodels.DeliveryChart) (string, error) {
	chartTGZName := fmt.Sprintf("%s-%s.tgz", chartInfo.Name, chartInfo.Version)
	chartTGZFileParent := getChartTGZDir(productName, versionName)
	chartTGZFilePath := filepath.Join(chartTGZFileParent, chartTGZName)
	if _, err := os.Stat(chartTGZFilePath); err == nil {
		// local cache exists
		log.Infof("local cache exists, path %s", chartTGZFilePath)
		return chartTGZFilePath, nil
	}

	chartRepo, err := getChartRepoData(chartInfo.Repo)
	if err != nil {
		return "", fmt.Errorf("failed to query chart-repo info, repoName %s", chartInfo.Repo)
	}

	client, err := createChartRepoClient(chartRepo)
	if err != nil {
		return "", err
	}

	if err = os.MkdirAll(chartTGZFileParent, 0644); err != nil {
		return "", errors.Wrapf(err, "failed to craete tgz parent dir")
	}

	out, err := os.Create(chartTGZFilePath)
	if err != nil {
		_ = os.RemoveAll(chartTGZFilePath)
		return "", errors.Wrapf(err, "failed to create chart tgz file")
	}

	response, err := client.DownloadFile(fmt.Sprintf("charts/%s", chartTGZName))
	if err != nil {
		return "", errors.Wrapf(err, "failed to download file")
	}

	if response.StatusCode != 200 {
		return "", errors.Wrapf(err, "download file failed %s", chartTGZName)
	}
	defer func() { _ = response.Body.Close() }()

	b, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "", errors.Wrapf(err, "failed to read response data")
	}

	defer func(out *os.File) {
		_ = out.Close()
	}(out)

	err = ioutil.WriteFile(chartTGZFilePath, b, 0644)
	if err != nil {
		return "", err
	}
	return chartTGZFilePath, nil
}

func getChartConfigInfo(projectName, version string, chartName string, log *zap.SugaredLogger) (*commonmodels.DeliveryChart, error) {
	deliveryVersion, err := GetDeliveryVersion(&commonrepo.DeliveryVersionArgs{
		ProductName: projectName,
		Version:     version,
	}, log)
	if err != nil {
		return nil, err
	}

	var chartInfo *commonmodels.DeliveryChart
	for _, singleChart := range deliveryVersion.Charts {
		if singleChart.Name == chartName {
			chartInfo = singleChart
		}
	}

	if chartInfo == nil {
		return nil, fmt.Errorf("can't find target chart: %s", chartName)
	}
	return chartInfo, nil
}

func DownloadDeliveryChart(projectName, version string, chartName string, log *zap.SugaredLogger) (string, error) {
	chartInfo, err := getChartConfigInfo(projectName, version, chartName, log)
	if err != nil {
		return "", err
	}
	// prepare chart data
	filePath, err := downloadChart(projectName, version, chartInfo)
	if err != nil {
		return "", err
	}
	return filePath, err
}

func preDownloadAndUncompressChart(projectName, versionName, chartName string, log *zap.SugaredLogger) (string, error) {

	chartInfo, err := getChartConfigInfo(projectName, versionName, chartName, log)
	if err != nil {
		return "", err
	}
	dstDir := getChartExpandDir(projectName, versionName)
	dstDir = filepath.Join(dstDir, fmt.Sprintf("%s-%s", chartInfo.Name, chartInfo.Version))

	filePath, err := DownloadDeliveryChart(projectName, versionName, chartName, log)
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
