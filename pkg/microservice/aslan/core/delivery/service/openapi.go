/*
 * Copyright 2024 The KodeRover Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package service

import (
	"fmt"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

type OpenAPIListDeliveryVersionV2Resp struct {
	List  []*OpenAPIDeliveryVersionInfoV2 `json:"list"`
	Total int                             `json:"total"`
}

type OpenAPIDeliveryVersionInfoV2 struct {
	ID          primitive.ObjectID            `json:"id"`
	VersionName string                        `json:"version_name"`
	Type        setting.DeliveryVersionType   `json:"type"`
	Source      setting.DeliveryVersionSource `json:"source"`
	Status      setting.DeliveryVersionStatus `json:"status"`
	Labels      []string                      `json:"labels"`
	Description string                        `json:"description"`
	CreatedBy   string                        `json:"created_by"`
	CreateTime  int64                         `json:"create_time"`
}

func OpenAPIListDeliveryVersion(projectName string, pageNum, pageSize int) (*OpenAPIListDeliveryVersionV2Resp, error) {
	args := new(ListDeliveryVersionV2Args)
	args.ProjectName = projectName
	args.Page = pageNum
	args.PerPage = pageSize
	args.Verbosity = VerbosityBrief

	versions, total, err := ListDeliveryVersionV2(args, log.SugaredLogger())
	if err != nil {
		return nil, fmt.Errorf("failed to list delivery version, error: %v", err)
	}

	resp := make([]*OpenAPIDeliveryVersionInfoV2, 0)
	for _, version := range versions {
		resp = append(resp, &OpenAPIDeliveryVersionInfoV2{
			ID:          version.ID,
			VersionName: version.Version,
			Type:        version.Type,
			Status:      version.Status,
			Labels:      version.Labels,
			Source:      version.Source,
			Description: version.Desc,
			CreatedBy:   version.CreatedBy,
			CreateTime:  version.CreatedAt,
		})
	}
	return &OpenAPIListDeliveryVersionV2Resp{
		List:  resp,
		Total: total,
	}, nil
}

type OpenAPIDeliveryVersionService struct {
	ServiceName          string                         `json:"service_name"`
	ChartName            string                         `json:"chart_name"`
	OriginalChartVersion string                         `json:"original_chart_version"`
	ChartVersion         string                         `json:"chart_version"`
	ChartStatus          config.Status                  `json:"chart_status"`
	YamlContent          string                         `json:"yaml_content"`
	Images               []*OpenAPIDeliveryVersionImage `json:"images"`
	Error                string                         `json:"error"`
}

type OpenAPIDeliveryVersionImage struct {
	ContainerName  string                      `json:"container_name"`
	ImageName      string                      `json:"image_name"`
	SourceImage    string                      `json:"source_image"`
	SourceImageTag string                      `json:"source_image_tag"`
	TargetImage    string                      `json:"target_image"`
	TargetImageTag string                      `json:"target_image_tag"`
	ImagePath      *commonmodels.ImagePathSpec `json:"image_path"`
	PushImage      bool                        `json:"push_image"`
	Status         config.Status               `json:"status"`
	Error          string                      `json:"error"`
}

type OpenAPIGetDeliveryVersionV2Resp struct {
	Version         string                           `bson:"version"                 json:"version"`
	ProjectName     string                           `bson:"project_name"            json:"project_name"`
	EnvName         string                           `bson:"env_name"                json:"env_name"`
	Production      bool                             `bson:"production"              json:"production"`
	Type            setting.DeliveryVersionType      `bson:"type"                    json:"type"`
	Source          setting.DeliveryVersionSource    `bson:"source"                  json:"source"`
	Desc            string                           `bson:"desc"                    json:"desc"`
	Labels          []string                         `bson:"labels"                  json:"labels"`
	ImageRegistryID string                           `bson:"image_registry_id"       json:"image_registry_id"`
	ChartRepoName   string                           `bson:"chart_repo_name"         json:"chart_repo_name"`
	Services        []*OpenAPIDeliveryVersionService `bson:"services"                json:"services"`
	Status          setting.DeliveryVersionStatus    `bson:"status"                  json:"status"`
	Error           string                           `bson:"error"                   json:"error"`
	CreatedBy       string                           `bson:"created_by"              json:"created_by"`
	CreatedAt       int64                            `bson:"created_at"              json:"created_at"`
	DeletedAt       int64                            `bson:"deleted_at"              json:"deleted_at"`
}

func OpenAPIGetDeliveryVersion(projectName, versionName string) (*OpenAPIGetDeliveryVersionV2Resp, error) {
	version, err := commonrepo.NewDeliveryVersionV2Coll().Get(&commonrepo.DeliveryVersionV2Args{
		ProjectName: projectName,
		Version:     versionName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get delivery version, projectName: %s, versionName: %s error: %v", projectName, versionName, err)
	}

	resp := &OpenAPIGetDeliveryVersionV2Resp{
		Version:         version.Version,
		ProjectName:     version.ProjectName,
		EnvName:         version.EnvName,
		Production:      version.Production,
		Type:            version.Type,
		Source:          version.Source,
		Status:          version.Status,
		Desc:            version.Desc,
		Labels:          version.Labels,
		ImageRegistryID: version.ImageRegistryID,
		ChartRepoName:   version.ChartRepoName,
		Error:           version.Error,
		CreatedBy:       version.CreatedBy,
		CreatedAt:       version.CreatedAt,
	}

	openapiServices := make([]*OpenAPIDeliveryVersionService, 0)
	for _, service := range version.Services {
		openapiService := &OpenAPIDeliveryVersionService{
			ServiceName:  service.ServiceName,
			ChartName:    service.ChartName,
			ChartVersion: service.ChartVersion,
			ChartStatus:  service.ChartStatus,
			YamlContent:  service.YamlContent,
			Error:        service.Error,
		}

		images := make([]*OpenAPIDeliveryVersionImage, 0)
		for _, image := range service.Images {
			images = append(images, &OpenAPIDeliveryVersionImage{
				ContainerName:  image.ContainerName,
				ImageName:      image.ImageName,
				SourceImage:    image.SourceImage,
				SourceImageTag: image.SourceImageTag,
				TargetImage:    image.TargetImage,
				TargetImageTag: image.TargetImageTag,
				PushImage:      image.PushImage,
				ImagePath:      image.ImagePath,
				Status:         image.Status,
				Error:          image.Error,
			})
		}

		openapiService.Images = images

		openapiServices = append(openapiServices, openapiService)
	}
	resp.Services = openapiServices

	return resp, nil
}

func OpenAPIDeleteDeliveryVersion(ID string) error {
	logger := log.SugaredLogger()
	version := new(commonrepo.DeliveryVersionV2Args)
	version.ID = ID
	ctxErr := DeleteDeliveryVersionV2(version, logger)
	if ctxErr != nil {
		return fmt.Errorf("failed to delete delivery version, ID: %s, error: %v", ID, ctxErr)
	}

	return ctxErr
}

type OpenAPICreateK8SDeliveryVersionV2Request struct {
	ProjectKey      string                           `json:"project_key"`
	VersionName     string                           `json:"version_name"`
	Source          setting.DeliveryVersionSource    `json:"source"`
	EnvName         string                           `json:"env_name"`
	Production      bool                             `json:"production"`
	Desc            string                           `json:"desc"`
	Labels          []string                         `json:"labels"`
	ImageRegistryID string                           `json:"image_registry_id"`
	Services        []*OpenAPIDeliveryVersionService `json:"services"`
	CreateBy        string                           `json:"-"`
}

func OpenAPICreateK8SDeliveryVersion(openAPIReq *OpenAPICreateK8SDeliveryVersionV2Request) error {
	services := make([]*commonmodels.DeliveryVersionService, 0)
	for _, service := range openAPIReq.Services {
		images := make([]*commonmodels.DeliveryVersionImage, 0)
		for _, image := range service.Images {
			images = append(images, &commonmodels.DeliveryVersionImage{
				ContainerName:  image.ContainerName,
				ImageName:      image.ImageName,
				SourceImage:    image.SourceImage,
				SourceImageTag: image.SourceImageTag,
				TargetImage:    image.TargetImage,
				TargetImageTag: image.TargetImageTag,
				PushImage:      image.PushImage,
			})
		}

		service := &commonmodels.DeliveryVersionService{
			ServiceName: service.ServiceName,
			YamlContent: service.YamlContent,
			Images:      images,
		}

		services = append(services, service)
	}

	createDeliveryVersionRequest := &CreateDeliveryVersionRequest{
		Version:         openAPIReq.VersionName,
		ProjectName:     openAPIReq.ProjectKey,
		EnvName:         openAPIReq.EnvName,
		Production:      openAPIReq.Production,
		Source:          openAPIReq.Source,
		Labels:          openAPIReq.Labels,
		Desc:            openAPIReq.Desc,
		CreateBy:        openAPIReq.CreateBy,
		Services:        services,
		ImageRegistryID: openAPIReq.ImageRegistryID,
	}
	return CreateK8SDeliveryVersionV2(createDeliveryVersionRequest, log.SugaredLogger())
}

type OpenAPICreateHelmDeliveryVersionV2Request struct {
	ProjectKey            string                           `json:"project_key"`
	VersionName           string                           `json:"version_name"`
	EnvName               string                           `json:"env_name"`
	Production            bool                             `json:"production"`
	Source                setting.DeliveryVersionSource    `json:"source"`
	Desc                  string                           `json:"desc"`
	Labels                []string                         `json:"labels"`
	ImageRegistryID       string                           `json:"image_registry_id"`
	ChartRepoName         string                           `json:"chart_repo_name"`
	OriginalChartRepoName string                           `json:"original_chart_repo_name"`
	Services              []*OpenAPIDeliveryVersionService `json:"services"`
	CreateBy              string                           `json:"-"`
}

func OpenAPICreateHelmDeliveryVersion(openAPIReq *OpenAPICreateHelmDeliveryVersionV2Request) error {
	services := make([]*commonmodels.DeliveryVersionService, 0)
	for _, service := range openAPIReq.Services {
		images := make([]*commonmodels.DeliveryVersionImage, 0)
		for _, image := range service.Images {
			images = append(images, &commonmodels.DeliveryVersionImage{
				ContainerName:  image.ContainerName,
				ImageName:      image.ImageName,
				SourceImage:    image.SourceImage,
				SourceImageTag: image.SourceImageTag,
				TargetImage:    image.TargetImage,
				TargetImageTag: image.TargetImageTag,
				ImagePath:      image.ImagePath,
				PushImage:      image.PushImage,
			})
		}

		service := &commonmodels.DeliveryVersionService{
			ServiceName:          service.ServiceName,
			YamlContent:          service.YamlContent,
			OriginalChartVersion: service.OriginalChartVersion,
			ChartVersion:         service.ChartVersion,
			Images:               images,
		}

		services = append(services, service)
	}

	createDeliveryVersionRequest := &CreateDeliveryVersionRequest{
		Version:               openAPIReq.VersionName,
		ProjectName:           openAPIReq.ProjectKey,
		EnvName:               openAPIReq.EnvName,
		Production:            openAPIReq.Production,
		Source:                openAPIReq.Source,
		Labels:                openAPIReq.Labels,
		Desc:                  openAPIReq.Desc,
		ImageRegistryID:       openAPIReq.ImageRegistryID,
		ChartRepoName:         openAPIReq.ChartRepoName,
		OriginalChartRepoName: openAPIReq.OriginalChartRepoName,
		Services:              services,
		CreateBy:              openAPIReq.CreateBy,
	}

	return CreateHelmDeliveryVersionV2(createDeliveryVersionRequest, log.SugaredLogger())
}

func OpenAPIRetryCreateDeliveryVersion(id string) error {
	logger := log.SugaredLogger()

	ctxErr := RetryDeliveryVersionV2(id, logger)
	if ctxErr != nil {
		return fmt.Errorf("failed to retry create delivery version, id: %s, error: %v", id, ctxErr)
	}

	return ctxErr
}
