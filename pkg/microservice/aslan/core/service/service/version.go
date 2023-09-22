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
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	fsservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/fs"
	commonutil "github.com/koderover/zadig/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/pkg/setting"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

type ListServiceVersionsResponse struct {
	ServiceName string `json:"service_name"`
	Revision    int64  `json:"revision"`
	CreateTime  int64  `json:"create_time"`
	CreateBy    string `json:"create_by"`
}

func ListServiceVersions(ctx *internalhandler.Context, projectName, serviceName string, isProduction bool, log *zap.SugaredLogger) ([]ListServiceVersionsResponse, error) {
	var (
		revisions []*models.Service
		err       error
		resp      []ListServiceVersionsResponse
	)
	if isProduction {
		revisions, err = mongodb.NewProductionServiceColl().ListServiceAllRevisionsAndStatus(serviceName, projectName)
	} else {
		revisions, err = mongodb.NewServiceColl().ListServiceAllRevisionsAndStatus(serviceName, projectName)
	}
	if err != nil {
		return nil, e.ErrListServiceTemplateVersions.AddErr(fmt.Errorf("failed to list service revisions, error: %v", err))
	}

	for _, revision := range revisions {
		resp = append(resp, ListServiceVersionsResponse{
			ServiceName: revision.ServiceName,
			Revision:    revision.Revision,
			CreateTime:  revision.CreateTime,
			CreateBy:    revision.CreateBy,
		})
	}
	return resp, nil
}

type DiffServiceVersionsResponse struct {
	Type          string `json:"type"`
	YamlA         string `json:"yaml_a"`
	YamlB         string `json:"yaml_b"`
	VariableYamlA string `json:"variable_yaml_a"`
	VariableYamlB string `json:"variable_yaml_b"`
}

func DiffServiceVersions(ctx *internalhandler.Context, projectName, serviceName string, versionA, versionB int64, isProduction bool, log *zap.SugaredLogger) (DiffServiceVersionsResponse, error) {
	var (
		revisionA, revisionB *models.Service
		err                  error
		resp                 DiffServiceVersionsResponse
	)

	if isProduction {
		revisionA, err = mongodb.NewProductionServiceColl().Find(&mongodb.ServiceFindOption{
			ProductName: projectName,
			ServiceName: serviceName,
			Revision:    versionA,
		})
	} else {
		revisionA, err = mongodb.NewServiceColl().Find(&mongodb.ServiceFindOption{
			ProductName: projectName,
			ServiceName: serviceName,
			Revision:    versionA,
		})

	}
	if err != nil {
		return resp, e.ErrDiffServiceTemplateVersions.AddErr(fmt.Errorf("failed to find %s/%s service for version A %d, isProduction %v, error: %v", projectName, serviceName, versionA, isProduction, err))
	}
	resp.Type = revisionA.Type
	if revisionA.Type == setting.K8SDeployType {
		resp.YamlA = revisionA.Yaml
		resp.VariableYamlA = revisionA.VariableYaml
	} else if revisionA.Type == setting.HelmDeployType {
		resp.YamlA = revisionA.HelmChart.ValuesYaml
	}

	if isProduction {
		revisionB, err = mongodb.NewProductionServiceColl().Find(&mongodb.ServiceFindOption{
			ProductName: projectName,
			ServiceName: serviceName,
			Revision:    versionB,
		})
	} else {
		revisionB, err = mongodb.NewServiceColl().Find(&mongodb.ServiceFindOption{
			ProductName: projectName,
			ServiceName: serviceName,
			Revision:    versionB,
		})
	}
	if err != nil {
		return resp, e.ErrDiffServiceTemplateVersions.AddErr(fmt.Errorf("failed to find %s/%s service for version B %d, isProduction %v, error: %v", projectName, serviceName, versionA, isProduction, err))
	}
	if revisionA.Type == setting.K8SDeployType {
		resp.YamlB = revisionB.Yaml
		resp.VariableYamlB = revisionB.VariableYaml
	} else if revisionB.Type == setting.HelmDeployType {
		resp.YamlB = revisionB.HelmChart.ValuesYaml
	}

	return resp, nil
}

func RollbackServiceVersion(ctx *internalhandler.Context, projectName, serviceName string, version int64, isProduction bool, log *zap.SugaredLogger) error {
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
