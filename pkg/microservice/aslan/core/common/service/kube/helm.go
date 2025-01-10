/*
Copyright 2023 The KodeRover Authors.

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

package kube

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	helmclient "github.com/mittwald/go-helm-client"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/releaseutil"
	versionedclient "istio.io/client-go/pkg/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	templatemodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	helmservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/helm"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/notify"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/repository"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	kubeclient "github.com/koderover/zadig/v2/pkg/shared/kube/client"
	"github.com/koderover/zadig/v2/pkg/tool/cache"
	helmtool "github.com/koderover/zadig/v2/pkg/tool/helmclient"
	kubeutil "github.com/koderover/zadig/v2/pkg/tool/kube/util"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/util"
	"github.com/koderover/zadig/v2/pkg/util/fs"
)

type ReleaseInstallParam struct {
	ProductName    string
	Namespace      string
	ReleaseName    string
	MergedValues   string
	IsChartInstall bool
	RenderChart    *templatemodels.ServiceRender
	ServiceObj     *commonmodels.Service
	ProdService    *commonmodels.ProductService
	Timeout        int
	DryRun         bool
	Production     bool
}

func InstallOrUpgradeHelmChartWithValues(param *ReleaseInstallParam, isRetry bool, helmClient *helmtool.HelmClient) error {
	namespace, valuesYaml, renderChart, serviceObj := param.Namespace, param.MergedValues, param.RenderChart, param.ServiceObj
	base := config.LocalServicePathWithRevision(serviceObj.ProductName, serviceObj.ServiceName, fmt.Sprint(serviceObj.Revision), param.Production)
	if param.IsChartInstall {
		base = config.LocalServicePathWithRevision(serviceObj.ProductName, serviceObj.ServiceName, param.RenderChart.ChartVersion, param.Production)
	}
	if err := commonutil.PreloadServiceManifestsByRevision(base, serviceObj, param.Production); err != nil {
		log.Warnf("failed to get chart of revision: %d for service: %s, use latest version",
			serviceObj.Revision, serviceObj.ServiceName)
		// use the latest version when it fails to download the specific version
		base = config.LocalServicePath(serviceObj.ProductName, serviceObj.ServiceName, param.Production)
		if err = commonutil.PreLoadServiceManifests(base, serviceObj, param.Production); err != nil {
			log.Errorf("failed to load chart info for service %v, production: %v", serviceObj.ServiceName, param.Production)
			return fmt.Errorf("failed to load chart info for service %s", serviceObj.ServiceName)
		}
	}

	chartFullPath := filepath.Join(base, serviceObj.ServiceName)
	if param.IsChartInstall {
		chartFullPath = filepath.Join(base, param.RenderChart.ChartName)
	}
	chartPath, err := fs.RelativeToCurrentPath(chartFullPath)
	if err != nil {
		log.Errorf("Failed to get relative path %s, err: %s", chartFullPath, err)
		return err
	}

	chartSpec := &helmclient.ChartSpec{
		ReleaseName:   param.ReleaseName,
		ChartName:     chartPath,
		Namespace:     namespace,
		Version:       renderChart.ChartVersion,
		ValuesYaml:    valuesYaml,
		UpgradeCRDs:   true,
		CleanupOnFail: true,
		MaxHistory:    10,
	}
	if isRetry {
		chartSpec.Replace = true
	}
	if param.Timeout > 0 {
		chartSpec.Timeout = time.Second * time.Duration(param.Timeout)
	}

	// If the target environment is a shared environment and a sub env, we need to clear the deployed K8s Service.
	ctx := context.TODO()
	err = EnsureDeletePreCreatedServices(ctx, param.ProductName, param.Namespace, chartSpec, helmClient)
	if err != nil {
		return fmt.Errorf("failed to ensure deleting pre-created K8s Services for product %q in namespace %q: %s", param.ProductName, param.Namespace, err)
	}

	helmClient, err = helmClient.Clone()
	if err != nil {
		return fmt.Errorf("failed to clone helm client: %s", err)
	}

	var release *release.Release
	release, err = helmClient.InstallOrUpgradeChart(ctx, chartSpec, nil)
	if err != nil {
		err = errors.WithMessagef(
			err,
			"failed to install or upgrade helm chart %s/%s",
			namespace, serviceObj.ServiceName)
	} else {
		err = EnsureZadigServiceByManifest(ctx, param.ProductName, param.Namespace, release.Manifest)
		if err != nil {
			err = errors.WithMessagef(err, "failed to ensure Zadig Service, err: %s", err)
		}
	}

	return err
}

// GeneMergedValues generate values.yaml used to install or upgrade helm chart, like param in after option -f
// If fullValues is set to true, full values yaml content will be returned, this case is used to preview values when running workflows
func GeneMergedValues(productSvc *commonmodels.ProductService, svcRender *templatemodels.ServiceRender, defaultValues string, images []string, fullValues bool) (string, error) {
	serviceName := productSvc.ServiceName
	var targetContainers []*commonmodels.Container

	imageMap := make(map[string]string)
	for _, image := range images {
		imageMap[commonutil.ExtractImageName(image)] = image
	}

	for _, container := range productSvc.Containers {
		overrideImage, ok := imageMap[container.ImageName]
		if ok {
			container.Image = overrideImage
		}
		targetContainers = append(targetContainers, container)
	}

	targetChart := svcRender

	replaceValuesMaps := make([]map[string]interface{}, 0)
	for _, targetContainer := range targetContainers {
		// prepare image replace info
		replaceValuesMap, err := commonutil.AssignImageData(targetContainer.Image, commonutil.GetValidMatchData(targetContainer.ImagePath))
		if err != nil {
			return "", fmt.Errorf("failed to pase image uri %s/%s, err %s", productSvc.ProductName, serviceName, err.Error())
		}
		replaceValuesMaps = append(replaceValuesMaps, replaceValuesMap)
	}

	imageKVS := make([]*helmtool.KV, 0)
	for _, imageSecs := range replaceValuesMaps {
		for key, value := range imageSecs {
			imageKVS = append(imageKVS, &helmtool.KV{
				Key:   key,
				Value: value,
			})
		}
	}

	// replace image into service's values.yaml
	replacedValuesYaml, err := commonutil.ReplaceImage(targetChart.ValuesYaml, replaceValuesMaps...)
	if err != nil {
		return "", fmt.Errorf("failed to replace image uri %s/%s, err %s", productSvc.ProductName, serviceName, err.Error())

	}
	if replacedValuesYaml == "" {
		return "", fmt.Errorf("failed to set new image uri into service's values.yaml %s/%s", productSvc.ProductName, serviceName)
	}

	// update values.yaml content in chart
	targetChart.ValuesYaml = replacedValuesYaml

	baseValuesYaml := ""
	if fullValues {
		baseValuesYaml = targetChart.ValuesYaml
	}

	// merge override values and kvs into service's yaml
	mergedValuesYaml, err := helmtool.MergeOverrideValues(baseValuesYaml, defaultValues, targetChart.GetOverrideYaml(), targetChart.OverrideValues, imageKVS)
	if err != nil {
		return "", fmt.Errorf("failed to merge override values, err: %s", err)
	}
	return mergedValuesYaml, nil
}

// @todo merge with proceedHelmRelease
// UpgradeHelmRelease upgrades helm release with some specific images
func UpgradeHelmRelease(product *commonmodels.Product, productSvc *commonmodels.ProductService,
	svcTemp *commonmodels.Service, images []string, timeout int, user string) error {
	chartInfo := productSvc.GetServiceRender()

	var (
		err                      error
		releaseName              string
		replacedMergedValuesYaml string
	)

	releaseName = productSvc.ReleaseName
	if productSvc.FromZadig() {
		releaseName = util.GeneReleaseName(svcTemp.GetReleaseNaming(), svcTemp.ProductName, product.Namespace, product.EnvName, svcTemp.ServiceName)
	}

	err = CheckReleaseInstalledByOtherEnv(sets.NewString(releaseName), product)
	if err != nil {
		return err
	}

	if productSvc.FromZadig() {
		releaseName = util.GeneReleaseName(svcTemp.GetReleaseNaming(), svcTemp.ProductName, product.Namespace, product.EnvName, svcTemp.ServiceName)
		replacedMergedValuesYaml, err = GeneMergedValues(productSvc, chartInfo, product.DefaultValues, images, false)
		if err != nil {
			return fmt.Errorf("failed to gene merged values, err: %s", err)
		}
	} else {
		releaseName = productSvc.ReleaseName
		svcTemp = &commonmodels.Service{
			ServiceName: releaseName,
			ProductName: product.ProductName,
			HelmChart: &commonmodels.HelmChart{
				Name:    chartInfo.ChartName,
				Repo:    chartInfo.ChartRepo,
				Version: chartInfo.ChartVersion,
			},
		}

		replacedMergedValuesYaml, err = helmtool.MergeOverrideValues("", product.DefaultValues, chartInfo.GetOverrideYaml(), chartInfo.OverrideValues, nil)
		if err != nil {
			return fmt.Errorf("failed to merge override values, err: %s", err)
		}

		chartRepo, err := commonrepo.NewHelmRepoColl().Find(&commonrepo.HelmRepoFindOption{RepoName: chartInfo.ChartRepo})
		if err != nil {
			return fmt.Errorf("failed to query chart-repo info, productName: %s, repoName: %s", product.ProductName, chartInfo.ChartRepo)
		}

		chartRef := fmt.Sprintf("%s/%s", chartInfo.ChartRepo, chartInfo.ChartName)
		localPath := config.LocalServicePathWithRevision(product.ProductName, releaseName, chartInfo.ChartVersion, product.Production)
		// remove local file to untar
		_ = os.RemoveAll(localPath)

		hClient, err := helmtool.NewClient()
		if err != nil {
			return err
		}
		err = hClient.DownloadChart(commonutil.GeneHelmRepo(chartRepo), chartRef, chartInfo.ChartVersion, localPath, true)
		if err != nil {
			return fmt.Errorf("failed to download chart, chartName: %s, chartRepo: %+v, err: %s", chartInfo.ChartName, chartRepo.RepoName, err)
		}
	}

	helmClient, err := helmtool.NewClientFromNamespace(product.ClusterID, product.Namespace)
	if err != nil {
		return err
	}

	param := &ReleaseInstallParam{
		ProductName:  svcTemp.ProductName,
		Namespace:    product.Namespace,
		ReleaseName:  releaseName,
		MergedValues: replacedMergedValuesYaml,
		RenderChart:  chartInfo,
		ServiceObj:   svcTemp,
		Timeout:      timeout,
		Production:   product.Production,
	}
	if !productSvc.FromZadig() {
		param.IsChartInstall = true
	}

	ensureUpgrade := func() error {
		hrs, errHistory := helmClient.ListReleaseHistory(param.ReleaseName, 10)
		if errHistory != nil {
			// list history should not block deploy operation, error will be logged instead of returned
			return nil
		}
		if len(hrs) == 0 {
			return nil
		}
		releaseutil.Reverse(hrs, releaseutil.SortByRevision)
		rel := hrs[0]

		if rel.Info.Status.IsPending() {
			return fmt.Errorf("failed to upgrade release: %s with exceptional status: %s", param.ReleaseName, rel.Info.Status)
		}
		return nil
	}

	err = ensureUpgrade()
	if err != nil {
		return err
	}

	// when replace image, should not wait
	err = InstallOrUpgradeHelmChartWithValues(param, false, helmClient)
	if err != nil {
		return err
	}

	err = helmservice.UpdateHelmServiceInEnv(product, productSvc, user)
	return err
}

func UninstallServiceByName(helmClient helmclient.Client, serviceName string, env *commonmodels.Product, revision int64, force bool) error {
	revisionSvc, err := repository.QueryTemplateService(&commonrepo.ServiceFindOption{
		ServiceName: serviceName,
		Revision:    revision,
		ProductName: env.ProductName,
	}, env.Production)
	if err != nil {
		return fmt.Errorf("failed to find service: %s with revision: %d, err: %s", serviceName, revision, err)
	}
	return UninstallService(helmClient, env, revisionSvc, force)
}

// UninstallService uninstall release deployed by zadig
func UninstallService(helmClient helmclient.Client, env *commonmodels.Product, revisionSvc *commonmodels.Service, force bool) error {
	releaseName := util.GeneReleaseName(revisionSvc.GetReleaseNaming(), env.ProductName, env.Namespace, env.EnvName, revisionSvc.ServiceName)

	err := EnsureDeleteZadigServiceByHelmRelease(context.TODO(), env, releaseName, helmClient)
	if err != nil {
		return fmt.Errorf("failed to ensure delete Zadig Service by helm release: %s", err)
	}

	return helmClient.UninstallRelease(&helmclient.ChartSpec{
		ReleaseName: releaseName,
		Namespace:   env.Namespace,
		Wait:        true,
		Force:       force,
		Timeout:     time.Second * 10,
	})
}

// UninstallService uninstall release deployed by zadig
func UninstallRelease(helmClient helmclient.Client, env *commonmodels.Product, releaseName string, force bool) error {
	err := EnsureDeleteZadigServiceByHelmRelease(context.TODO(), env, releaseName, helmClient)
	if err != nil {
		return fmt.Errorf("failed to ensure delete Zadig Service by helm release: %s", err)
	}

	return helmClient.UninstallRelease(&helmclient.ChartSpec{
		ReleaseName: releaseName,
		Namespace:   env.Namespace,
		Wait:        true,
		Force:       force,
		Timeout:     time.Second * 10,
	})
}

// 1. Uninstall related resources
// 2. Delete service info from database
func DeleteHelmReleaseFromEnv(userName, requestID string, productInfo *commonmodels.Product, releaseNames []string, log *zap.SugaredLogger) error {

	helmSvcOfflineLock := cache.NewRedisLock(fmt.Sprintf("product_svc_offline:%s:%s", productInfo.ProductName, productInfo.EnvName))

	helmSvcOfflineLock.Lock()
	defer helmSvcOfflineLock.Unlock()

	log.Infof("remove releases from env, releaseNames: %v", releaseNames)

	restConfig, err := kubeclient.GetRESTConfig(config.HubServerAddress(), productInfo.ClusterID)
	if err != nil {
		return err
	}
	helmClient, err := helmtool.NewClientFromRestConf(restConfig, productInfo.Namespace)
	if err != nil {
		return err
	}

	ctx := context.TODO()
	kclient, err := kubeclient.GetKubeClient(config.HubServerAddress(), productInfo.ClusterID)
	if err != nil {
		return err
	}

	istioClient, err := versionedclient.NewForConfig(restConfig)
	if err != nil {
		return err
	}

	releaseNameSet := sets.NewString(releaseNames...)
	prodSvcMap := productInfo.GetServiceMap()
	prodChartSvcMap := productInfo.GetChartServiceMap()
	releaseToServiceNameMap, err := commonutil.GetReleaseNameToServiceNameMap(productInfo)
	if err != nil {
		return fmt.Errorf("failed to get release name to service name map, err: %s", err)
	}
	serviceNameToProdSvcMap := make(map[string]*commonmodels.ProductService)
	releaseNameToChartProdSvcMap := make(map[string]*commonmodels.ProductService)
	for releaseName, serviceName := range releaseToServiceNameMap {
		if !releaseNameSet.Has(releaseName) {
			continue
		}
		if prodSvcMap[serviceName] == nil {
			releaseNameToChartProdSvcMap[releaseName] = prodChartSvcMap[releaseName]
		} else {
			if prodSvcMap[serviceName] == nil {
				return fmt.Errorf("failed to find service %s(release %s) in product %s", serviceName, releaseName, productInfo.ProductName)
			}
			serviceNameToProdSvcMap[serviceName] = prodSvcMap[serviceName]
		}
	}

	newSevices := make([][]*commonmodels.ProductService, 0)
	for _, serviceGroup := range productInfo.Services {
		var group []*commonmodels.ProductService
		for _, service := range serviceGroup {
			if !service.FromZadig() {
				if _, ok := releaseNameToChartProdSvcMap[service.ReleaseName]; !ok {
					group = append(group, service)
				}
			} else {
				if _, ok := serviceNameToProdSvcMap[service.ServiceName]; !ok {
					group = append(group, service)
				}
			}
		}
		newSevices = append(newSevices, group)
	}
	productInfo.Services = newSevices
	err = helmservice.UpdateHelmAllServicesInEnv(productInfo.ProductName, productInfo.EnvName, productInfo.Services, productInfo.Production)
	if err != nil {
		log.Errorf("UpdateHelmProductServices error: %v", err)
		return err
	}

	// services deployed by zadig
	for _, prodSvc := range serviceNameToProdSvcMap {
		if !commonutil.ServiceDeployed(prodSvc.ServiceName, productInfo.ServiceDeployStrategy) {
			continue
		}
		delete(productInfo.ServiceDeployStrategy, prodSvc.ServiceName)
	}

	for _, prodSvc := range releaseNameToChartProdSvcMap {
		if !commonutil.ReleaseDeployed(prodSvc.ReleaseName, productInfo.ServiceDeployStrategy) {
			continue
		}
		delete(productInfo.ServiceDeployStrategy, commonutil.GetReleaseDeployStrategyKey(prodSvc.ReleaseName))
	}

	err = commonrepo.NewProductColl().UpdateDeployStrategy(productInfo.EnvName, productInfo.ProductName, productInfo.ServiceDeployStrategy)
	if err != nil {
		log.Errorf("failed to update product deploy strategy, err: %s", err)
	}

	go func() {
		failedServices := sync.Map{}
		wg := sync.WaitGroup{}

		for _, prodSvc := range serviceNameToProdSvcMap {
			if !commonutil.ServiceDeployed(prodSvc.ServiceName, productInfo.ServiceDeployStrategy) {
				continue
			}
			wg.Add(1)
			go func(product *models.Product, prodSvc *commonmodels.ProductService) {
				defer wg.Done()
				templateSvc, err := repository.QueryTemplateService(&commonrepo.ServiceFindOption{ServiceName: prodSvc.ServiceName, Revision: prodSvc.Revision, ProductName: product.ProductName}, productInfo.Production)
				if err != nil {
					failedServices.Store(prodSvc.ServiceName, err.Error())
					return
				}
				if errUninstall := UninstallService(helmClient, productInfo, templateSvc, false); errUninstall != nil {
					errStr := fmt.Sprintf("helm uninstall service %s err: %s", prodSvc.ServiceName, errUninstall)
					failedServices.Store(prodSvc.ServiceName, errStr)
					log.Error(errStr)
				}
			}(productInfo, prodSvc)
		}
		for releaseName, prodSvc := range releaseNameToChartProdSvcMap {
			if !commonutil.ReleaseDeployed(prodSvc.ReleaseName, productInfo.ServiceDeployStrategy) {
				continue
			}
			wg.Add(1)
			go func(product *models.Product, releaseName string) {
				defer wg.Done()
				if errUninstall := UninstallRelease(helmClient, productInfo, releaseName, false); errUninstall != nil {
					errStr := fmt.Sprintf("helm uninstall release %s err: %s", releaseName, errUninstall)
					failedServices.Store(releaseName, errStr)
					log.Error(errStr)
				}
			}(productInfo, releaseName)
		}
		wg.Wait()
		errList := make([]string, 0)
		failedServices.Range(func(key, value interface{}) bool {
			errList = append(errList, value.(string))
			return true
		})
		// send err message to user
		if len(errList) > 0 {
			title := fmt.Sprintf("[%s] 的 [%s] 环境服务删除失败", productInfo.ProductName, productInfo.EnvName)
			notify.SendErrorMessage(userName, title, requestID, errors.New(strings.Join(errList, "\n")), log)
		}

		if productInfo.ShareEnv.Enable && !productInfo.ShareEnv.IsBase {
			err = EnsureGrayEnvConfig(ctx, productInfo, kclient, istioClient)
			if err != nil {
				log.Errorf("Failed to ensure gray env config: %s", err)
			}
		} else if productInfo.IstioGrayscale.Enable && !productInfo.IstioGrayscale.IsBase {
			err = EnsureFullPathGrayScaleConfig(ctx, productInfo, kclient, istioClient)
			if err != nil {
				log.Errorf("Failed to ensure full path gray scale config: %s", err)
			}
		}
	}()
	return nil
}

// DeleteHelmServiceFromEnv deletes the service from the environment
// 1. Uninstall related resources
// 2. Delete service info from database
func DeleteHelmServiceFromEnv(userName, requestID string, productInfo *commonmodels.Product, serviceNames []string, log *zap.SugaredLogger) error {

	helmSvcOfflineLock := cache.NewRedisLock(fmt.Sprintf("product_svc_offline:%s:%s", productInfo.ProductName, productInfo.EnvName))

	helmSvcOfflineLock.Lock()
	defer helmSvcOfflineLock.Unlock()

	log.Infof("remove svc from env, svc: %v", serviceNames)

	restConfig, err := kubeclient.GetRESTConfig(config.HubServerAddress(), productInfo.ClusterID)
	if err != nil {
		return err
	}
	helmClient, err := helmtool.NewClientFromRestConf(restConfig, productInfo.Namespace)
	if err != nil {
		return err
	}

	ctx := context.TODO()
	kclient, err := kubeclient.GetKubeClient(config.HubServerAddress(), productInfo.ClusterID)
	if err != nil {
		return err
	}

	istioClient, err := versionedclient.NewForConfig(restConfig)
	if err != nil {
		return err
	}

	deleteServiceSet := sets.NewString(serviceNames...)
	deletedSvcRevision := make(map[string]int64)

	newServices := make([][]*commonmodels.ProductService, 0)
	for _, serviceGroup := range productInfo.Services {
		var group []*commonmodels.ProductService
		for _, service := range serviceGroup {
			if !deleteServiceSet.Has(service.ServiceName) {
				group = append(group, service)
			} else {
				deletedSvcRevision[service.ServiceName] = service.Revision
			}
		}
		newServices = append(newServices, group)
	}
	productInfo.Services = newServices
	err = helmservice.UpdateHelmAllServicesInEnv(productInfo.ProductName, productInfo.EnvName, productInfo.Services, productInfo.Production)
	if err != nil {
		err = fmt.Errorf("UpdateHelmProductServices error: %v", err)
		log.Error(err)
		return err
	}

	// services deployed by zadig
	deployedSvcs := sets.NewString()
	for _, svc := range serviceNames {
		if !commonutil.ServiceDeployed(svc, productInfo.ServiceDeployStrategy) {
			continue
		}
		deployedSvcs.Insert(svc)
	}

	for _, singleName := range serviceNames {
		delete(productInfo.ServiceDeployStrategy, singleName)
	}
	err = commonrepo.NewProductColl().UpdateDeployStrategy(productInfo.EnvName, productInfo.ProductName, productInfo.ServiceDeployStrategy)
	if err != nil {
		log.Errorf("failed to update product deploy strategy, err: %s", err)
	}

	go func() {
		failedServices := sync.Map{}
		wg := sync.WaitGroup{}
		for service, revision := range deletedSvcRevision {
			if !deployedSvcs.Has(service) {
				continue
			}
			wg.Add(1)
			go func(product *models.Product, serviceName string, revision int64) {
				defer wg.Done()
				templateSvc, err := repository.QueryTemplateService(&commonrepo.ServiceFindOption{ServiceName: serviceName, Revision: revision, ProductName: product.ProductName}, productInfo.Production)
				if err != nil {
					failedServices.Store(serviceName, err.Error())
					return
				}
				if errUninstall := UninstallService(helmClient, productInfo, templateSvc, false); errUninstall != nil {
					errStr := fmt.Sprintf("helm uninstall service %s err: %s", serviceName, errUninstall)
					failedServices.Store(serviceName, errStr)
					log.Error(errStr)
				}
			}(productInfo, service, revision)
		}
		wg.Wait()
		errList := make([]string, 0)
		failedServices.Range(func(key, value interface{}) bool {
			errList = append(errList, value.(string))
			return true
		})
		// send err message to user
		if len(errList) > 0 {
			title := fmt.Sprintf("[%s] 的 [%s] 环境服务删除失败", productInfo.ProductName, productInfo.EnvName)
			notify.SendErrorMessage(userName, title, requestID, errors.New(strings.Join(errList, "\n")), log)
		}

		if productInfo.ShareEnv.Enable && !productInfo.ShareEnv.IsBase {
			err = EnsureGrayEnvConfig(ctx, productInfo, kclient, istioClient)
			if err != nil {
				log.Errorf("Failed to ensure gray env config: %s", err)
			}
		}
	}()
	return nil
}

func EnsureDeleteZadigServiceByHelmRelease(ctx context.Context, env *commonmodels.Product, releaseName string, helmClient helmclient.Client) error {
	if !env.ShareEnv.Enable && !env.IstioGrayscale.Enable {
		return nil
	}

	release, err := helmClient.GetRelease(releaseName)
	if err != nil {
		return fmt.Errorf("failed to get release %q in namespace %q: %s", releaseName, env.Namespace, err)
	}

	svcNames, err := kubeutil.GetSvcNamesFromManifest(release.Manifest)
	if err != nil {
		return fmt.Errorf("failed to get Service names from manifest: %s", err)
	}

	kclient, err := kubeclient.GetKubeClient(config.HubServerAddress(), env.ClusterID)
	if err != nil {
		return fmt.Errorf("failed to get kube client: %s", err)
	}

	restConfig, err := kubeclient.GetRESTConfig(config.HubServerAddress(), env.ClusterID)
	if err != nil {
		return fmt.Errorf("failed to get rest config: %s", err)
	}

	istioClient, err := versionedclient.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to get istio client: %s", err)
	}

	for _, svcName := range svcNames {
		err = EnsureDeleteZadigServiceBySvcName(ctx, env, svcName, kclient, istioClient)
		if err != nil {
			return fmt.Errorf("failed to ensure deleting Zadig service by service name %q for env %q of product %q: %s", svcName, env.EnvName, env.ProductName, err)
		}
	}

	return nil
}

func EnsureDeleteK8sService(ctx context.Context, ns, svcName string, kclient client.Client, systemCreatedOnly bool) error {
	svc := &corev1.Service{}
	err := kclient.Get(ctx, client.ObjectKey{
		Name:      svcName,
		Namespace: ns,
	}, svc)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}

	if systemCreatedOnly && !(svc.Labels != nil && svc.Labels[types.ZadigLabelKeyGlobalOwner] == types.Zadig) {
		return nil
	}

	deleteOption := metav1.DeletePropagationBackground
	return kclient.Delete(ctx, svc, &client.DeleteOptions{
		PropagationPolicy: &deleteOption,
	})
}

func EnsureDeleteZadigServiceBySvcName(ctx context.Context, env *commonmodels.Product, svcName string, kclient client.Client, istioClient versionedclient.Interface) error {
	svc := &corev1.Service{}
	err := kclient.Get(ctx, client.ObjectKey{
		Name:      svcName,
		Namespace: env.Namespace,
	}, svc)
	if err != nil {
		return fmt.Errorf("failed to find Service %q in namespace %q: %s", svcName, env.Namespace, err)
	}

	if env.ShareEnv.Enable {
		return EnsureDeleteZadigService(ctx, env, svc, kclient, istioClient)
	} else if env.IstioGrayscale.Enable {
		return EnsureDeleteGrayscaleService(ctx, env, svc, kclient, istioClient)
	}
	return nil
}
