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

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	fsservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/fs"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/repository"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/setting"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
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

type GetServiceVersionYamlResponse struct {
	Type         string `json:"type"`
	Yaml         string `json:"yaml"`
	VariableYaml string `json:"variable_yaml"`
}

func GetServiceVersionYaml(ctx *internalhandler.Context, projectName, serviceName string, revision int64, isProduction bool, log *zap.SugaredLogger) (GetServiceVersionYamlResponse, error) {
	resp := GetServiceVersionYamlResponse{}

	svcRevision, err := repository.QueryTemplateService(&commonrepo.ServiceFindOption{
		ProductName: projectName,
		ServiceName: serviceName,
		Revision:    revision,
	}, isProduction)
	if err != nil {
		return resp, e.ErrDiffServiceTemplateVersions.AddErr(fmt.Errorf("failed to find %s/%s service for version A %d, isProduction %v, error: %v", projectName, serviceName, revision, isProduction, err))
	}
	resp.Type = svcRevision.Type
	if svcRevision.Type == setting.K8SDeployType {
		resp.Yaml = svcRevision.Yaml
		resp.VariableYaml = svcRevision.VariableYaml
	} else if svcRevision.Type == setting.HelmDeployType {
		resp.VariableYaml = svcRevision.HelmChart.ValuesYaml
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

func DiffServiceVersions(ctx *internalhandler.Context, projectName, serviceName string, revisionA, revisionB int64, isProduction bool, log *zap.SugaredLogger) (DiffServiceVersionsResponse, error) {
	resp := DiffServiceVersionsResponse{}

	svcRevisionA, err := repository.QueryTemplateService(&commonrepo.ServiceFindOption{
		ProductName: projectName,
		ServiceName: serviceName,
		Revision:    revisionA,
	}, isProduction)
	if err != nil {
		return resp, e.ErrDiffServiceTemplateVersions.AddErr(fmt.Errorf("failed to find %s/%s service for version A %d, isProduction %v, error: %v", projectName, serviceName, revisionA, isProduction, err))
	}
	resp.Type = svcRevisionA.Type
	if svcRevisionA.Type == setting.K8SDeployType {
		resp.YamlA = svcRevisionA.Yaml
		resp.VariableYamlA = svcRevisionA.VariableYaml
	} else if svcRevisionA.Type == setting.HelmDeployType {
		resp.VariableYamlA = svcRevisionA.HelmChart.ValuesYaml
	}

	svcRevisionB, err := repository.QueryTemplateService(&commonrepo.ServiceFindOption{
		ProductName: projectName,
		ServiceName: serviceName,
		Revision:    revisionB,
	}, isProduction)
	if err != nil {
		return resp, e.ErrDiffServiceTemplateVersions.AddErr(fmt.Errorf("failed to find %s/%s service for version B %d, isProduction %v, error: %v", projectName, serviceName, revisionA, isProduction, err))
	}
	if svcRevisionA.Type == setting.K8SDeployType {
		resp.YamlB = svcRevisionB.Yaml
		resp.VariableYamlB = svcRevisionB.VariableYaml
	} else if svcRevisionB.Type == setting.HelmDeployType {
		resp.VariableYamlB = svcRevisionB.HelmChart.ValuesYaml
	}

	return resp, nil
}

func RollbackServiceVersion(ctx *internalhandler.Context, projectName, serviceName string, revision int64, isProduction bool, log *zap.SugaredLogger) error {
	service, err := repository.QueryTemplateService(&commonrepo.ServiceFindOption{
		ProductName: projectName,
		ServiceName: serviceName,
		Revision:    revision,
	}, isProduction)
	if err != nil {
		return fmt.Errorf("failed to find %s/%s service for version A %d, error: %v", projectName, serviceName, revision, err)
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
	err = repository.Create(service, isProduction)
	if err != nil {
		return e.ErrRollbackServiceTemplateVersion.AddErr(fmt.Errorf("failed to create service %s/%s/%d, isProduction %v, error: %v", service.ProductName, service.ServiceName, service.Revision, isProduction, err))
	}

	return nil
}
