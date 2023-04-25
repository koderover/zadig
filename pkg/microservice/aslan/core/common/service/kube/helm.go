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
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/notify"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/render"
	"go.uber.org/zap"

	helmclient "github.com/mittwald/go-helm-client"
	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/releaseutil"
	versionedclient "istio.io/client-go/pkg/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	templatemodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonutil "github.com/koderover/zadig/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	helmtool "github.com/koderover/zadig/pkg/tool/helmclient"
	kubeutil "github.com/koderover/zadig/pkg/tool/kube/util"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/types"
	"github.com/koderover/zadig/pkg/util"
	"github.com/koderover/zadig/pkg/util/converter"
	"github.com/koderover/zadig/pkg/util/fs"
)

type ReleaseInstallParam struct {
	ProductName  string
	Namespace    string
	ReleaseName  string
	MergedValues string
	RenderChart  *templatemodels.ServiceRender
	ServiceObj   *commonmodels.Service
	Timeout      int
	DryRun       bool
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

func InstallOrUpgradeHelmChartWithValues(param *ReleaseInstallParam, isRetry bool, helmClient *helmtool.HelmClient) error {
	namespace, valuesYaml, renderChart, serviceObj := param.Namespace, param.MergedValues, param.RenderChart, param.ServiceObj
	base := config.LocalServicePathWithRevision(serviceObj.ProductName, serviceObj.ServiceName, serviceObj.Revision)
	if err := commonutil.PreloadServiceManifestsByRevision(base, serviceObj); err != nil {
		log.Warnf("failed to get chart of revision: %d for service: %s, use latest version",
			serviceObj.Revision, serviceObj.ServiceName)
		// use the latest version when it fails to download the specific version
		base = config.LocalServicePath(serviceObj.ProductName, serviceObj.ServiceName)
		if err = commonutil.PreLoadServiceManifests(base, serviceObj); err != nil {
			log.Errorf("failed to load chart info for service %v", serviceObj.ServiceName)
			return fmt.Errorf("failed to load chart info for service %s", serviceObj.ServiceName)
		}
	}

	chartFullPath := filepath.Join(base, serviceObj.ServiceName)
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
		DryRun:        param.DryRun,
	}
	if isRetry {
		chartSpec.Replace = true
	}
	if param.Timeout > 0 {
		chartSpec.Timeout = time.Second * time.Duration(param.Timeout)
	}

	// If the target environment is a shared environment and a sub env, we need to clear the deployed K8s Service.
	ctx := context.TODO()
	if !chartSpec.DryRun {
		err = EnsureDeletePreCreatedServices(ctx, param.ProductName, param.Namespace, chartSpec, helmClient)
		if err != nil {
			return fmt.Errorf("failed to ensure deleting pre-created K8s Services for product %q in namespace %q: %s", param.ProductName, param.Namespace, err)
		}
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
		if !chartSpec.DryRun {
			err = EnsureZadigServiceByManifest(ctx, param.ProductName, param.Namespace, release.Manifest)
			if err != nil {
				err = errors.WithMessagef(err, "failed to ensure Zadig Service %s", err)
			}
		}
	}

	return err
}

func geneMergedValues(productSvc *commonmodels.ProductService, renderSet *commonmodels.RenderSet, images []string, variableYaml string) (string, error) {
	serviceName := productSvc.ServiceName
	var targetContainers []*commonmodels.Container
	for _, image := range images {
		imageName := commonutil.ExtractImageName(image)
		for _, container := range productSvc.Containers {
			if container.ImageName == imageName {
				container.Image = image
				targetContainers = append(targetContainers, container)
				break
			}
		}
	}

	targetChart := renderSet.GetChartRenderMap()[serviceName]
	if targetChart == nil {
		return "", fmt.Errorf("failed to find chart info %s", serviceName)
	}

	replaceValuesMaps := make([]map[string]interface{}, 0)
	for _, targetContainer := range targetContainers {
		// prepare image replace info
		replaceValuesMap, err := commonutil.AssignImageData(targetContainer.Image, getValidMatchData(targetContainer.ImagePath))
		if err != nil {
			return "", fmt.Errorf("failed to pase image uri %s/%s, err %s", productSvc.ProductName, serviceName, err.Error())
		}
		replaceValuesMaps = append(replaceValuesMaps, replaceValuesMap)
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

	// handle variables
	// turn variables into key-value format to have higher priority
	if len(variableYaml) > 0 {
		flatMaps, err := converter.YamlToFlatMap([]byte(variableYaml))
		if err != nil {
			return "", fmt.Errorf("failed to convert variable yaml, err: %s", err)
		}
		err = targetChart.AbsorbKVS(flatMaps)
		if err != nil {
			return "", fmt.Errorf("failed to absorb kvs, err: %s", err)
		}
	}

	// merge override values and kvs into service's yaml
	mergedValuesYaml, err := helmtool.MergeOverrideValues(replacedValuesYaml, renderSet.DefaultValues, targetChart.GetOverrideYaml(), targetChart.OverrideValues)
	if err != nil {
		return "", fmt.Errorf("failed to merge override values, err: %s", err)
	}

	// replace image into final merged values.yaml
	return commonutil.ReplaceImage(mergedValuesYaml, replaceValuesMaps...)
}

// UpgradeHelmRelease upgrades helm release with some specific images
func UpgradeHelmRelease(product *commonmodels.Product, renderSet *commonmodels.RenderSet, productSvc *commonmodels.ProductService,
	svcTemp *commonmodels.Service, images []string, variableYaml string, timeout int) error {

	replacedMergedValuesYaml, err := geneMergedValues(productSvc, renderSet, images, variableYaml)
	if err != nil {
		return err
	}

	helmClient, err := helmtool.NewClientFromNamespace(product.ClusterID, product.Namespace)
	if err != nil {
		return err
	}

	param := &ReleaseInstallParam{
		ProductName:  svcTemp.ProductName,
		Namespace:    product.Namespace,
		ReleaseName:  util.GeneReleaseName(svcTemp.GetReleaseNaming(), svcTemp.ProductName, product.Namespace, product.EnvName, svcTemp.ServiceName),
		MergedValues: replacedMergedValuesYaml,
		RenderChart:  renderSet.GetChartRenderMap()[svcTemp.ServiceName],
		ServiceObj:   svcTemp,
		Timeout:      timeout,
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

	// for helm services, wo should update deploy info directly
	if err = commonrepo.NewRenderSetColl().Update(renderSet); err != nil {
		log.Errorf("[RenderSet.update] product %s error: %s", product.ProductName, err.Error())
		return fmt.Errorf("failed to update render set, productName %s", product.ProductName)
	}

	if err = commonrepo.NewProductColl().Update(product); err != nil {
		log.Errorf("update product %s error: %s", product.ProductName, err.Error())
		return fmt.Errorf("failed to update product info, name %s", product.ProductName)
	}

	return nil
}

func UninstallServiceByName(helmClient helmclient.Client, serviceName string, env *commonmodels.Product, revision int64, force bool) error {
	revisionSvc, err := commonrepo.NewServiceColl().Find(&commonrepo.ServiceFindOption{
		ServiceName: serviceName,
		Revision:    revision,
		ProductName: env.ProductName,
	})
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

// DeleteHelmServiceFromEnv deletes the service from the environment
// 1. Uninstall related resources
// 2. Delete service info from database
func DeleteHelmServiceFromEnv(userName, requestID string, productInfo *commonmodels.Product, serviceNames []string, log *zap.SugaredLogger) error {
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

	for serviceGroupIndex, serviceGroup := range productInfo.Services {
		var group []*commonmodels.ProductService
		for _, service := range serviceGroup {
			if !deleteServiceSet.Has(service.ServiceName) {
				group = append(group, service)
			} else {
				deletedSvcRevision[service.ServiceName] = service.Revision
			}
		}
		err := commonrepo.NewProductColl().UpdateGroup(productInfo.EnvName, productInfo.ProductName, serviceGroupIndex, group)
		if err != nil {
			log.Errorf("update product error: %v", err)
			return err
		}
	}

	for _, singleName := range serviceNames {
		delete(productInfo.ServiceDeployStrategy, singleName)
	}
	err = commonrepo.NewProductColl().UpdateDeployStrategy(productInfo.EnvName, productInfo.ProductName, productInfo.ServiceDeployStrategy)
	if err != nil {
		log.Errorf("failed to update product deploy strategy, err: %s", err)
	}

	renderset, err := commonrepo.NewRenderSetColl().Find(&commonrepo.RenderSetFindOption{
		Name:        productInfo.Render.Name,
		EnvName:     productInfo.EnvName,
		ProductTmpl: productInfo.ProductName,
		Revision:    productInfo.Render.Revision,
	})
	if err != nil {
		return fmt.Errorf("failed to find renderset: %s/%d, err: %s", productInfo.Render.Name, productInfo.Render.Revision, err)
	}
	rcs := make([]*template.ServiceRender, 0)
	for _, v := range renderset.ChartInfos {
		if !deleteServiceSet.Has(v.ServiceName) {
			rcs = append(rcs, v)
		}
	}
	renderset.ChartInfos = rcs

	// create new renderset
	if err := render.CreateK8sHelmRenderSet(renderset, log); err != nil {
		return fmt.Errorf("failed to create renderset, name %s, err: %s", renderset.Name, err)
	}

	productInfo.Render.Revision = renderset.Revision
	err = commonrepo.NewProductColl().UpdateRender(renderset.EnvName, productInfo.ProductName, productInfo.Render)
	if err != nil {
		return fmt.Errorf("failed to update product render info, renderName: %s, err: %s", productInfo.Render.Name, err)
	}

	go func() {
		failedServices := sync.Map{}
		wg := sync.WaitGroup{}
		for service, revision := range deletedSvcRevision {
			wg.Add(1)
			go func(product *models.Product, serviceName string, revision int64) {
				defer wg.Done()
				templateSvc, err := commonrepo.NewServiceColl().Find(&commonrepo.ServiceFindOption{ServiceName: serviceName, Revision: revision, ProductName: product.ProductName})
				if err != nil {
					failedServices.Store(serviceName, err.Error())
					return
				}
				log.Infof("uninstall release for service: %s", serviceName)
				if !commonutil.ServiceDeployed(serviceName, productInfo.ServiceDeployStrategy) {
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
	if !env.ShareEnv.Enable {
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

	return ensureDeleteZadigService(ctx, env, svc, kclient, istioClient)
}
