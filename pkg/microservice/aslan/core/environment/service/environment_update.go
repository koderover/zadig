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

package service

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-multierror"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	templatemodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	helmtool "github.com/koderover/zadig/pkg/tool/helmclient"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/util"
	helmclient "github.com/mittwald/go-helm-client"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

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

func InstallService(helmClient *helmtool.HelmClient, param *ReleaseInstallParam) error {
	handler := func(serviceObj *commonmodels.Service, isRetry bool, log *zap.SugaredLogger) error {
		errInstall := installOrUpgradeHelmChartWithValues(param, isRetry, helmClient)
		if errInstall != nil {
			log.Errorf("failed to upgrade service: %s, namespace: %s, isRetry: %v, err: %s", serviceObj.ServiceName, param.Namespace, isRetry, errInstall)
			return errors.Wrapf(errInstall, "failed to install or upgrade service %s", serviceObj.ServiceName)
		}
		return nil
	}
	errList := batchExecutorWithRetry(3, time.Millisecond*2500, []*commonmodels.Service{param.serviceObj}, handler, log.SugaredLogger())
	if len(errList) > 0 {
		return errList[0]
	}
	return nil
}

func setProductServiceError(productInfo *commonmodels.Product, serviceName string, err error) error {
	foundSvc := false
	// update service revision data in product
	for groupIndex, svcGroup := range productInfo.Services {
		for _, svc := range svcGroup {
			if svc.ServiceName == serviceName {
				if err != nil {
					svc.Error = err.Error()
				} else {
					svc.Error = ""
				}
				foundSvc = true
				break
			}
		}
		if foundSvc {
			if err := commonrepo.NewProductColl().UpdateGroup(productInfo.EnvName, productInfo.ProductName, groupIndex, svcGroup); err != nil {
				return fmt.Errorf("failed to update service group: %s:%s, err: %s ", productInfo.ProductName, productInfo.EnvName, err)
			}
			break
		}
	}
	return nil
}

func updateServiceRevisionInProduct(productInfo *commonmodels.Product, serviceName string, serviceRevision int64) error {
	foundSvc := false
	// update service revision data in product
	for groupIndex, svcGroup := range productInfo.Services {
		for _, svc := range svcGroup {
			if svc.ServiceName == serviceName {
				svc.Revision = serviceRevision
				foundSvc = true
				break
			}
		}
		if foundSvc {
			if err := commonrepo.NewProductColl().UpdateGroup(productInfo.EnvName, productInfo.ProductName, groupIndex, svcGroup); err != nil {
				return fmt.Errorf("failed to update service group: %s:%s, err: %s ", productInfo.ProductName, productInfo.EnvName, err)
			}
			break
		}
	}
	return nil
}

// reInstallHelmServiceInEnv uninstall the service and reinstall to update releaseNaming rule
func reInstallHelmServiceInEnv(productInfo *commonmodels.Product, templateSvc *commonmodels.Service) (finalError error) {
	productSvcMap, serviceName := productInfo.GetServiceMap(), templateSvc.ServiceName
	productSvc, ok := productSvcMap[serviceName]
	// service not applied in this environment
	if !ok {
		return nil
	}

	var err error
	defer func() {
		finalError = setProductServiceError(productInfo, templateSvc.ServiceName, err)
	}()

	renderInfo, errRender := commonrepo.NewRenderSetColl().Find(&commonrepo.RenderSetFindOption{
		ProductTmpl: productInfo.ProductName,
		EnvName:     productInfo.EnvName,
		Name:        productInfo.Render.Name,
		Revision:    productInfo.Render.Revision})
	if errRender != nil {
		err = fmt.Errorf("failed to find renderset, %s:%d, err: %s", productInfo.Render.Name, productInfo.Render.Revision, errRender)
		return
	}

	var renderChart *templatemodels.RenderChart
	for _, _renderChart := range renderInfo.ChartInfos {
		if _renderChart.ServiceName == serviceName {
			renderChart = _renderChart
		}
	}
	if renderChart == nil {
		err = fmt.Errorf("failed to find renderchart: %s", renderChart.ServiceName)
		return
	}

	helmClient, errCreateClient := helmtool.NewClientFromNamespace(productInfo.ClusterID, productInfo.Namespace)
	if errCreateClient != nil {
		err = fmt.Errorf("failed to create helm client, err: %s", errCreateClient)
		return
	}

	prodSvcTemp, errFindProd := commonrepo.NewServiceColl().Find(&commonrepo.ServiceFindOption{ServiceName: serviceName, Revision: productSvc.Revision, ProductName: productInfo.ProductName})
	if errFindProd != nil {
		err = fmt.Errorf("failed to get service: %s with revision: %d, err: %s", serviceName, productSvc.Revision, errFindProd)
		return
	}

	// nothing would happen if release naming rule are same
	if prodSvcTemp.GetReleaseNaming() == templateSvc.GetReleaseNaming() {
		return nil
	}

	if err = UninstallService(helmClient, productInfo, prodSvcTemp, true); err != nil {
		err = fmt.Errorf("helm uninstall service: %s in namespace: %s, err: %s", serviceName, productInfo.Namespace, err)
		return
	}

	param, errBuildParam := buildInstallParam(productInfo.Namespace, productInfo.EnvName, renderInfo.DefaultValues, renderChart, templateSvc)
	if errBuildParam != nil {
		err = fmt.Errorf("failed to generate install param, service: %s, namespace: %s, err: %s", templateSvc.ServiceName, productInfo.Namespace, errBuildParam)
		return
	}

	err = InstallService(helmClient, param)
	if err != nil {
		return
	}

	err = updateServiceRevisionInProduct(productInfo, templateSvc.ServiceName, templateSvc.Revision)
	return
}

func updateHelmServiceInEnv(userName string, productInfo *commonmodels.Product, templateSvc *commonmodels.Service) (finalError error) {
	productSvcMap, serviceName := productInfo.GetServiceMap(), templateSvc.ServiceName
	// service not applied in this environment
	if _, ok := productSvcMap[serviceName]; !ok {
		return nil
	}

	var err error
	defer func() {
		finalError = setProductServiceError(productInfo, templateSvc.ServiceName, err)
	}()

	renderInfo, errFindRender := commonrepo.NewRenderSetColl().Find(&commonrepo.RenderSetFindOption{
		ProductTmpl: productInfo.ProductName,
		EnvName:     productInfo.EnvName,
		Name:        productInfo.Render.Name,
		Revision:    productInfo.Render.Revision})
	if errFindRender != nil {
		err = fmt.Errorf("failed to find renderset, %s:%d, err: %s", productInfo.Render.Name, productInfo.Render.Revision, errFindRender)
		return
	}

	var renderChart *templatemodels.RenderChart
	for _, _renderChart := range renderInfo.ChartInfos {
		if _renderChart.ServiceName == serviceName {
			renderChart = _renderChart
		}
	}
	if renderChart == nil {
		err = fmt.Errorf("failed to find renderchart for service: %s", renderChart.ServiceName)
		return
	}

	chartArg := &commonservice.RenderChartArg{}
	chartArg.FillRenderChartModel(renderChart, renderChart.ChartVersion)
	newRenderSet, errDiffRender := diffRenderSet(userName, productInfo.ProductName, productInfo.EnvName, productInfo, []*commonservice.RenderChartArg{chartArg}, log.SugaredLogger())
	if errDiffRender != nil {
		err = fmt.Errorf("failed to build renderset for service: %s, err: %s", renderChart.ServiceName, errDiffRender)
		return
	}

	// update renderChart
	for _, _renderChart := range newRenderSet.ChartInfos {
		if _renderChart.ServiceName == serviceName {
			renderChart = _renderChart
		}
	}

	helmClient, errCreateClient := helmtool.NewClientFromNamespace(productInfo.ClusterID, productInfo.Namespace)
	if errCreateClient != nil {
		err = fmt.Errorf("failed to create helm client, err: %s", errCreateClient)
		return
	}

	param, errBuildParam := buildInstallParam(productInfo.Namespace, productInfo.EnvName, renderInfo.DefaultValues, renderChart, templateSvc)
	if err != nil {
		return fmt.Errorf("failed to generate install param, service: %s err: %s", templateSvc.ServiceName, errBuildParam)
	}

	err = InstallService(helmClient, param)
	if err != nil {
		return fmt.Errorf("fauled to install release for service: %s, err: %s", templateSvc.ServiceName, err)
	}

	err = updateServiceRevisionInProduct(productInfo, templateSvc.ServiceName, templateSvc.Revision)
	return
}

func updateK8sServiceInEnv(productInfo *commonmodels.Product, templateSvc *commonmodels.Service) error {
	productSvcMap, serviceName := productInfo.GetServiceMap(), templateSvc.ServiceName
	// service not applied in this environment
	if _, ok := productSvcMap[serviceName]; !ok {
		return nil
	}
	return UpdateProductV2(productInfo.EnvName, productInfo.ProductName, "", "", []string{templateSvc.ServiceName}, false, nil, log.SugaredLogger())
}

// ReInstallHelmSvcInAllEnvs reinstall svc in all envs in which the svc is already installed
func ReInstallHelmSvcInAllEnvs(productName string, templateSvc *commonmodels.Service) error {
	products, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
		Name: productName,
	})
	if err != nil {
		return fmt.Errorf("failed to list envs for product: %s, err: %s", productName, err)
	}
	retErr := &multierror.Error{}
	for _, product := range products {
		err := reInstallHelmServiceInEnv(product, templateSvc)
		if err != nil {
			retErr = multierror.Append(retErr, fmt.Errorf("failed to update service: %s/%s, err: %s", product.EnvName, templateSvc.ServiceName, err))
		}
	}
	return retErr.ErrorOrNil()
}

// updateHelmSvcInAllEnvs updates helm svc in all envs in which the svc is already installed
func updateHelmSvcInAllEnvs(userName, productName string, templateSvcs []*commonmodels.Service) error {
	products, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
		Name: productName,
	})
	if err != nil {
		return fmt.Errorf("failed to list envs for product: %s, err: %s", productName, err)
	}
	retErr := &multierror.Error{}
	for _, product := range products {
		for _, templateSvc := range templateSvcs {
			err := updateHelmServiceInEnv(userName, product, templateSvc)
			if err != nil {
				retErr = multierror.Append(retErr, fmt.Errorf("failed to update service: %s/%s, err: %s", product.EnvName, templateSvc.ServiceName, err))
			}
		}
	}
	return retErr.ErrorOrNil()
}

// updateK8sSvcInAllEnvs updates k8s svc in all envs in which the svc is already installed
func updateK8sSvcInAllEnvs(productName string, templateSvc *commonmodels.Service) error {
	products, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
		Name: productName,
	})
	if err != nil {
		return fmt.Errorf("failed to list envs for product: %s, err: %s", productName, err)
	}
	retErr := &multierror.Error{}
	for _, product := range products {
		err := updateK8sServiceInEnv(product, templateSvc)
		if err != nil {
			retErr = multierror.Append(retErr, err)
		}
	}
	return retErr.ErrorOrNil()
}

func AutoDeployHelmServiceToEnvs(userName, requestID, projectName string, serviceTemplates []*commonmodels.Service, log *zap.SugaredLogger) error {
	templateProduct, err := templaterepo.NewProductColl().Find(projectName)
	if err != nil {
		return fmt.Errorf("failed to find template product when depolying services: %s, err: %s", projectName, err)
	}
	if templateProduct.AutoDeploy == nil || !templateProduct.AutoDeploy.Enable {
		return nil
	}
	go func() {
		err = updateHelmSvcInAllEnvs(userName, projectName, serviceTemplates)
		if err != nil {
			commonservice.SendErrorMessage(userName, "服务自动部署失败", requestID, err, log)
		}
	}()
	return nil
}

func AutoDeployYamlServiceToEnvs(userName, requestID string, serviceTemplate *commonmodels.Service, log *zap.SugaredLogger) error {
	templateProduct, err := templaterepo.NewProductColl().Find(serviceTemplate.ProductName)
	if err != nil {
		return fmt.Errorf("failed to find template product when depolying services: %s, err: %s", serviceTemplate.ServiceName, err)
	}
	if templateProduct.AutoDeploy == nil || !templateProduct.AutoDeploy.Enable {
		return nil
	}
	go func() {
		err = updateK8sSvcInAllEnvs(serviceTemplate.ProductName, serviceTemplate)
		if err != nil {
			commonservice.SendErrorMessage(userName, "服务自动部署失败", requestID, err, log)
		}
	}()
	return nil
}
