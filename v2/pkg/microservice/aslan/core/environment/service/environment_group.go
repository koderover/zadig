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
	"fmt"
	"sort"
	"strings"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/repository"
	"github.com/koderover/zadig/v2/pkg/setting"
	kubeclient "github.com/koderover/zadig/v2/pkg/shared/kube/client"
	"github.com/koderover/zadig/v2/pkg/shared/kube/wrapper"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/kube/getter"
	"github.com/koderover/zadig/v2/pkg/tool/kube/informer"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/util"
)

type EnvGroupRequest struct {
	ProjectName string `form:"projectName"`
	Page        int    `form:"page"`
	PerPage     int    `form:"perPage"`
	ServiceName string `form:"serviceName"`
}

// CalculateNonK8sProductStatus calculate product status for non k8s product: Helm + Host
func CalculateNonK8sProductStatus(productInfo *commonmodels.Product, log *zap.SugaredLogger) (string, error) {
	productName, envName, retStatus := productInfo.ProductName, productInfo.EnvName, setting.PodRunning
	_, workloads, err := commonservice.ListWorkloadsInEnv(envName, productName, "", 0, 0, log)
	if err != nil {
		return retStatus, e.ErrListGroups.AddDesc(err.Error())
	}
	for _, workload := range workloads {
		if !workload.Ready {
			return setting.PodUnstable, nil
		}
	}
	return retStatus, nil
}

func CalculateK8sProductStatus(productInfo *commonmodels.Product, log *zap.SugaredLogger) (string, error) {
	envName, productName := productInfo.EnvName, productInfo.ProductName
	cls, err := kubeclient.GetKubeClientSet(config.HubServerAddress(), productInfo.ClusterID)
	if err != nil {
		log.Errorf("[%s][%s] error: %v", envName, productName, err)
		return setting.PodUnstable, e.ErrListGroups.AddDesc(err.Error())
	}
	inf, err := informer.NewInformer(productInfo.ClusterID, productInfo.Namespace, cls)
	if err != nil {
		log.Errorf("[%s][%s] error: %v", envName, productName, err)
		return setting.PodUnstable, e.ErrListGroups.AddDesc(err.Error())
	}

	k8sHandler := &K8sService{log: log}
	return k8sHandler.calculateProductStatus(productInfo, inf)
}

func ListGroups(serviceName, envName, productName string, perPage, page int, production bool, log *zap.SugaredLogger) ([]*commonservice.ServiceResp, int, error) {
	var (
		count           = 0
		allServices     = make([]*commonmodels.ProductService, 0)
		currentServices = make([]*commonmodels.ProductService, 0)
		resp            = make([]*commonservice.ServiceResp, 0)
	)

	projectType := getProjectType(productName)
	if projectType == setting.HelmDeployType {
		log.Infof("listing group for helm project is not supported: %s/%s", productName, envName)
		return resp, count, nil
	}

	opt := &commonrepo.ProductFindOptions{Name: productName, EnvName: envName, Production: util.GetBoolPointer(production)}
	productInfo, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		log.Errorf("[%s][%s] error: %v", envName, productName, err)
		return resp, count, e.ErrListGroups.AddDesc(err.Error())
	}

	for _, groupServices := range productInfo.Services {
		for _, service := range groupServices {
			if serviceName != "" && strings.Contains(service.ServiceName, serviceName) {
				allServices = append(allServices, service)
			} else if serviceName == "" {
				allServices = append(allServices, service)
			}
		}
	}
	count = len(allServices)
	//sort services by name
	sort.SliceStable(allServices, func(i, j int) bool { return allServices[i].ServiceName < allServices[j].ServiceName })

	// add updatable field
	latestSvcs, err := repository.GetMaxRevisionsServicesMap(productName, production)
	if err != nil {
		return resp, count, e.ErrListGroups.AddDesc(fmt.Sprintf("failed to find latest services for %s/%s", productName, envName))
	}
	for _, svc := range allServices {
		if latestSvc, ok := latestSvcs[svc.ServiceName]; ok {
			svc.Updatable = svc.Revision < latestSvc.Revision
		}
		svc.DeployStrategy = productInfo.ServiceDeployStrategy[svc.ServiceName]
	}

	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), productInfo.ClusterID)
	if err != nil {
		log.Errorf("[%s][%s] failed to get kubeclient error: %v", envName, productName, err)
		return resp, count, e.ErrListGroups.AddDesc(err.Error())
	}
	cls, err := kubeclient.GetKubeClientSet(config.HubServerAddress(), productInfo.ClusterID)
	if err != nil {
		log.Errorf("[%s][%s] error: %v", envName, productName, err)
		return resp, count, e.ErrListGroups.AddDesc(err.Error())
	}
	inf, err := informer.NewInformer(productInfo.ClusterID, productInfo.Namespace, cls)
	if err != nil {
		log.Errorf("[%s][%s] error: %v", envName, productName, err)
		return resp, count, e.ErrListGroups.AddDesc(err.Error())
	}

	currentPage := page - 1
	if page*perPage < count {
		currentServices = allServices[currentPage*perPage : currentPage*perPage+perPage]
	} else {
		if currentPage*perPage > count {
			return resp, count, nil
		}
		currentServices = allServices[currentPage*perPage:]
	}
	resp = envHandleFunc(getProjectType(productName), log).listGroupServices(currentServices, envName, inf, productInfo)

	respMap := make(map[string]*commonservice.ServiceResp)
	for _, serviceResp := range resp {
		respMap[serviceResp.ServiceName] = serviceResp
	}
	if projectType == setting.K8SDeployType {
		for _, releaseType := range types.ZadigReleaseTypeList {
			releaseService, err := listZadigXReleaseServices(releaseType, productInfo.Namespace, productName, envName, kubeClient)
			if err != nil {
				return resp, count, e.ErrListGroups.AddErr(errors.Wrapf(err, "list zadigx %s release services", releaseType))
			}
			for _, serviceResp := range releaseService {
				if svc, ok := respMap[serviceResp.ServiceName]; !ok {
					resp = append(resp, serviceResp)
				} else {
					// when zadigx release resource exists, should set ZadigXReleaseType field too
					svc.ZadigXReleaseType = serviceResp.ZadigXReleaseType
					svc.ZadigXReleaseTag = serviceResp.ZadigXReleaseTag
				}
			}
		}
	}
	return resp, count, nil
}

func listZadigXReleaseServices(releaseType, namespace, productName, envName string, client client.Client) ([]*commonservice.ServiceResp, error) {
	selector := labels.Set{types.ZadigReleaseTypeLabelKey: releaseType}.AsSelector()
	deployments, err := getter.ListDeployments(namespace, selector, client)
	if err != nil {
		return nil, err
	}
	services := make([]*commonservice.ServiceResp, 0)
	serviceSets := make(map[string]*commonservice.ServiceResp)
	for _, deployment := range deployments {
		serviceName := deployment.GetLabels()[types.ZadigReleaseServiceNameLabelKey]
		if serviceName == "" {
			log.Warnf("listZadigXReleaseServices: deployment %s/%s has no service name label", deployment.Namespace, deployment.Name)
			continue
		}
		if resp, ok := serviceSets[serviceName]; ok {
			resp.Images = append(resp.Images, wrapper.Deployment(deployment).ImageInfos()...)
			resp.Status = func() string {
				if resp.Status == setting.PodUnstable || deployment.Status.Replicas != deployment.Status.ReadyReplicas {
					return setting.PodUnstable
				}
				return setting.PodRunning
			}()
			continue
		}
		svcResp := &commonservice.ServiceResp{
			ServiceName: serviceName,
			Type:        setting.K8SDeployType,
			Status: func() string {
				if deployment.Status.Replicas == deployment.Status.ReadyReplicas {
					return setting.PodRunning
				}
				return setting.PodUnstable

			}(),
			Images:            wrapper.Deployment(deployment).ImageInfos(),
			ProductName:       productName,
			EnvName:           envName,
			DeployStrategy:    "deploy",
			ZadigXReleaseType: releaseType,
			ZadigXReleaseTag:  deployment.GetLabels()[types.ZadigReleaseVersionLabelKey],
		}
		serviceSets[serviceName] = svcResp
		services = append(services, svcResp)
	}
	return services, nil
}
