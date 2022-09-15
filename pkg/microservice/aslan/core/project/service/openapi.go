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
	"time"

	"go.uber.org/zap"

	configbase "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
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
