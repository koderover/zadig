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

// ReInstallServiceInEnv uninstall the service and reinstall to update releaseNaming rule
func ReInstallServiceInEnv(productInfo *commonmodels.Product, serviceName string, templateSvc *commonmodels.Service) error {
	productSvcMap := productInfo.GetServiceMap()
	productSvc, ok := productSvcMap[serviceName]
	// service not applied in this environment
	if !ok {
		return nil
	}

	renderInfo, err := commonrepo.NewRenderSetColl().Find(&commonrepo.RenderSetFindOption{
		Name:     productInfo.Render.Name,
		Revision: productInfo.Render.Revision})
	if err != nil {
		return fmt.Errorf("failed to find renderset, %s:%d", productInfo.Render.Name, productInfo.Render.Revision)
	}

	var renderChart *templatemodels.RenderChart
	for _, _renderChart := range renderInfo.ChartInfos {
		if _renderChart.ServiceName == serviceName {
			renderChart = _renderChart
		}
	}
	if renderChart == nil {
		return fmt.Errorf("failed to find renderchart: %s", renderChart.ServiceName)
	}

	helmClient, err := helmtool.NewClientFromNamespace(productInfo.ClusterID, productInfo.Namespace)
	if err != nil {
		return err
	}

	prodSvcTemp, err := commonrepo.NewServiceColl().Find(&commonrepo.ServiceFindOption{ServiceName: serviceName, Revision: productSvc.Revision, ProductName: productInfo.ProductName})
	if err != nil {
		return fmt.Errorf("failed to get service: %s with revision: %d, err: %s", serviceName, productSvc.Revision, productInfo.ProductName)
	}

	// nothing would happen if release naming rule are same
	if prodSvcTemp.GetReleaseNaming() == templateSvc.GetReleaseNaming() {
		return nil
	}

	if err := UninstallService(helmClient, productInfo, prodSvcTemp, true); err != nil {
		return fmt.Errorf("helm uninstall service: %s in namespace: %s, err: %s", serviceName, productInfo.Namespace, err)
	}

	param, err := buildInstallParam(productInfo.Namespace, productInfo.EnvName, renderInfo.DefaultValues, renderChart, templateSvc)
	if err != nil {
		return fmt.Errorf("failed to generate install param, service: %s, namespace: %s, err: %s", templateSvc.ServiceName, productInfo.Namespace, err)
	}

	err = InstallService(helmClient, param)
	if err != nil {
		return fmt.Errorf("fauled to install release for service: %s, err: %s", templateSvc.ServiceName, err)
	}

	foundSvc := false
	// update service revision data in product
	for groupIndex, svcGroup := range productInfo.Services {
		for _, svc := range svcGroup {
			if svc.ServiceName == serviceName {
				svc.Revision = templateSvc.Revision
				foundSvc = true
				break
			}
		}
		if foundSvc {
			if err = commonrepo.NewProductColl().UpdateGroup(productInfo.EnvName, productInfo.ProductName, groupIndex, svcGroup); err != nil {
				return fmt.Errorf("failed to update service group: %s:%s, err: %s ", productInfo.ProductName, productInfo.EnvName, err)
			}
			break
		}
	}

	return nil
}

// UpdateSvcInAllEnvs updates svc in all envs in which the svc is already installed
func UpdateSvcInAllEnvs(productName, serviceName string, templateSvc *commonmodels.Service) error {
	products, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
		Name: productName,
	})
	if err != nil {
		return fmt.Errorf("failed to list envs for product: %s, err: %s", productName, err)
	}
	retErr := &multierror.Error{}
	for _, product := range products {
		err := ReInstallServiceInEnv(product, serviceName, templateSvc)
		if err != nil {
			retErr = multierror.Append(retErr, err)
		}
	}
	return retErr.ErrorOrNil()
}
