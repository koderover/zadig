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

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	fsservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/fs"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/kube"
	commonutil "github.com/koderover/zadig/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/pkg/setting"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

type ListEnvServiceVersionsResponse struct {
	ServiceName string `json:"service_name"`
	Revision    int64  `json:"revision"`
	CreateTime  int64  `json:"create_time"`
	CreateBy    string `json:"create_by"`
}

func ListEnvServiceVersions(ctx *internalhandler.Context, projectName, envName, serviceName string, isProduction bool, log *zap.SugaredLogger) ([]ListEnvServiceVersionsResponse, error) {
	resp := []ListEnvServiceVersionsResponse{}
	revisions, err := mongodb.NewEnvServiceVersionColl().ListServiceVersions(projectName, envName, serviceName, isProduction)
	if err != nil {
		return nil, e.ErrListServiceTemplateVersions.AddErr(fmt.Errorf("failed to list service revisions, error: %v", err))
	}

	for _, revision := range revisions {
		resp = append(resp, ListEnvServiceVersionsResponse{
			ServiceName: revision.Service.ServiceName,
			Revision:    revision.Revision,
			CreateTime:  revision.CreateTime,
			CreateBy:    revision.CreateBy,
		})
	}
	return resp, nil
}

type DiffEnvServiceVersionsResponse struct {
	Type          string `json:"type"`
	YamlA         string `json:"yaml_a"`
	YamlB         string `json:"yaml_b"`
	VariableYamlA string `json:"variable_yaml_a"`
	VariableYamlB string `json:"variable_yaml_b"`
}

func DiffEnvServiceVersions(ctx *internalhandler.Context, projectName, envName, serviceName string, versionA, versionB int64, isProduction bool, log *zap.SugaredLogger) (DiffEnvServiceVersionsResponse, error) {
	resp := DiffEnvServiceVersionsResponse{}

	revisionA, err := mongodb.NewEnvServiceVersionColl().Find(projectName, envName, serviceName, isProduction, versionA)
	if err != nil {
		return resp, e.ErrDiffServiceTemplateVersions.AddErr(fmt.Errorf("failed to find %s/%s service for version A %d, isProduction %v, error: %v", projectName, serviceName, versionA, isProduction, err))
	}
	resp.Type = revisionA.Service.Type
	if revisionA.Service.Type == setting.K8SDeployType {
		fakeEnv := &commonmodels.Product{
			ProductName: revisionA.ProductName,
			EnvName:     revisionA.EnvName,
			Namespace:   revisionA.EnvName,
			Production:  revisionA.Production,
		}
		parsedYaml, err := kube.RenderEnvService(fakeEnv, revisionA.Service.GetServiceRender(), revisionA.Service)
		if err != nil {
			err = fmt.Errorf("Failed to render env Service %s, error: %v", revisionA.Service.ServiceName, err)
			return resp, err
		}
		resp.YamlA = parsedYaml
		resp.VariableYamlA = revisionA.Service.VariableYaml
	} else if revisionA.Service.Type == setting.HelmDeployType {
		resp.VariableYamlB, err = GetHelmMergedValues(revisionA)
	}

	revisionB, err := mongodb.NewEnvServiceVersionColl().Find(projectName, envName, serviceName, isProduction, versionB)
	if err != nil {
		return resp, e.ErrDiffServiceTemplateVersions.AddErr(fmt.Errorf("failed to find %s/%s service for version B %d, isProduction %v, error: %v", projectName, serviceName, versionA, isProduction, err))
	}
	if revisionB.Service.Type == setting.K8SDeployType {
		fakeEnv := &commonmodels.Product{
			ProductName: revisionB.ProductName,
			EnvName:     revisionB.EnvName,
			Namespace:   revisionB.EnvName,
			Production:  revisionB.Production,
		}
		parsedYaml, err := kube.RenderEnvService(fakeEnv, revisionB.Service.GetServiceRender(), revisionB.Service)
		if err != nil {
			err = fmt.Errorf("Failed to render env Service %s, error: %v", revisionB.Service.ServiceName, err)
			return resp, err
		}
		resp.YamlB = parsedYaml
		resp.VariableYamlB = revisionB.Service.VariableYaml
	} else if revisionB.Service.Type == setting.HelmDeployType {
		resp.VariableYamlB, err = GetHelmMergedValues(revisionB)
		if err != nil {
			return resp, e.ErrDiffServiceTemplateVersions.AddErr(fmt.Errorf("failed to get helm merged values for %s/%s/%s service for version B %d, isProduction %v, error: %v", projectName, envName, serviceName, versionA, isProduction, err))
		}
	}

	return resp, nil
}

func RollbackEnvServiceVersion(ctx *internalhandler.Context, projectName, envName, serviceName string, version int64, isProduction bool, log *zap.SugaredLogger) error {
	var (
		service *models.Service
		err     error
	)
	if isProduction {
		service, err = mongodb.NewProductionServiceColl().Find(&mongodb.ServiceFindOption{
			ProductName: projectName,
			ServiceName: serviceName,
			Revision:    version,
		})
	} else {
		service, err = mongodb.NewServiceColl().Find(&mongodb.ServiceFindOption{
			ProductName: projectName,
			ServiceName: serviceName,
			Revision:    version,
		})
	}
	if err != nil {
		return fmt.Errorf("failed to find %s/%s service for version A %d, error: %v", projectName, serviceName, version, err)
	}

	rev, err := commonutil.GenerateServiceNextRevision(isProduction, serviceName, projectName)
	if err != nil {
		return e.ErrRollbackServiceTemplateVersion.AddErr(fmt.Errorf("failed to generate service %s/%s revision, isProduction: %v, error: %v", projectName, serviceName, isProduction, err))
	}

	// helm rollback chart
	if service.Type == setting.HelmDeployType {
		localBase := config.LocalServicePathWithRevision(service.ProductName, service.ServiceName, fmt.Sprint(service.Revision), isProduction)
		s3Base := config.ObjectStorageServicePath(service.ProductName, service.ServiceName, isProduction)
		serviceNameWithRevision := config.ServiceNameWithRevision(service.ServiceName, service.Revision)

		// download from s3
		err = fsservice.DownloadAndExtractFilesFromS3(serviceNameWithRevision, localBase, s3Base, log)
		if err != nil {
			return e.ErrRollbackServiceTemplateVersion.AddErr(fmt.Errorf("failed to download and extract %s files from s3, error: %v", serviceNameWithRevision, err))
		}

		// upload it as new version
		if err = commonservice.CopyAndUploadService(projectName, serviceName, localBase+"/"+serviceName, []string{fmt.Sprintf("%s-%d", serviceName, rev)}, isProduction); err != nil {
			return e.ErrRollbackServiceTemplateVersion.AddErr(fmt.Errorf("Failed to save or upload files for service %s in project %s, error: %s", serviceName, projectName, err))
		}
	}

	if service.Type == setting.HelmDeployType {
		err = mongodb.NewServiceColl().UpdateStatus(service.ServiceName, service.ProductName, setting.ProductStatusDeleting)
		log.Errorf("failed to update service %s/%s status to deleting, error: %v", service.ProductName, service.ServiceName, err)
	}

	err = mongodb.NewServiceColl().Delete(service.ServiceName, service.Type, service.ProductName, setting.ProductStatusDeleting, rev)
	if err != nil {
		log.Errorf("failed to delete service %s/%s/%d, error: %v", service.ProductName, service.ServiceName, service.Revision, err)
	}

	service.Revision = rev
	service.CreateBy = ctx.UserName
	service.Status = ""
	if isProduction {
		err = commonrepo.NewProductionServiceColl().Create(service)
	} else {
		err = commonrepo.NewServiceColl().Create(service)
	}
	if err != nil {
		return e.ErrRollbackServiceTemplateVersion.AddErr(fmt.Errorf("failed to create service %s/%s/%d, isProduction %v, error: %v", service.ProductName, service.ServiceName, service.Revision, isProduction, err))
	}

	return nil
}

func GetHelmMergedValues(envSvcVersion *models.EnvServiceVersion) (string, error) {
	return commonutil.GeneHelmMergedValues(envSvcVersion.Service, envSvcVersion.DefaultValues, envSvcVersion.Service.GetServiceRender())
}
