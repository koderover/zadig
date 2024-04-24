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
	"strings"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/v2/pkg/setting"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

type OpenAPIListDeliveryVersionResp struct {
	List  []*OpenAPIDeliveryVersionInfo `json:"list"`
	Total int                           `json:"total"`
}

type OpenAPIDeliveryVersionInfo struct {
	ID          primitive.ObjectID              `json:"id"`
	VersionName string                          `json:"version_name"`
	Type        string                          `json:"type"`
	Status      string                          `json:"status"`
	Labels      []string                        `json:"labels"`
	Description string                          `json:"description"`
	Progress    *OpenAPIDeliveryVersionProgress `json:"progress"`
	CreatedBy   string                          `json:"created_by"`
	CreateTime  int64                           `json:"create_time"`
}

type OpenAPIDeliveryVersionProgress struct {
	SuccessCount int    `json:"success_count"`
	TotalCount   int    `json:"total_count"`
	UploadStatus string `json:"upload_status"`
	Error        string `json:"error"`
}

func OpenAPIListDeliveryVersion(projectName string, pageNum, pageSize int) (*OpenAPIListDeliveryVersionResp, error) {
	args := new(ListDeliveryVersionArgs)
	args.ProjectName = projectName
	args.Page = pageNum
	args.PerPage = pageSize
	args.Verbosity = VerbosityBrief

	versions, total, err := ListDeliveryVersion(args, log.SugaredLogger())
	if err != nil {
		return nil, fmt.Errorf("failed to list delivery version, error: %v", err)
	}

	resp := make([]*OpenAPIDeliveryVersionInfo, 0)
	for _, version := range versions {
		resp = append(resp, &OpenAPIDeliveryVersionInfo{
			ID:          version.VersionInfo.ID,
			VersionName: version.VersionInfo.Version,
			Type:        version.VersionInfo.Type,
			Status:      version.VersionInfo.Status,
			Description: version.VersionInfo.Desc,
			CreatedBy:   version.VersionInfo.CreatedBy,
			CreateTime:  version.VersionInfo.CreatedAt,
		})
	}
	return &OpenAPIListDeliveryVersionResp{
		List:  resp,
		Total: total,
	}, nil
}

type OpenAPIGetDeliveryVersionResp struct {
	VersionInfo     *OpenAPIDeliveryVersionInfo      `json:"version_info"`
	DeployInfos     []*OpenAPIDeliveryDeployInfo     `json:"deploy_infos"`
	DistributeInfos []*OpenAPIDeliveryDistributeInfo `json:"distribute_infos"`
}

// only used for helm chart and image
// ServiceName = ChartName
// ServiceModule = ImageName
type OpenAPIDeliveryDistributeInfo struct {
	ID             primitive.ObjectID    `json:"id"`
	ServiceName    string                `json:"service_name"`
	DistributeType config.DistributeType `json:"distribute_type"`

	// for helm chart
	ChartName     string `json:"chart_name"`
	ChartRepoName string `json:"chart_repo_name"`
	ChartVersion  string `json:"chart_version"`
	// for image
	ServiceModule string `json:"service_module"`
	Image         string `json:"image"`
	ImageName     string `json:"image_name"`
	Namespace     string `json:"namespace"`

	CreateTime     int64                            `json:"create_time"`
	SubDistributes []*OpenAPIDeliveryDistributeInfo `json:"sub_distributes"`
}

// only used for k8s
type OpenAPIDeliveryDeployInfo struct {
	ID            primitive.ObjectID `json:"id"`
	ServiceName   string             `json:"service_name"`
	ServiceModule string             `json:"service_module"`
	Image         string             `json:"image"`
	ImageName     string             `json:"image_name"`
	RegistryID    string             `json:"registry_id"`

	CreateTime int64 `json:"create_time"`
}

func OpenAPIGetDeliveryVersion(ID string) (*OpenAPIGetDeliveryVersionResp, error) {
	version := new(commonrepo.DeliveryVersionArgs)
	version.ID = ID
	data, err := GetDetailReleaseData(version, log.SugaredLogger())
	if err != nil {
		return nil, fmt.Errorf("failed to get delivery version, ID: %s, error: %v", ID, err)
	}

	resp := new(OpenAPIGetDeliveryVersionResp)
	resp.VersionInfo = &OpenAPIDeliveryVersionInfo{
		ID:          data.VersionInfo.ID,
		VersionName: data.VersionInfo.Version,
		Type:        data.VersionInfo.Type,
		Status:      data.VersionInfo.Status,
		Labels:      data.VersionInfo.Labels,
		Description: data.VersionInfo.Desc,
		Progress: &OpenAPIDeliveryVersionProgress{
			SuccessCount: data.VersionInfo.Progress.SuccessCount,
			TotalCount:   data.VersionInfo.Progress.TotalCount,
			UploadStatus: data.VersionInfo.Progress.UploadStatus,
			Error:        data.VersionInfo.Progress.Error,
		},
		CreatedBy:  data.VersionInfo.CreatedBy,
		CreateTime: data.VersionInfo.CreatedAt,
	}
	resp.DeployInfos = make([]*OpenAPIDeliveryDeployInfo, 0)
	resp.DistributeInfos = make([]*OpenAPIDeliveryDistributeInfo, 0)

	if resp.VersionInfo.Type == setting.DeliveryVersionTypeYaml {
		for _, info := range data.DeployInfo {
			openAPIInfo := &OpenAPIDeliveryDeployInfo{
				ID:         info.ID,
				CreateTime: info.CreatedAt,
			}
			openAPIInfo.ServiceName = info.RealServiceName
			openAPIInfo.Image = info.Image
			openAPIInfo.ImageName = info.ImageName
			openAPIInfo.ServiceModule = info.ServiceModule
			openAPIInfo.RegistryID = info.RegistryID
			resp.DeployInfos = append(resp.DeployInfos, openAPIInfo)
		}
	} else if resp.VersionInfo.Type == setting.DeliveryVersionTypeChart {
		setDistributeInfo := func(info *commonmodels.DeliveryDistribute) *OpenAPIDeliveryDistributeInfo {
			openAPIInfo := &OpenAPIDeliveryDistributeInfo{
				ID:             info.ID,
				DistributeType: info.DistributeType,
				CreateTime:     info.CreatedAt,
				SubDistributes: make([]*OpenAPIDeliveryDistributeInfo, 0),
			}
			if info.DistributeType == config.Chart {
				openAPIInfo.ServiceName = info.ChartName
				openAPIInfo.ChartName = info.ChartName
				openAPIInfo.ChartRepoName = info.ChartRepoName
				openAPIInfo.ChartVersion = info.ChartVersion
			} else if info.DistributeType == config.Image {
				openAPIInfo.ServiceName = info.ChartName
				openAPIInfo.Image = info.Image
				openAPIInfo.ImageName = info.ImageName
				openAPIInfo.ServiceModule = info.ImageName
				openAPIInfo.Namespace = info.Namespace
			}
			return openAPIInfo
		}

		for _, info := range data.DistributeInfo {
			openAPIInfo := setDistributeInfo(info)
			if len(info.SubDistributes) > 0 {
				for _, subInfo := range info.SubDistributes {
					subOpenAPIInfo := setDistributeInfo(subInfo)
					openAPIInfo.SubDistributes = append(openAPIInfo.SubDistributes, subOpenAPIInfo)
				}
			}
			resp.DistributeInfos = append(resp.DistributeInfos, openAPIInfo)
		}
	}

	return resp, nil
}

func OpenAPIDeleteDeliveryVersion(ID string) error {
	logger := log.SugaredLogger()
	version := new(commonrepo.DeliveryVersionArgs)
	version.ID = ID
	ctxErr := DeleteDeliveryVersion(version, logger)
	if ctxErr != nil {
		return fmt.Errorf("failed to delete delivery version, ID: %s, error: %v", ID, ctxErr)
	}

	errs := make([]string, 0)
	err := DeleteDeliveryBuild(&commonrepo.DeliveryBuildArgs{ReleaseID: ID}, logger)
	if err != nil {
		errs = append(errs, err.Error())
	}
	err = DeleteDeliveryDeploy(&commonrepo.DeliveryDeployArgs{ReleaseID: ID}, logger)
	if err != nil {
		errs = append(errs, err.Error())
	}
	err = DeleteDeliveryTest(&commonrepo.DeliveryTestArgs{ReleaseID: ID}, logger)
	if err != nil {
		errs = append(errs, err.Error())
	}
	err = DeleteDeliveryDistribute(&commonrepo.DeliveryDistributeArgs{ReleaseID: ID}, logger)
	if err != nil {
		errs = append(errs, err.Error())
	}

	if len(errs) != 0 {
		ctxErr = e.NewHTTPError(500, strings.Join(errs, ","))
	}
	return ctxErr
}

type OpenAPICreateK8SDeliveryVersionRequest struct {
	ProjectKey      string                                     `json:"project_key"`
	VersionName     string                                     `json:"version_name"`
	Retry           bool                                       `json:"retry"`
	EnvName         string                                     `json:"env_name"`
	Production      bool                                       `json:"production"`
	Desc            string                                     `json:"desc"`
	Labels          []string                                   `json:"labels"`
	ImageRegistryID string                                     `json:"image_registry_id"`
	YamlDatas       []*OpenAPICreateK8SDeliveryVersionYamlData `json:"yaml_datas"`
	CreateBy        string                                     `json:"-"`
}

type OpenAPICreateK8SDeliveryVersionYamlData struct {
	ServiceName string                             `json:"service_name"`
	YamlContent string                             `json:"-"`
	ImageDatas  []*OpenAPIDeliveryVersionImageData `json:"image_datas"`
}

type OpenAPIDeliveryVersionImageData struct {
	ContainerName string `json:"container_name"`
	ImageName     string `json:"image_name"`
	ImageTag      string `json:"image_tag"`
}

func OpenAPICreateK8SDeliveryVersion(openAPIReq *OpenAPICreateK8SDeliveryVersionRequest) error {
	env, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:       openAPIReq.ProjectKey,
		EnvName:    openAPIReq.EnvName,
		Production: &openAPIReq.Production,
	})
	if err != nil {
		return fmt.Errorf("failed to find product, product name: %s, env name: %s, error: %v", openAPIReq.ProjectKey, openAPIReq.EnvName, err)
	}
	prodSvcMap := env.GetServiceMap()

	yamlDatas := make([]*CreateK8SDeliveryVersionYamlData, 0)
	for _, openAPIYamlData := range openAPIReq.YamlDatas {
		prodSvc := prodSvcMap[openAPIYamlData.ServiceName]
		if prodSvc == nil {
			return fmt.Errorf("product service not found, service name: %s", openAPIYamlData.ServiceName)
		}
		containerImageMap := prodSvc.GetContainerImageMap()

		imageDatas := make([]*ImageData, 0)
		for _, openAPIImageData := range openAPIYamlData.ImageDatas {
			image := containerImageMap[openAPIImageData.ContainerName]
			if image == "" {
				return fmt.Errorf("container image not found, product name: %s, env name: %s, service name: %s, container name: %s", openAPIReq.ProjectKey, openAPIReq.EnvName, openAPIYamlData.ServiceName, openAPIImageData.ContainerName)
			}

			imageDatas = append(imageDatas, &ImageData{
				ContainerName: openAPIImageData.ContainerName,
				Image:         image,
				ImageName:     openAPIImageData.ImageName,
				ImageTag:      openAPIImageData.ImageTag,
				Selected:      true,
			})
		}

		yamlContent, _, err := kube.FetchCurrentAppliedYaml(&kube.GeneSvcYamlOption{
			ProductName: openAPIReq.ProjectKey,
			EnvName:     openAPIReq.EnvName,
			ServiceName: openAPIYamlData.ServiceName,
		})
		if err != nil {
			return fmt.Errorf("failed to fetch current applied yaml, env: %s/%s, service: %s, error: %v", openAPIReq.ProjectKey, openAPIReq.EnvName, openAPIYamlData.ServiceName, err)
		}

		yamlDatas = append(yamlDatas, &CreateK8SDeliveryVersionYamlData{
			ServiceName: openAPIYamlData.ServiceName,
			YamlContent: yamlContent,
			ImageDatas:  imageDatas,
		})
	}

	args := &CreateK8SDeliveryVersionArgs{
		ProductName: openAPIReq.ProjectKey,
		Retry:       openAPIReq.Retry,
		CreateBy:    openAPIReq.CreateBy,
		Version:     openAPIReq.VersionName,
		Desc:        openAPIReq.Desc,
		EnvName:     openAPIReq.EnvName,
		Production:  openAPIReq.Production,
		Labels:      openAPIReq.Labels,
		DeliveryVersionYamlData: &DeliveryVersionYamlData{
			ImageRegistryID: openAPIReq.ImageRegistryID,
			YamlDatas:       yamlDatas,
		},
	}
	err = CreateK8SDeliveryVersion(args, log.SugaredLogger())
	if err != nil {
		return fmt.Errorf("failed to create k8s delivery version, project name: %s, version name %s, retry: %v, error: %v",
			args.ProductName, args.Version, args.Retry, err)
	}

	return nil
}

type OpenAPICreateHelmDeliveryVersionRequest struct {
	ProjectKey      string                                       `json:"project_key"`
	VersionName     string                                       `json:"version_name"`
	Retry           bool                                         `json:"retry"`
	EnvName         string                                       `json:"env_name"`
	Production      bool                                         `json:"production"`
	Desc            string                                       `json:"desc"`
	Labels          []string                                     `json:"labels"`
	ImageRegistryID string                                       `json:"image_registry_id"`
	ChartRepoName   string                                       `json:"chart_repo_name"`
	ChartDatas      []*OpenAPICreateHelmDeliveryVersionChartData `json:"chart_datas"`
	CreateBy        string                                       `json:"-"`
}

type OpenAPICreateHelmDeliveryVersionChartData struct {
	ServiceName string                             `json:"service_name"`
	Version     string                             `json:"version"`
	ImageDatas  []*OpenAPIDeliveryVersionImageData `json:"image_datas"`
}

func OpenAPICreateHelmDeliveryVersion(openAPIReq *OpenAPICreateHelmDeliveryVersionRequest) error {
	chartDatas := make([]*CreateHelmDeliveryVersionChartData, 0)
	for _, openAPIChartData := range openAPIReq.ChartDatas {
		imageDatas := make([]*ImageData, 0)
		for _, openAPIImageData := range openAPIChartData.ImageDatas {
			imageDatas = append(imageDatas, &ImageData{
				ContainerName: openAPIImageData.ContainerName,
				ImageName:     openAPIImageData.ImageName,
				ImageTag:      openAPIImageData.ImageTag,
				Selected:      true,
			})
		}

		chartDatas = append(chartDatas, &CreateHelmDeliveryVersionChartData{
			ServiceName: openAPIChartData.ServiceName,
			Version:     openAPIChartData.Version,
			ImageData:   imageDatas,
		})
	}

	args := &CreateHelmDeliveryVersionArgs{
		ProductName: openAPIReq.ProjectKey,
		Retry:       openAPIReq.Retry,
		CreateBy:    openAPIReq.CreateBy,
		Version:     openAPIReq.VersionName,
		Desc:        openAPIReq.Desc,
		EnvName:     openAPIReq.EnvName,
		Production:  openAPIReq.Production,
		Labels:      openAPIReq.Labels,
		DeliveryVersionChartData: &DeliveryVersionChartData{
			ChartRepoName:   openAPIReq.ChartRepoName,
			ImageRegistryID: openAPIReq.ImageRegistryID,
			ChartDatas:      chartDatas,
		},
	}
	err := CreateHelmDeliveryVersion(args, log.SugaredLogger())
	if err != nil {
		return fmt.Errorf("failed to create k8s delivery version, project name: %s, version name %s, retry: %v, error: %v",
			args.ProductName, args.Version, args.Retry, err)
	}

	return nil
}
