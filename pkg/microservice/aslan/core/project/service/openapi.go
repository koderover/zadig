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

	"github.com/koderover/zadig/pkg/util"
	"go.uber.org/zap"

	configbase "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	envService "github.com/koderover/zadig/pkg/microservice/aslan/core/environment/service"
	svcService "github.com/koderover/zadig/pkg/microservice/aslan/core/service/service"
	policyservice "github.com/koderover/zadig/pkg/microservice/policy/core/service"
	"github.com/koderover/zadig/pkg/setting"
)

func CreateProjectOpenAPI(userID, username string, args *OpenAPICreateProductReq, logger *zap.SugaredLogger) error {
	var rbs []*policyservice.RoleBinding
	rbs = append(rbs, &policyservice.RoleBinding{
		Name:   configbase.RoleBindingNameFromUIDAndRole(userID, setting.ProjectAdmin, ""),
		UID:    userID,
		Role:   string(setting.ProjectAdmin),
		Preset: true,
	})

	if args.IsPublic {
		rbs = append(rbs, &policyservice.RoleBinding{
			Name:   configbase.RoleBindingNameFromUIDAndRole("*", setting.ReadOnly, ""),
			UID:    "*",
			Role:   string(setting.ReadOnly),
			Preset: true,
		})
	}

	for _, rb := range rbs {
		err := policyservice.UpdateOrCreateRoleBinding(args.ProjectName, rb, logger)
		if err != nil {
			logger.Errorf("failed to create rolebinding %s, err: %s", rb.Name, err)
			return err
		}
	}

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
	}

	return CreateProductTemplate(createArgs, logger)

}

func InitializeYAMLProject(userID, username, requestID string, args *OpenAPIInitializeProjectReq, logger *zap.SugaredLogger) error {

	// =========================== FIRST STEP: project creation ===============================
	var rbs []*policyservice.RoleBinding
	rbs = append(rbs, &policyservice.RoleBinding{
		Name:   configbase.RoleBindingNameFromUIDAndRole(userID, setting.ProjectAdmin, ""),
		UID:    userID,
		Role:   string(setting.ProjectAdmin),
		Preset: true,
	})

	if args.IsPublic {
		rbs = append(rbs, &policyservice.RoleBinding{
			Name:   configbase.RoleBindingNameFromUIDAndRole("*", setting.ReadOnly, ""),
			UID:    "*",
			Role:   string(setting.ReadOnly),
			Preset: true,
		})
	}

	for _, rb := range rbs {
		err := policyservice.UpdateOrCreateRoleBinding(args.ProjectName, rb, logger)
		if err != nil {
			logger.Errorf("failed to create rolebinding %s, err: %s", rb.Name, err)
			return err
		}
	}

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
	}

	err = CreateProductTemplate(createArgs, logger)
	if err != nil {
		logger.Errorf("failed to create project for initialization, error: %s", err)
		return err
	}

	// ============================= SECOND STEP: service creation ===============================
	for _, service := range args.ServiceList {
		// create service
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
	}

	// ============================= THIRD STEP: environment creation ===============================
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
			opt := &commonrepo.ServiceFindOption{
				ServiceName:   serviceName,
				ProductName:   args.ProjectKey,
				ExcludeStatus: setting.ProductStatusDeleting,
			}

			serviceTmpl, err := commonrepo.NewServiceColl().Find(opt)
			if err != nil {
				errMsg := fmt.Sprintf("Can not find service with option %+v, error: %v", opt, err)
				logger.Error(errMsg)
				return errors.New(errMsg)
			}

			singleService := &envService.ProductK8sServiceCreationInfo{
				ProductService: &commonmodels.ProductService{
					ServiceName: serviceTmpl.ServiceName,
					ProductName: serviceTmpl.ProductName,
					Type:        serviceTmpl.Type,
					Revision:    serviceTmpl.Revision,
				},
				DeployStrategy: "deploy",
			}

			singleService.Containers = make([]*commonmodels.Container, 0)
			for _, c := range serviceTmpl.Containers {
				container := &commonmodels.Container{
					Name:      c.Name,
					Image:     c.Image,
					ImagePath: c.ImagePath,
					ImageName: util.GetImageNameFromContainerInfo(c.ImageName, c.Name),
				}
				singleService.Containers = append(singleService.Containers, container)
				singleService.VariableYaml = serviceTmpl.VariableYaml
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
