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
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	commontypes "github.com/koderover/zadig/pkg/microservice/aslan/core/common/types"
	commonutil "github.com/koderover/zadig/pkg/microservice/aslan/core/common/util"
	envService "github.com/koderover/zadig/pkg/microservice/aslan/core/environment/service"
	svcService "github.com/koderover/zadig/pkg/microservice/aslan/core/service/service"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/util"
)

func CreateProjectOpenAPI(userID, username string, args *OpenAPICreateProductReq, logger *zap.SugaredLogger) error {
	// generate required information to create the project
	// 1. find all current clusters
	clusterList := make([]string, 0)
	clusters, err := commonrepo.NewK8SClusterColl().List(&commonrepo.ClusterListOpts{})
	if err != nil {
		logger.Errorf("failed to find resource list to fill in to the creating project, returning")
		return err
	}

	for _, cluster := range clusters {
		clusterList = append(clusterList, cluster.ID.Hex())
	}

	feature := new(template.ProductFeature)

	switch args.ProjectType {
	case config.ProjectTypeHelm:
		feature.BasicFacility = "kubernetes"
		feature.CreateEnvType = "system"
		feature.DeployType = "helm"
	case config.ProjectTypeYaml:
		feature.BasicFacility = "kubernetes"
		feature.CreateEnvType = "system"
		feature.DeployType = "k8s"
	case config.ProjectTypeLoaded:
		feature.BasicFacility = "kubernetes"
		feature.CreateEnvType = "external"
		feature.DeployType = "k8s"
	case config.ProjectTypeVM:
		feature.BasicFacility = "cloud_host"
		feature.CreateEnvType = "system"
	default:
		return errors.New("unsupported project type")
	}

	createArgs := &template.Product{
		ProjectName:    args.ProjectName,
		ProductName:    args.ProjectKey,
		CreateTime:     time.Now().Unix(),
		UpdateBy:       username,
		Enabled:        true,
		Description:    args.Description,
		ClusterIDs:     clusterList,
		ProductFeature: feature,
		Public:         args.IsPublic,
		Admins:         []string{userID},
	}

	return CreateProductTemplate(createArgs, logger)

}

func InitializeYAMLProject(userID, username, requestID string, args *OpenAPIInitializeProjectReq, logger *zap.SugaredLogger) error {
	// generate required information to create the project
	// 1. find all current clusters
	clusterList := make([]string, 0)
	clusters, err := commonrepo.NewK8SClusterColl().List(&commonrepo.ClusterListOpts{})
	if err != nil {
		logger.Errorf("failed to find resource list to fill in to the creating project, returning")
		return err
	}

	for _, cluster := range clusters {
		clusterList = append(clusterList, cluster.ID.Hex())
	}

	feature := new(template.ProductFeature)

	//creating YAML type project
	feature.BasicFacility = "kubernetes"
	feature.CreateEnvType = "system"
	feature.DeployType = "k8s"

	createArgs := &template.Product{
		ProjectName:    args.ProjectName,
		ProductName:    args.ProjectKey,
		CreateTime:     time.Now().Unix(),
		UpdateBy:       username,
		Enabled:        true,
		Description:    args.Description,
		ClusterIDs:     clusterList,
		ProductFeature: feature,
		Public:         args.IsPublic,
		Admins:         []string{userID},
	}

	err = CreateProductTemplate(createArgs, logger)
	if err != nil {
		logger.Errorf("failed to create project for initialization, error: %s", err)
		return err
	}

	// ============================= SECOND STEP: service creation ===============================
	for _, service := range args.ServiceList {
		// create service
		if service.Source == config.SourceFromYaml {
			creationArgs := &commonmodels.Service{
				ProductName: args.ProjectKey,
				ServiceName: service.ServiceName,
				Yaml:        service.Yaml,
				Source:      setting.SourceFromZadig,
				Type:        setting.K8SDeployType,
				CreateBy:    username,
			}

			_, err := svcService.CreateServiceTemplate(username, creationArgs, false, logger)
			if err != nil {
				logger.Errorf("failed to create service: %s for project initialization, error: %s", service.ServiceName, err)
				return err
			}
		} else if service.Source == config.SourceFromTemplate {
			template, err := commonrepo.NewYamlTemplateColl().GetByName(service.TemplateName)
			if err != nil {
				logger.Errorf("Failed to find template of name: %s, err: %w", service.TemplateName, err)
				return err
			}

			mergedYaml, mergedKVs, err := commonutil.MergeServiceVariableKVsAndKVInput(template.ServiceVariableKVs, service.VariableYaml)
			if err != nil {
				return fmt.Errorf("failed to merge variable yaml, err: %w", err)
			}
			loadArgs := &svcService.LoadServiceFromYamlTemplateReq{
				ProjectName:        args.ProjectKey,
				ServiceName:        service.ServiceName,
				TemplateID:         template.ID.Hex(),
				AutoSync:           service.AutoSync,
				VariableYaml:       mergedYaml,
				ServiceVariableKVs: mergedKVs,
			}

			err = svcService.LoadServiceFromYamlTemplate(username, loadArgs, false, logger)
			if err != nil {
				logger.Errorf("failed to create service: %s for project initialization, error: %s", service.ServiceName, err)
				return err
			}
		}
	}

	// ============================= THIRD STEP: environment creation ===============================
	serviceMap := make(map[string]*commonmodels.Service)

	allService, err := commonrepo.NewServiceColl().ListMaxRevisionsByProduct(args.ProjectKey)
	if err != nil {
		logger.Errorf("failed to find service list for initialization, error: %s", err)
		return err
	}

	for _, service := range allService {
		serviceMap[service.ServiceName] = service
	}

	serviceList := make([][]*envService.ProductK8sServiceCreationInfo, 0)

	// first we need to find all the services created before
	projectInfo, err := templaterepo.NewProductColl().Find(args.ProjectKey)
	if err != nil {
		logger.Errorf("failed to find project info for initialization, error: %s", err)
		return err
	}

	for _, serviceNameList := range projectInfo.Services {
		serviceGroup := make([]*envService.ProductK8sServiceCreationInfo, 0)

		for _, serviceName := range serviceNameList {
			singleService := &envService.ProductK8sServiceCreationInfo{
				ProductService: &commonmodels.ProductService{
					ServiceName: serviceMap[serviceName].ServiceName,
					ProductName: serviceMap[serviceName].ProductName,
					Type:        serviceMap[serviceName].Type,
					Revision:    serviceMap[serviceName].Revision,
				},
				DeployStrategy: "deploy",
			}

			singleService.Containers = make([]*commonmodels.Container, 0)
			for _, c := range serviceMap[serviceName].Containers {
				container := &commonmodels.Container{
					Name:      c.Name,
					Image:     c.Image,
					ImagePath: c.ImagePath,
					ImageName: util.GetImageNameFromContainerInfo(c.ImageName, c.Name),
				}
				singleService.Containers = append(singleService.Containers, container)
				singleService.VariableYaml = serviceMap[serviceName].VariableYaml
				singleService.VariableKVs = commontypes.ServiceToRenderVariableKVs(serviceMap[serviceName].ServiceVariableKVs)
			}
			serviceGroup = append(serviceGroup, singleService)
		}
		serviceList = append(serviceList, serviceGroup)
	}

	creationArgs := make([]*envService.CreateSingleProductArg, 0)
	for _, envDef := range args.EnvList {
		clusterInfo, err := commonrepo.NewK8SClusterColl().FindByName(envDef.ClusterName)
		if err != nil {
			logger.Errorf("failed to find a cluster with name: %s, error: %s", envDef.ClusterName, err)
			return errors.New("failed to find a cluster with name: " + envDef.ClusterName + " to create env: " + envDef.EnvName)
		}
		// create env creation args
		singleCreateArgs := &envService.CreateSingleProductArg{
			ProductName: args.ProjectKey,
			Namespace:   envDef.Namespace,
			ClusterID:   clusterInfo.ID.Hex(),
			EnvName:     envDef.EnvName,
			Production:  false,
			Services:    serviceList,
		}

		creationArgs = append(creationArgs, singleCreateArgs)
	}

	return envService.CreateYamlProduct(args.ProjectKey, username, requestID, creationArgs, logger)
}

func OpenAPIInitializeHelmProject(userID, username, requestID string, args *OpenAPIInitializeProjectReq, logger *zap.SugaredLogger) error {
	// generate required information to create the project
	// 1. find all current clusters
	clusterList := make([]string, 0)
	clusters, err := commonrepo.NewK8SClusterColl().List(&commonrepo.ClusterListOpts{})
	if err != nil {
		logger.Errorf("failed to find resource list to fill in to the creating project")
		return err
	}

	for _, cluster := range clusters {
		clusterList = append(clusterList, cluster.ID.Hex())
	}

	feature := new(template.ProductFeature)

	//creating YAML type project
	feature.BasicFacility = "kubernetes"
	feature.CreateEnvType = "system"
	feature.DeployType = "helm"

	createArgs := &template.Product{
		ProjectName:    args.ProjectName,
		ProductName:    args.ProjectKey,
		CreateTime:     time.Now().Unix(),
		UpdateBy:       username,
		Enabled:        true,
		Description:    args.Description,
		ClusterIDs:     clusterList,
		ProductFeature: feature,
		Public:         args.IsPublic,
		Admins:         []string{userID},
	}

	err = CreateProductTemplate(createArgs, logger)
	if err != nil {
		logger.Errorf("failed to create project for initialization, error: %s", err)
		return err
	}

	// ============================= SECOND STEP: service creation ===============================
	for _, service := range args.ServiceList {
		if service.Source == config.SourceFromTemplate {
			variables := make([]*svcService.Variable, 0)
			for _, kv := range service.VariableYaml {
				if v, ok := kv.Value.(string); ok {
					variables = append(variables, &svcService.Variable{
						Key:   kv.Key,
						Value: v,
					})
				}
			}
			template := &svcService.CreateFromChartTemplate{
				TemplateName: service.TemplateName,
				ValuesYAML:   service.ValuesYaml,
				Variables:    variables,
			}
			templateArgs := &svcService.HelmServiceCreationArgs{
				Name:       service.ServiceName,
				CreatedBy:  username,
				AutoSync:   service.AutoSync,
				Production: false,
			}
			templateArgs.Source = "chartTemplate"
			templateArgs.CreateFrom = template

			_, err := svcService.CreateOrUpdateHelmServiceFromChartTemplate(args.ProjectKey, templateArgs, false, logger)
			if err != nil {
				logger.Errorf("failed to create service from chart template, error: %s", err)
				return err
			}

		}
	}

	// ============================= THIRD STEP: environment creation ===============================
	allService, err := commonrepo.NewServiceColl().ListMaxRevisionsByProduct(args.ProjectKey)
	if err != nil {
		logger.Errorf("failed to find service list for initialization, error: %s", err)
		return err
	}

	creationArgs := make([]*envService.CreateSingleProductArg, 0)
	for _, envDef := range args.EnvList {
		clusterInfo, err := commonrepo.NewK8SClusterColl().FindByName(envDef.ClusterName)
		if err != nil {
			logger.Errorf("failed to find a cluster with name: %s, error: %s", envDef.ClusterName, err)
			return errors.New("failed to find a cluster with name: " + envDef.ClusterName + " to create env: " + envDef.EnvName)
		}

		creationInfos := make([]*envService.ProductHelmServiceCreationInfo, 0)
		for _, service := range allService {
			creationInfo := &envService.ProductHelmServiceCreationInfo{
				HelmSvcRenderArg: &commonservice.HelmSvcRenderArg{},
				DeployStrategy:   "deploy",
			}
			if service.HelmChart != nil {
				creationInfo.ChartVersion = service.HelmChart.Version
			}
			creationInfo.EnvName = envDef.EnvName
			creationInfo.ServiceName = service.ServiceName
			creationInfos = append(creationInfos, creationInfo)
		}

		// create env creation args
		singleCreateArgs := &envService.CreateSingleProductArg{
			ProductName: args.ProjectKey,
			Namespace:   envDef.Namespace,
			ClusterID:   clusterInfo.ID.Hex(),
			EnvName:     envDef.EnvName,
			Production:  false,
			ChartValues: creationInfos,
		}

		creationArgs = append(creationArgs, singleCreateArgs)
	}

	return envService.CreateHelmProduct(args.ProjectKey, username, requestID, creationArgs, logger)
}

func ListProjectOpenAPI(pageSize, pageNum int64, logger *zap.SugaredLogger) (*OpenAPIProjectListResp, error) {
	resp, err := ListProjects(
		&ProjectListOptions{
			PageSize:  pageSize,
			PageNum:   pageNum,
			Verbosity: VerbosityDetailed,
		},
		logger,
	)
	if err != nil {
		logger.Errorf("OpenAPI: failed to list projects, error: %s", err)
		return nil, err
	}

	result, ok := resp.(*ProjectDetailedResponse)
	if !ok {
		logger.Errorf("OpenAPI: failed to list projects, error: %v", err)
		return nil, err
	}

	list := &OpenAPIProjectListResp{
		Total: result.Total,
	}
	for _, p := range result.ProjectDetailedRepresentation {
		list.Projects = append(list.Projects, &ProjectBrief{
			ProjectName: p.Alias,
			ProjectKey:  p.Name,
			Description: p.Desc,
			DeployType:  p.DeployType,
		})
	}
	return list, nil
}

func GetProjectDetailOpenAPI(projectName string, logger *zap.SugaredLogger) (*OpenAPIProjectDetailResp, error) {
	project, err := templaterepo.NewProductColl().Find(projectName)
	if err != nil {
		logger.Errorf("OpenAPI: failed to find project %s, error: %s", projectName, err)
		return nil, err
	}

	return &OpenAPIProjectDetailResp{
		ProjectName: project.ProjectName,
		ProjectKey:  project.ProductName,
		IsPublic:    project.Public,
		Desc:        project.Description,
		DeployType:  project.ProductFeature.DeployType,
		CreateTime:  project.CreateTime,
		CreatedBy:   project.UpdateBy,
	}, nil
}

func DeleteProjectOpenAPI(userName, requestID, projectName string, isDelete bool, logger *zap.SugaredLogger) error {
	return DeleteProductTemplate(userName, projectName, requestID, isDelete, logger)
}

func OpenAPIGetGlobalVariables(projectName string, logger *zap.SugaredLogger) (*commontypes.GlobalVariables, error) {
	resp := &commontypes.GlobalVariables{}
	var err error
	resp.Variables, err = GetGlobalVariables(projectName, false, logger)
	if err != nil {
		logger.Errorf("failed to get global variables for project:%s", projectName)
		return nil, err
	}
	resp.ProductionVariables, err = GetGlobalVariables(projectName, true, logger)
	if err != nil {
		logger.Errorf("failed to get global variables for project:%s", projectName)
		return nil, err
	}
	return resp, nil
}
