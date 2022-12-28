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
	"encoding/json"
	"fmt"
	"time"

	"github.com/hashicorp/go-multierror"
	helmclient "github.com/mittwald/go-helm-client"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	templatemodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	e "github.com/koderover/zadig/pkg/tool/errors"
	helmtool "github.com/koderover/zadig/pkg/tool/helmclient"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/types"
	"github.com/koderover/zadig/pkg/util"
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

	var renderChart *templatemodels.ServiceRender
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

func updateK8sServiceInEnv(productInfo *commonmodels.Product, templateSvc *commonmodels.Service) error {
	productSvcMap, serviceName := productInfo.GetServiceMap(), templateSvc.ServiceName
	// service not applied in this environment
	if _, ok := productSvcMap[serviceName]; !ok {
		return nil
	}

	svcRender := &templatemodels.ServiceRender{
		ServiceName: templateSvc.ServiceName,
	}
	currentRenderset, err := FindProductRenderSet(productInfo.ProductName, productInfo.Render.Name, productInfo.EnvName, log.SugaredLogger())
	if err != nil {
		return fmt.Errorf("failed to find renderset, err: %s", err)
	}
	for _, sv := range currentRenderset.ServiceVariables {
		if sv.ServiceName == templateSvc.ServiceName {
			svcRender = sv
		}
	}
	filter := func(service *commonmodels.ProductService) bool {
		if templateSvc.ServiceName == service.ServiceName {
			return true
		}
		return false
	}
	return updateK8sProduct(productInfo, "system", "", []string{svcRender.ServiceName}, filter, []*templatemodels.ServiceRender{svcRender}, nil, false, currentRenderset.DefaultValues, log.SugaredLogger())
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

	updateArgs := &UpdateMultiHelmProductArg{
		ProductName: productName,
		EnvNames:    nil,
		ChartValues: nil,
	}

	for _, product := range products {

		envAffected := false
		productSvcMap := product.GetServiceMap()
		renderInfo, errFindRender := commonrepo.NewRenderSetColl().Find(&commonrepo.RenderSetFindOption{
			ProductTmpl: product.ProductName,
			EnvName:     product.EnvName,
			Name:        product.Render.Name,
			Revision:    product.Render.Revision})
		if errFindRender != nil {
			err = fmt.Errorf("failed to find renderset, %s:%d, err: %s", product.Render.Name, product.Render.Revision, errFindRender)
			return err
		}

		for _, templateSvc := range templateSvcs {
			// only update services
			if _, ok := productSvcMap[templateSvc.ServiceName]; !ok {
				continue
			}

			var renderChart *templatemodels.ServiceRender
			for _, _renderChart := range renderInfo.ChartInfos {
				if _renderChart.ServiceName == templateSvc.ServiceName {
					renderChart = _renderChart
					break
				}
			}
			if renderChart == nil {
				//log.Errorf("failed to find render chart info, serviceName: %s, renderset: %s", templateSvc.ServiceName, product.Render.Name)
				continue
			}

			chartArg := &commonservice.HelmSvcRenderArg{
				EnvName:     product.EnvName,
				ServiceName: templateSvc.ServiceName,
			}
			chartArg.LoadFromRenderChartModel(renderChart)
			updateArgs.ChartValues = append(updateArgs.ChartValues, chartArg)
			envAffected = true
		}
		if envAffected {
			updateArgs.EnvNames = append(updateArgs.EnvNames, product.EnvName)
		}
	}

	_, err = UpdateMultipleHelmEnv("", userName, updateArgs, log.SugaredLogger())
	if err != nil {
		return err
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

func updateK8sProduct(exitedProd *commonmodels.Product, user, requestID string, updateRevisionSvc []string, filter svcUpgradeFilter, updatedSvcs []*templatemodels.ServiceRender, deployStrategy map[string]string,
	force bool, variableYaml string, log *zap.SugaredLogger) error {
	envName, productName := exitedProd.EnvName, exitedProd.ProductName
	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), exitedProd.ClusterID)
	if err != nil {
		return e.ErrUpdateEnv.AddErr(err)
	}

	if !force {
		modifiedServices := getModifiedServiceFromObjectMetaList(kube.GetDirtyResources(exitedProd.Namespace, kubeClient))
		var specifyModifiedServices []*serviceInfo
		for _, modifiedService := range modifiedServices {
			if util.InStringArray(modifiedService.Name, updateRevisionSvc) {
				specifyModifiedServices = append(specifyModifiedServices, modifiedService)
			}
		}
		if len(specifyModifiedServices) > 0 {
			data, err := json.Marshal(specifyModifiedServices)
			if err != nil {
				log.Errorf("Marshal failure: %v", err)
			}
			log.Errorf("the following services are modified since last update: %s", data)
			return fmt.Errorf("the following services are modified since last update: %s", data)
		}
	}

	err = ensureKubeEnv(exitedProd.Namespace, exitedProd.RegistryID, map[string]string{setting.ProductLabel: productName}, exitedProd.ShareEnv.Enable, kubeClient, log)

	if err != nil {
		log.Errorf("[%s][P:%s] service.UpdateProductV2 create kubeEnv error: %v", envName, productName, err)
		return err
	}

	renderSet, err := commonservice.CreateRenderSetByMerge(
		&commonmodels.RenderSet{
			Name:             exitedProd.Namespace,
			EnvName:          envName,
			ProductTmpl:      productName,
			ServiceVariables: updatedSvcs,
			DefaultValues:    variableYaml,
		},
		log,
	)
	if err != nil {
		log.Errorf("[%s][P:%s] create renderset error: %v", envName, productName, err)
		return e.ErrUpdateEnv.AddDesc(e.FindProductTmplErrMsg)
	}

	//renderSet, err := commonservice.ValidateRenderSet(exitedProd.ProductName, exitedProd.Render.Name, exitedProd.EnvName, nil, log)
	//if err != nil {
	//	log.Errorf("[%s][P:%s] validate product renderset error: %v", envName, exitedProd.ProductName, err)
	//	return e.ErrUpdateEnv.AddDesc(err.Error())
	//}

	log.Infof("[%s][P:%s] updateProductImpl, services: %v", envName, productName, updateRevisionSvc)

	updateProd, err := GetInitProduct(productName, types.GeneralEnv, false, "", log)
	if err != nil {
		log.Errorf("[%s][P:%s] GetProductTemplate error: %v", envName, productName, err)
		return e.ErrUpdateEnv.AddDesc(e.FindProductTmplErrMsg)
	}

	// build services
	productSvcs := exitedProd.GetServiceMap()
	svcGroupMap := make(map[string]int)
	for i, svg := range exitedProd.Services {
		for _, svc := range svg {
			svcGroupMap[svc.ServiceName] = i
		}
	}
	svcGroups := make([][]*commonmodels.ProductService, 0)
	for _, svcGroup := range updateProd.Services {
		validSvcGroup := make([]*commonmodels.ProductService, 0)
		for _, svc := range svcGroup {
			if curSvc, ok := productSvcs[svc.ServiceName]; ok {
				// services to be updated
				validSvcGroup = append(validSvcGroup, curSvc)
				delete(svcGroupMap, curSvc.ServiceName)
			} else if util.InStringArray(svc.ServiceName, updateRevisionSvc) {
				// services to be added
				validSvcGroup = append(validSvcGroup, svc)
			}
		}
		svcGroups = append(svcGroups, validSvcGroup)
	}

	for svcName, groupIndex := range svcGroupMap {
		if groupIndex >= len(svcGroups) {
			groupIndex = 0
		}
		svcGroups[groupIndex] = append(svcGroups[groupIndex], productSvcs[svcName])
	}

	updateProd.Services = svcGroups

	switch exitedProd.Status {
	case setting.ProductStatusCreating, setting.ProductStatusUpdating, setting.ProductStatusDeleting:
		log.Errorf("[%s][P:%s] Product is not in valid status", envName, productName)
		return e.ErrUpdateEnv.AddDesc(e.EnvCantUpdatedMsg)
	default:
		// do nothing
	}

	// 设置产品状态为更新中
	if err := commonrepo.NewProductColl().UpdateStatus(envName, productName, setting.ProductStatusUpdating); err != nil {
		log.Errorf("[%s][P:%s] Product.UpdateStatus error: %v", envName, productName, err)
		return e.ErrUpdateEnv.AddDesc(e.UpdateEnvStatusErrMsg)
	}

	go func() {
		productErrMsg := ""
		err = updateProductImpl(updateRevisionSvc, deployStrategy, exitedProd, updateProd, renderSet, filter, log)
		if err != nil {
			productErrMsg = err.Error()
			log.Errorf("[%s][P:%s] failed to update product %#v", envName, productName, err)
			// 发送更新产品失败消息给用户
			title := fmt.Sprintf("更新 [%s] 的 [%s] 环境失败", productName, envName)
			commonservice.SendErrorMessage(user, title, requestID, err, log)
			updateProd.Status = setting.ProductStatusFailed
			productErrMsg = err.Error()
		} else {
			updateProd.Status = setting.ProductStatusSuccess
		}
		if err = commonrepo.NewProductColl().UpdateStatusAndError(envName, productName, updateProd.Status, productErrMsg); err != nil {
			log.Errorf("[%s][%s] Product.Update set product status error: %v", envName, productName, err)
			return
		}
	}()

	return nil
}

func updateCVMProduct(exitedProd *commonmodels.Product, user, requestID string, log *zap.SugaredLogger) error {
	envName, productName := exitedProd.EnvName, exitedProd.ProductName
	var serviceNames []string
	//TODO:The host update environment cannot remove deleted services
	services, err := commonrepo.NewServiceColl().ListMaxRevisionsAllSvcByProduct(productName)
	if err != nil && !commonrepo.IsErrNoDocuments(err) {
		log.Errorf("ListMaxRevisionsAllSvcByProduct: %s", err)
		return fmt.Errorf("ListMaxRevisionsAllSvcByProduct: %s", err)
	}
	if services != nil {
		svcNames := make([]string, len(services))
		for _, svc := range services {
			svcNames = append(svcNames, svc.ServiceName)
		}
		serviceNames = svcNames
	}

	if exitedProd.Render == nil {
		exitedProd.Render = &commonmodels.RenderInfo{ProductTmpl: exitedProd.ProductName}
	}

	// 检查renderset是否覆盖产品所有key
	renderSet, err := commonservice.ValidateRenderSet(exitedProd.ProductName, exitedProd.Render.Name, exitedProd.EnvName, nil, log)
	if err != nil {
		log.Errorf("[%s][P:%s] validate product renderset error: %v", envName, exitedProd.ProductName, err)
		return e.ErrUpdateEnv.AddDesc(err.Error())
	}

	log.Infof("[%s][P:%s] updateProductImpl, services: %v", envName, productName, serviceNames)

	// 查找产品模板
	updateProd, err := GetInitProduct(productName, types.GeneralEnv, false, "", log)
	if err != nil {
		log.Errorf("[%s][P:%s] GetProductTemplate error: %v", envName, productName, err)
		return e.ErrUpdateEnv.AddDesc(e.FindProductTmplErrMsg)
	}

	switch exitedProd.Status {
	case setting.ProductStatusCreating, setting.ProductStatusUpdating, setting.ProductStatusDeleting:
		log.Errorf("[%s][P:%s] Product is not in valid status", envName, productName)
		return e.ErrUpdateEnv.AddDesc(e.EnvCantUpdatedMsg)
	default:
		// do nothing
	}

	if err := commonrepo.NewProductColl().UpdateStatus(envName, productName, setting.ProductStatusUpdating); err != nil {
		log.Errorf("[%s][P:%s] Product.UpdateStatus error: %v", envName, productName, err)
		return e.ErrUpdateEnv.AddDesc(e.UpdateEnvStatusErrMsg)
	}

	go func() {
		productErrMsg := ""
		err = updateProductImpl(serviceNames, nil, exitedProd, updateProd, renderSet, nil, log)
		if err != nil {
			productErrMsg = err.Error()
			log.Errorf("[%s][P:%s] failed to update product %#v", envName, productName, err)
			// 发送更新产品失败消息给用户›
			title := fmt.Sprintf("更新 [%s] 的 [%s] 环境失败", productName, envName)
			commonservice.SendErrorMessage(user, title, requestID, err, log)
			updateProd.Status = setting.ProductStatusFailed
			productErrMsg = err.Error()
		} else {
			updateProd.Status = setting.ProductStatusSuccess
		}
		if err = commonrepo.NewProductColl().UpdateStatusAndError(envName, productName, updateProd.Status, productErrMsg); err != nil {
			log.Errorf("[%s][%s] Product.Update set product status error: %v", envName, productName, err)
			return
		}
	}()

	return nil
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
