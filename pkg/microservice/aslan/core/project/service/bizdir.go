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

package service

import (
	"fmt"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/pm"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/repository"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/setting"
	kubeclient "github.com/koderover/zadig/v2/pkg/shared/kube/client"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	helmtool "github.com/koderover/zadig/v2/pkg/tool/helmclient"
	"github.com/koderover/zadig/v2/pkg/tool/kube/informer"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"helm.sh/helm/v3/pkg/action"
	"k8s.io/apimachinery/pkg/util/sets"
)

type GroupDetail struct {
	GroupName string                        `json:"group_name"`
	Projects  []*commonmodels.ProjectDetail `json:"projects"`
}

func GetBizDirProject() ([]GroupDetail, error) {
	groups, err := commonrepo.NewProjectGroupColl().List()
	if err != nil {
		return nil, e.ErrGetBizDirProject.AddErr(fmt.Errorf("failed to list project groups, error: %v", err))
	}

	groupedProjectSet := sets.NewString()
	resp := []GroupDetail{}
	for _, group := range groups {
		groupDetail := GroupDetail{
			GroupName: group.Name,
		}
		for _, project := range group.Projects {
			groupDetail.Projects = append(groupDetail.Projects, project)
			groupedProjectSet.Insert(project.ProjectKey)
		}
		resp = append(resp, groupDetail)
	}

	projects, err := templaterepo.NewProductColl().List()
	if err != nil {
		return nil, e.ErrGetBizDirProject.AddErr(fmt.Errorf("failed to list projects, error: %v", err))
	}

	ungrouped := GroupDetail{
		GroupName: setting.UNGROUPED,
	}
	for _, project := range projects {
		if !groupedProjectSet.Has(project.ProductName) {
			projectDetail := *&commonmodels.ProjectDetail{
				ProjectName:       project.ProjectName,
				ProjectKey:        project.ProductName,
				ProjectDeployType: project.ProductFeature.GetDeployType(),
			}
			ungrouped.Projects = append(ungrouped.Projects, &projectDetail)
		}
	}
	resp = append(resp, ungrouped)

	return resp, nil
}

func GetBizDirProjectServices(projectName string) ([]string, error) {
	svcSet := sets.NewString()
	productTmpl, err := templaterepo.NewProductColl().Find(projectName)
	if err != nil {
		return nil, e.ErrGetBizDirProjectService.AddErr(fmt.Errorf("Can not find project %s, error: %s", projectName, err))
	}

	testServices, err := commonrepo.NewServiceColl().ListMaxRevisionsForServices(productTmpl.AllTestServiceInfos(), "")
	if err != nil {
		return nil, e.ErrGetBizDirProjectService.AddErr(fmt.Errorf("Failed to list testing services by %+v, err: %s", productTmpl.AllTestServiceInfos(), err))
	}
	for _, svc := range testServices {
		svcSet.Insert(svc.ServiceName)
	}

	prodServices, err := commonrepo.NewProductionServiceColl().ListMaxRevisionsByProject(projectName, productTmpl.ProductFeature.DeployType)
	if err != nil {
		return nil, e.ErrGetBizDirProjectService.AddErr(fmt.Errorf("Failed to production list services by %+v, err: %s", projectName, err))
	}
	for _, svc := range prodServices {
		svcSet.Insert(svc.ServiceName)
	}

	return svcSet.List(), nil
}

func SearchBizDirByProject(projectKeyword string) ([]GroupDetail, error) {
	groups, err := commonrepo.NewProjectGroupColl().List()
	if err != nil {
		return nil, e.ErrSearchBizDirByProject.AddErr(fmt.Errorf("failed to list project groups, error: %v", err))
	}

	projectGroupMap := make(map[string]string)
	for _, group := range groups {
		for _, project := range group.Projects {
			projectGroupMap[project.ProjectKey] = group.Name
		}
	}

	projects, count, err := templaterepo.NewProductColl().PageListProjectByFilter(
		templaterepo.ProductListByFilterOpt{
			Filter: projectKeyword,
			Skip:   0,
			Limit:  999,
		},
	)
	if err != nil {
		return nil, e.ErrSearchBizDirByProject.AddErr(fmt.Errorf("failed to list projects by filter %s, error: %v", projectKeyword, err))
	}
	if count >= 999 {
		log.Errorf("too many projects(>=999) found by filter %s", projectKeyword)
	}

	groupProjectMap := make(map[string][]*commonmodels.ProjectDetail)
	resp := []GroupDetail{}
	for _, project := range projects {
		projectDetail := &commonmodels.ProjectDetail{
			ProjectKey:        project.Name,
			ProjectName:       project.Alias,
			ProjectDeployType: project.ProductFeature.GetDeployType(),
		}
		if projectGroupMap[project.Name] != "" {
			groupProjectMap[projectGroupMap[project.Name]] = append(groupProjectMap[projectGroupMap[project.Name]], projectDetail)
		} else {
			groupProjectMap[setting.UNGROUPED] = append(groupProjectMap[setting.UNGROUPED], projectDetail)
		}
	}
	for group, project := range groupProjectMap {
		groupDetail := GroupDetail{
			GroupName: group,
			Projects:  project,
		}
		resp = append(resp, groupDetail)
	}

	return resp, nil
}

type SearchBizDirByServiceGroup struct {
	GroupName string                          `json:"group_name"`
	Projects  []*SearchBizDirByServiceProject `json:"projects"`
}

type SearchBizDirByServiceProject struct {
	Project  *commonmodels.ProjectDetail `json:"project"`
	Services []string                    `json:"services"`
}

func SearchBizDirByService(serviceName string) ([]*SearchBizDirByServiceGroup, error) {
	resp := []*SearchBizDirByServiceGroup{}
	groups, err := commonrepo.NewProjectGroupColl().List()
	if err != nil {
		return nil, e.ErrSearchBizDirByService.AddErr(fmt.Errorf("failed to list project groups, error: %v", err))
	}

	projects, err := templaterepo.NewProductColl().List()
	if err != nil {
		return nil, e.ErrSearchBizDirByService.AddErr(fmt.Errorf("failed to list template projects, error: %v", err))
	}

	templateProjectMap := make(map[string]*template.Product)
	for _, project := range projects {
		templateProjectMap[project.ProductName] = project
	}

	projectGroupMap := make(map[string]string)
	for _, group := range groups {
		for _, project := range group.Projects {
			projectGroupMap[project.ProjectKey] = group.Name
		}
	}

	groupMap := make(map[string]*SearchBizDirByServiceGroup)
	projectMap := make(map[string]*SearchBizDirByServiceProject)
	addToRespMap := func(service *commonmodels.Service) {
		if _, ok := templateProjectMap[service.ProductName]; !ok {
			log.Warnf("project %s not found for service %s", service.ProductName, service.ServiceName)
			return
		}

		groupName, ok := projectGroupMap[service.ProductName]
		if !ok {
			groupName = setting.UNGROUPED
		}
		elemGroup, ok := groupMap[groupName]
		if !ok {
			elemGroup = &SearchBizDirByServiceGroup{
				GroupName: groupName,
			}
			groupMap[groupName] = elemGroup
		}

		if elem, ok := projectMap[service.ProductName]; !ok {
			project := &SearchBizDirByServiceProject{
				Project: &commonmodels.ProjectDetail{
					ProjectKey:        templateProjectMap[service.ProductName].ProductName,
					ProjectName:       templateProjectMap[service.ProductName].ProjectName,
					ProjectDeployType: templateProjectMap[service.ProductName].ProductFeature.DeployType,
				},
				Services: []string{service.ServiceName},
			}
			elemGroup.Projects = append(elemGroup.Projects, project)
			projectMap[service.ProductName] = project
		} else {
			svcSet := sets.NewString(elem.Services...)
			svcSet.Insert(service.ServiceName)
			elem.Services = svcSet.List()
		}
	}

	testServices, err := commonrepo.NewServiceColl().SearchMaxRevisionsByService(serviceName)
	if err != nil {
		return nil, e.ErrSearchBizDirByService.AddErr(fmt.Errorf("Failed to search testing services by service name %v, err: %s", serviceName, err))
	}
	for _, svc := range testServices {
		addToRespMap(svc)
	}

	prodServices, err := commonrepo.NewProductionServiceColl().SearchMaxRevisionsByService(serviceName)
	if err != nil {
		return nil, e.ErrSearchBizDirByService.AddErr(fmt.Errorf("Failed to search production services by service name %v, err: %s", serviceName, err))
	}
	for _, svc := range prodServices {
		addToRespMap(svc)
	}

	for _, elem := range groupMap {
		resp = append(resp, elem)
	}

	return resp, nil
}

type GetBizDirServiceDetailResponse struct {
	ProjectName  string   `json:"project_name"`
	EnvName      string   `json:"env_name"`
	Production   bool     `json:"production"`
	Name         string   `json:"name"`
	Type         string   `json:"type"`
	Status       string   `json:"status"`
	Images       []string `json:"images"`
	ChartVersion string   `json:"chart_version"`
	UpdateTime   int64    `json:"update_time"`
}

func GetBizDirServiceDetail(projectName, serviceName string) ([]GetBizDirServiceDetailResponse, error) {
	resp := []GetBizDirServiceDetailResponse{}

	project, err := templaterepo.NewProductColl().Find(projectName)
	if err != nil {
		return nil, e.ErrGetBizDirServiceDetail.AddErr(fmt.Errorf("failed to find project %s, error: %v", projectName, err))
	}

	envs, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
		Name: projectName,
	})
	if err != nil {
		return nil, e.ErrGetBizDirServiceDetail.AddErr(fmt.Errorf("failed to list product %s, error: %v", projectName, err))
	}

	for _, env := range envs {
		prodSvc := env.GetServiceMap()[serviceName]
		if prodSvc == nil && !project.IsHostProduct() {
			// not deployed in this env
			continue
		}

		if project.IsK8sYamlProduct() {
			cls, err := kubeclient.GetKubeClientSet(config.HubServerAddress(), env.ClusterID)
			if err != nil {
				return resp, e.ErrGetBizDirServiceDetail.AddDesc(err.Error())
			}
			informer, err := informer.NewInformer(env.ClusterID, env.Namespace, cls)
			if err != nil {
				return resp, e.ErrGetBizDirServiceDetail.AddDesc(err.Error())
			}

			detail := GetBizDirServiceDetailResponse{
				ProjectName: env.ProductName,
				EnvName:     env.EnvName,
				Production:  env.Production,
				Name:        serviceName,
				Type:        setting.K8SDeployType,
				UpdateTime:  prodSvc.UpdateTime,
			}
			serviceTmpl, err := repository.QueryTemplateService(&commonrepo.ServiceFindOption{
				ServiceName: prodSvc.ServiceName,
				Revision:    prodSvc.Revision,
				ProductName: prodSvc.ProductName,
			}, env.Production)
			if err != nil {
				return nil, fmt.Errorf("failed to get service template for productName: %s, serviceName: %s, revision %d, error: %v",
					prodSvc.ProductName, prodSvc.ServiceName, prodSvc.Revision, err)
			}

			serviceStatus := commonservice.QueryPodsStatus(env, serviceTmpl, serviceTmpl.ServiceName, cls, informer, log.SugaredLogger())
			detail.Status = serviceStatus.PodStatus
			detail.Images = serviceStatus.Images

			resp = append(resp, detail)
		} else if project.IsHostProduct() {
			cls, err := kubeclient.GetKubeClientSet(config.HubServerAddress(), env.ClusterID)
			if err != nil {
				return resp, e.ErrGetBizDirServiceDetail.AddDesc(err.Error())
			}
			informer, err := informer.NewInformer(env.ClusterID, env.Namespace, cls)
			if err != nil {
				return resp, e.ErrGetBizDirServiceDetail.AddDesc(err.Error())
			}

			detail := GetBizDirServiceDetailResponse{
				ProjectName: env.ProductName,
				EnvName:     env.EnvName,
				Production:  env.Production,
				Name:        serviceName,
				Type:        setting.K8SDeployType,
			}
			serviceTmpl, err := repository.QueryTemplateService(&commonrepo.ServiceFindOption{
				ServiceName: serviceName,
				ProductName: projectName,
			}, env.Production)
			if err != nil {
				return nil, fmt.Errorf("failed to get service template for productName: %s, serviceName: %s, error: %v",
					prodSvc.ProductName, prodSvc.ServiceName, err)
			}
			detail.UpdateTime = serviceTmpl.DeployTime

			serviceStatus := commonservice.QueryPodsStatus(env, serviceTmpl, serviceTmpl.ServiceName, cls, informer, log.SugaredLogger())
			detail.Status = serviceStatus.PodStatus
			detail.Images = serviceStatus.Images

			resp = append(resp, detail)
		} else if project.IsHelmProduct() {
			restConfig, err := kube.GetRESTConfig(env.ClusterID)
			if err != nil {
				log.Errorf("GetRESTConfig error: %s", err)
				return nil, fmt.Errorf("failed to get k8s rest config, err: %s", err)
			}
			helmClient, err := helmtool.NewClientFromRestConf(restConfig, env.Namespace)
			if err != nil {
				log.Errorf("[%s][%s] NewClientFromRestConf error: %s", env.EnvName, projectName, err)
				return nil, fmt.Errorf("failed to init helm client, err: %s", err)
			}

			svcToReleaseNameMap, err := commonutil.GetServiceNameToReleaseNameMap(env)
			if err != nil {
				return nil, fmt.Errorf("failed to build release-service map: %s", err)
			}
			releaseName := svcToReleaseNameMap[serviceName]
			if releaseName == "" {
				return nil, fmt.Errorf("release name not found for service %s", serviceName)
			}

			detail := GetBizDirServiceDetailResponse{
				ProjectName: env.ProductName,
				EnvName:     env.EnvName,
				Production:  env.Production,
				Name:        fmt.Sprintf("%s(%s)", releaseName, serviceName),
				Type:        setting.HelmDeployType,
			}

			listClient := action.NewList(helmClient.ActionConfig)
			listClient.Filter = releaseName
			releases, err := listClient.Run()
			if err != nil {
				return nil, e.ErrGetBizDirServiceDetail.AddErr(fmt.Errorf("failed to list helm releases by %s, error: %v", serviceName, err))
			}
			if len(releases) == 0 {
				continue
			}
			if len(releases) > 1 {
				return nil, e.ErrGetBizDirServiceDetail.AddDesc("helm release number is not equal to 1")
			}

			detail.Status = string(releases[0].Info.Status)
			detail.ChartVersion = releases[0].Chart.Metadata.Version
			detail.UpdateTime = releases[0].Info.LastDeployed.Unix()

			resp = append(resp, detail)
		} else if project.IsCVMProduct() {
			detail := GetBizDirServiceDetailResponse{
				ProjectName: env.ProductName,
				EnvName:     env.EnvName,
				Production:  env.Production,
				Name:        serviceName,
				Type:        project.ProductFeature.DeployType,
				UpdateTime:  prodSvc.UpdateTime,
			}

			serviceTmpl, err := commonservice.GetServiceTemplate(
				prodSvc.ServiceName, setting.PMDeployType, prodSvc.ProductName, "", prodSvc.Revision, log.SugaredLogger(),
			)
			if err != nil {
				return nil, fmt.Errorf("failed to get service template for productName: %s, serviceName: %s, revision %d, error: %v",
					prodSvc.ProductName, prodSvc.ServiceName, prodSvc.Revision, err)
			}

			if len(serviceTmpl.EnvStatuses) > 0 {
				envStatuses := make([]*commonmodels.EnvStatus, 0)
				filterEnvStatuses, err := pm.GenerateEnvStatus(serviceTmpl.EnvConfigs, log.NopSugaredLogger())
				if err != nil {
					return nil, fmt.Errorf("failed to generate env status for productName: %s, serviceName: %s, revision %d, error: %v", prodSvc.ProductName, prodSvc.ServiceName, prodSvc.Revision, err)
				}
				filterEnvStatusSet := sets.NewString()
				for _, v := range filterEnvStatuses {
					filterEnvStatusSet.Insert(v.Address)
				}
				for _, envStatus := range serviceTmpl.EnvStatuses {
					if envStatus.EnvName == env.EnvName && filterEnvStatusSet.Has(envStatus.Address) {
						envStatuses = append(envStatuses, envStatus)
					}
				}

				if len(envStatuses) > 0 {
					total := 0
					running := 0
					for _, envStatus := range envStatuses {
						total++
						if envStatus.Status == setting.PodRunning {
							running++
						}
					}
					detail.Status = fmt.Sprintf("%d/%d", running, total)
				}
			}

			resp = append(resp, detail)
		}

	}

	return resp, nil
}
