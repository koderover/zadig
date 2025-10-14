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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/repository"

	"github.com/hashicorp/go-multierror"
	"github.com/otiai10/copy"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/release"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/yaml"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/setting"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	helmtool "github.com/koderover/zadig/v2/pkg/tool/helmclient"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/util"
	"github.com/koderover/zadig/v2/pkg/util/converter"
	jsonutil "github.com/koderover/zadig/v2/pkg/util/json"
)

type HelmReleaseQueryArgs struct {
	ProjectName string `json:"projectName"     form:"projectName"`
	Filter      string `json:"filter"          form:"filter"`
}

type ReleaseFilter struct {
	Name   string        `json:"name"`
	Status ReleaseStatus `json:"status"`
}

type HelmReleaseResp struct {
	ReleaseName       string        `json:"releaseName"`
	ServiceName       string        `json:"serviceName"`
	Revision          int           `json:"revision"`
	ChartRepo         string        `json:"chartRepo"`
	Chart             string        `json:"chart"`
	ChartVersion      string        `json:"chartVersion"`
	AppVersion        string        `json:"appVersion"`
	OverrideValues    string        `json:"overrideValues"`
	OverrideYaml      string        `json:"overrideYaml"`
	Status            ReleaseStatus `json:"status"`
	Updatable         bool          `json:"updatable"`
	IsHelmChartDeploy bool          `json:"isHelmChartDeploy"`
	DeployStrategy    string        `json:"deployStrategy"`
	Error             string        `json:"error"`
}

type ChartInfo struct {
	ServiceName string `json:"serviceName"`
	Revision    int64  `json:"revision"`
}

type HelmChartsResp struct {
	ChartInfos []*ChartInfo      `json:"chartInfos"`
	FileInfos  []*types.FileInfo `json:"fileInfos"`
}

type SvcDataSet struct {
	ProdSvc    *models.ProductService
	TmplSvc    *models.Service
	SvcRelease *release.Release
}

type ReleaseStatus string

const (
	HelmReleaseStatusPending     ReleaseStatus = "pending"
	HelmReleaseStatusDeployed    ReleaseStatus = "deployed"
	HelmReleaseStatusFailed      ReleaseStatus = "failed"
	HelmReleaseStatusNotDeployed ReleaseStatus = "notDeployed"
)

func listReleaseInNamespace(helmClient *helmtool.HelmClient, filter *ReleaseFilter) ([]*release.Release, error) {
	listClient := action.NewList(helmClient.ActionConfig)

	switch filter.Status {
	case HelmReleaseStatusDeployed:
		listClient.StateMask = action.ListDeployed
	case HelmReleaseStatusFailed:
		listClient.StateMask = action.ListFailed | action.ListSuperseded
	case HelmReleaseStatusPending:
		listClient.StateMask = action.ListPendingRollback | action.ListPendingUpgrade | action.ListPendingInstall | action.ListUninstalling
	default:
		listClient.StateMask = action.ListAll
	}
	listClient.Filter = filter.Name

	return listClient.Run()
}

func getReleaseStatus(re *release.Release) ReleaseStatus {
	switch re.Info.Status {
	case release.StatusDeployed:
		return HelmReleaseStatusDeployed
	case release.StatusFailed, release.StatusSuperseded:
		return HelmReleaseStatusFailed
	case release.StatusPendingRollback, release.StatusPendingUpgrade, release.StatusPendingInstall, release.StatusUninstalling:
		return HelmReleaseStatusPending
	default:
		return HelmReleaseStatusFailed
	}
}

type ImageData struct {
	Image     string `json:"image"`
	ImageName string `json:"imageName"`
	ImageTag  string `json:"imageTag"`
	Selected  bool   `json:"selected"`
}

type ServiceImages struct {
	ServiceName string       `json:"serviceName"`
	Images      []*ImageData `json:"imageData"`
}

type ChartImagesResp struct {
	ServiceImages []*ServiceImages `json:"serviceImages"`
}

func ListReleases(args *HelmReleaseQueryArgs, envName string, production bool, log *zap.SugaredLogger) ([]*HelmReleaseResp, error) {
	projectName, filterStr := args.ProjectName, args.Filter
	opt := &commonrepo.ProductFindOptions{Name: projectName, EnvName: envName, Production: util.GetBoolPointer(production)}
	prod, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		return nil, fmt.Errorf("failed to find project: %s, err: %s", projectName, err)
	}

	helmClientInterface, err := helmtool.NewClientFromNamespace(prod.ClusterID, prod.Namespace)
	if err != nil {
		log.Errorf("[%s][%s] NewClientFromRestConf error: %s", envName, projectName, err)
		return nil, fmt.Errorf("failed to init helm client, err: %s", err)
	}

	filter := &ReleaseFilter{}
	if len(filterStr) > 0 {
		data, err := jsonutil.ToJSON(filterStr)
		if err != nil {
			log.Warnf("invalid filter, err: %s", err)
			return nil, fmt.Errorf("invalid filter, err: %s", err)
		}

		if err = json.Unmarshal(data, filter); err != nil {
			log.Warnf("Invalid filter, err: %s", err)
			return nil, fmt.Errorf("invalid filter, err: %s", err)
		}
	}

	releases, err := listReleaseInNamespace(helmClientInterface, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list release, err: %s", err)
	}
	releaseMap := make(map[string]*release.Release)
	for _, re := range releases {
		releaseMap[re.Name] = re
	}

	svcToReleaseNameMap, err := commonutil.GetServiceNameToReleaseNameMap(prod)
	if err != nil {
		return nil, fmt.Errorf("failed to build release-service map: %s", err)
	}
	svcDatSetMap := make(map[string]*SvcDataSet)
	svcDataList := make([]*SvcDataSet, 0)

	for _, prodSvc := range prod.GetServiceMap() {
		serviceName := prodSvc.ServiceName
		releaseName := svcToReleaseNameMap[serviceName]
		svcDataSet := &SvcDataSet{
			ProdSvc:    prodSvc,
			SvcRelease: releaseMap[releaseName],
		}

		svcDatSetMap[serviceName] = svcDataSet
		svcDataList = append(svcDataList, svcDataSet)
	}
	for _, prodSvc := range prod.GetChartServiceMap() {
		serviceName := prodSvc.ServiceName
		releaseName := prodSvc.ReleaseName
		svcDataSet := &SvcDataSet{
			ProdSvc:    prodSvc,
			SvcRelease: releaseMap[releaseName],
		}

		svcDatSetMap[serviceName] = svcDataSet
		svcDataList = append(svcDataList, svcDataSet)
	}

	// set service template data
	serviceTmpls, err := repository.ListMaxRevisionsServices(projectName, production, false)
	if err != nil {
		return nil, errors.Errorf("failed to list service templates for project: %s", projectName)
	}
	for _, tmplSvc := range serviceTmpls {
		if svcDataSet, ok := svcDatSetMap[tmplSvc.ServiceName]; ok {
			svcDataSet.TmplSvc = tmplSvc
		}
	}

	filterFunc := func(svcDataSet *SvcDataSet) bool {
		// filter by name
		if filter.Name != "" && !strings.Contains(svcDataSet.ProdSvc.ServiceName, filter.Name) {
			return false
		}
		// filter by status
		switch filter.Status {
		case HelmReleaseStatusDeployed:
			if svcDataSet.SvcRelease == nil || svcDataSet.SvcRelease.Info.Status != release.StatusDeployed {
				return false
			}
		case HelmReleaseStatusFailed:
			if svcDataSet.SvcRelease != nil && svcDataSet.SvcRelease.Info.Status != release.StatusFailed && svcDataSet.SvcRelease.Info.Status != release.StatusSuperseded {
				return false
			}
			if svcDataSet.ProdSvc.Error == "" {
				return false
			}
		case HelmReleaseStatusPending:
			if svcDataSet.SvcRelease == nil || (!svcDataSet.SvcRelease.Info.Status.IsPending() && svcDataSet.SvcRelease.Info.Status != release.StatusUninstalling) {
				return false
			}
		case HelmReleaseStatusNotDeployed:
			if svcDataSet.SvcRelease != nil {
				return false
			}
		}
		return true
	}

	ret := make([]*HelmReleaseResp, 0)

	chartInfoMap := prod.GetChartRenderMap()
	chartDeployInfoMap := prod.GetChartDeployRenderMap()
	for _, svcDataSet := range svcDataList {
		if !filterFunc(svcDataSet) {
			continue
		}

		updatable := false
		if svcDataSet.TmplSvc != nil {
			if svcDataSet.ProdSvc.FromZadig() && svcDataSet.ProdSvc.Revision != svcDataSet.TmplSvc.Revision {
				updatable = true
			}
		}

		respObj := &HelmReleaseResp{
			ServiceName:    svcDataSet.ProdSvc.ServiceName,
			Status:         HelmReleaseStatusNotDeployed,
			Updatable:      updatable,
			DeployStrategy: prod.ServiceDeployStrategy[svcDataSet.ProdSvc.ServiceName],
			Error:          svcDataSet.ProdSvc.Error,
		}
		respObj.Updatable = updatable

		if svcDataSet.SvcRelease != nil {
			respObj.ReleaseName = svcDataSet.SvcRelease.Name
			respObj.Revision = svcDataSet.SvcRelease.Version
			respObj.Chart = svcDataSet.SvcRelease.Chart.Name()
			respObj.ChartVersion = svcDataSet.SvcRelease.Chart.Metadata.Version
			respObj.AppVersion = svcDataSet.SvcRelease.Chart.AppVersion()
			respObj.Status = getReleaseStatus(svcDataSet.SvcRelease)
		} else if svcDataSet.TmplSvc != nil {
			// service template deploy
			respObj.ReleaseName = util.GeneReleaseName(svcDataSet.TmplSvc.GetReleaseNaming(), svcDataSet.TmplSvc.ProductName, prod.Namespace, prod.EnvName, svcDataSet.TmplSvc.ServiceName)
			respObj.ChartRepo = svcDataSet.TmplSvc.HelmChart.Repo
			respObj.Chart = svcDataSet.TmplSvc.HelmChart.Name
			respObj.ChartVersion = svcDataSet.TmplSvc.HelmChart.Version
			respObj.AppVersion = svcDataSet.TmplSvc.HelmChart.Version
		} else if svcDataSet.ProdSvc != nil {
			// chart deploy
			respObj.ReleaseName = svcDataSet.ProdSvc.ReleaseName
		}

		if svcDataSet.ProdSvc.FromZadig() {
			if chartInfo, ok := chartInfoMap[respObj.ReleaseName]; ok {
				respObj.OverrideYaml = chartInfo.GetOverrideYaml()
				respObj.OverrideValues = chartInfo.OverrideValues
			}
		} else {
			if chartDeployInfo, ok := chartDeployInfoMap[respObj.ReleaseName]; ok {
				respObj.ChartRepo = chartDeployInfo.ChartRepo
				respObj.OverrideYaml = chartDeployInfo.GetOverrideYaml()
				respObj.OverrideValues = chartDeployInfo.OverrideValues
			}
			respObj.IsHelmChartDeploy = true
			respObj.DeployStrategy = prod.ServiceDeployStrategy[commonutil.GetReleaseDeployStrategyKey(svcDataSet.ProdSvc.ReleaseName)]
		}

		if svcDataSet.ProdSvc.Error != "" {
			respObj.Status = HelmReleaseStatusFailed
		}

		ret = append(ret, respObj)
	}

	return ret, nil
}

func loadChartFilesInfo(productName, serviceName string, revision int64, dir string) ([]*types.FileInfo, error) {
	base := config.LocalTestServicePathWithRevision(productName, serviceName, fmt.Sprint(revision))

	var fis []*types.FileInfo
	files, err := os.ReadDir(filepath.Join(base, serviceName, dir))
	if err != nil {
		log.Warnf("failed to read chart info for service %s with revision %d", serviceName, revision)
		base = config.LocalTestServicePath(productName, serviceName)
		files, err = os.ReadDir(filepath.Join(base, serviceName, dir))
		if err != nil {
			return nil, err
		}
	}

	for _, file := range files {
		info, _ := file.Info()
		if info == nil {
			continue
		}
		fi := &types.FileInfo{
			Parent:  dir,
			Name:    file.Name(),
			Size:    info.Size(),
			Mode:    file.Type(),
			ModTime: info.ModTime().Unix(),
			IsDir:   file.IsDir(),
		}

		fis = append(fis, fi)
	}
	return fis, nil
}

// prepare chart version data
func prepareChartVersionData(prod *models.Product, serviceObj *models.Service) error {
	productName := prod.ProductName
	serviceName, revision := serviceObj.ServiceName, serviceObj.Revision
	base := config.LocalTestServicePathWithRevision(productName, serviceName, fmt.Sprint(revision))
	if err := commonutil.PreloadServiceManifestsByRevision(base, serviceObj, prod.Production); err != nil {
		log.Warnf("failed to get chart of revision: %d for service: %s, use latest version", revision, serviceName)
		// use the latest version when it fails to download the specific version
		base = config.LocalTestServicePath(productName, serviceName)
		if err = commonutil.PreLoadServiceManifests(base, serviceObj, prod.Production); err != nil {
			log.Errorf("failed to load chart info for service %v", serviceObj.ServiceName)
			return err
		}
	}

	fullPath := filepath.Join(base, serviceObj.ServiceName)
	deliveryChartPath := filepath.Join(config.LocalDeliveryChartPathWithRevision(productName, serviceObj.ServiceName, serviceObj.Revision), serviceObj.ServiceName)
	err := copy.Copy(fullPath, deliveryChartPath)
	if err != nil {
		return err
	}

	helmClient, err := helmtool.NewClientFromNamespace(prod.ClusterID, prod.Namespace)
	if err != nil {
		log.Errorf("[%s][%s] init helm client error: %s", prod.EnvName, productName, err)
		return err
	}

	releaseName := util.GeneReleaseName(serviceObj.GetReleaseNaming(), prod.ProductName, prod.Namespace, prod.EnvName, serviceObj.ServiceName)
	valuesMap, err := helmClient.GetReleaseValues(releaseName, true)
	if err != nil {
		log.Errorf("failed to get values map data, err: %s", err)
		return err
	}

	currentValuesYaml, err := yaml.Marshal(valuesMap)
	if err != nil {
		return err
	}

	// write values.yaml
	if err = os.WriteFile(filepath.Join(deliveryChartPath, setting.ValuesYaml), currentValuesYaml, 0644); err != nil {
		return err
	}

	return nil
}

func GetChartInfos(productName, envName, serviceName string, production bool, log *zap.SugaredLogger) (*HelmChartsResp, error) {
	opt := &commonrepo.ProductFindOptions{Name: productName, EnvName: envName, Production: &production}
	prod, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		return nil, e.ErrGetHelmCharts.AddErr(err)
	}

	allServiceMap := prod.GetServiceMap()
	serviceMap := make(map[string]*models.ProductService)

	//validate data, make sure service and chart info exists
	if len(serviceName) > 0 {
		serviceList := strings.Split(serviceName, ",")
		for _, singleService := range serviceList {
			if service, ok := allServiceMap[singleService]; ok {
				serviceMap[service.ServiceName] = service
			} else {
				return nil, e.ErrGetHelmCharts.AddDesc(fmt.Sprintf("failed to find service %s in target namespace", singleService))
			}
		}
	} else {
		serviceMap = allServiceMap
	}

	if len(serviceMap) == 0 {
		return nil, nil
	}

	ret := &HelmChartsResp{
		ChartInfos: make([]*ChartInfo, 0),
		FileInfos:  make([]*types.FileInfo, 0),
	}

	errList := new(multierror.Error)
	wg := sync.WaitGroup{}

	for _, service := range serviceMap {
		ret.ChartInfos = append(ret.ChartInfos, &ChartInfo{
			ServiceName: service.ServiceName,
			Revision:    service.Revision,
		})
		wg.Add(1)
		// download chart info with particular version
		go func(serviceName string, revision int64) {
			defer wg.Done()
			serviceObj, err := repository.QueryTemplateService(&commonrepo.ServiceFindOption{
				ProductName: productName,
				ServiceName: serviceName,
				Revision:    revision,
				Type:        setting.HelmDeployType,
			}, prod.Production)
			if err != nil {
				log.Errorf("failed to query services name: %s, revision: %d, error: %s", serviceName, revision, err)
				errList = multierror.Append(errList, fmt.Errorf("failed to query service, serviceName: %s, revision: %d", serviceName, revision))
				return
			}
			err = prepareChartVersionData(prod, serviceObj)
			if err != nil {
				errList = multierror.Append(errList, fmt.Errorf("failed to prepare chart info for service %s", serviceObj.ServiceName))
				return
			}
		}(service.ServiceName, service.Revision)
	}
	wg.Wait()

	if errList.ErrorOrNil() != nil {
		return nil, errList.ErrorOrNil()
	}

	// expand file info for first service
	serviceToExpand := ret.ChartInfos[0].ServiceName
	fis, err := loadChartFilesInfo(productName, serviceToExpand, serviceMap[serviceToExpand].Revision, "")
	if err != nil {
		log.Errorf("Failed to load service file info, err: %s", err)
		return nil, e.ErrListTemplate.AddErr(err)
	}
	ret.FileInfos = fis

	return ret, nil
}

func GetImageInfos(productName, envName, serviceNames string, production bool, log *zap.SugaredLogger) (*ChartImagesResp, error) {
	opt := &commonrepo.ProductFindOptions{Name: productName, EnvName: envName, Production: &production}
	prod, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		return nil, fmt.Errorf("failed to find product: %s:%s to get image infos, err: %s", productName, envName, err)
	}

	helmClient, err := helmtool.NewClientFromNamespace(prod.ClusterID, prod.Namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to init kube client: %s:%s to get image infos, err: %s", productName, envName, err)
	}

	// filter releases, only list releases deployed by zadig
	serviceMap := prod.GetServiceMap()
	templateSvcs, err := commonutil.GetProductUsedTemplateSvcs(prod)
	if err != nil {
		return nil, fmt.Errorf("failed to get service tempaltes,  err: %s", err)
	}
	templateSvcMap := make(map[string]*models.Service)
	for _, ts := range templateSvcs {
		templateSvcMap[ts.ServiceName] = ts
	}
	services := strings.Split(serviceNames, ",")

	ret := &ChartImagesResp{}

	for _, svcName := range services {
		prodSvc, ok := serviceMap[svcName]
		if !ok || prodSvc == nil {
			return nil, fmt.Errorf("failed to find service: %s in product", svcName)
		}

		ts, ok := templateSvcMap[svcName]
		if !ok {
			return nil, fmt.Errorf("failed to find template service: %s", svcName)
		}

		releaseName := util.GeneReleaseName(ts.GetReleaseNaming(), productName, prod.Namespace, prod.EnvName, svcName)
		valuesYaml, err := helmClient.GetReleaseValues(releaseName, true)
		if err != nil {
			return nil, fmt.Errorf("failed to get values for relase: %s, err: %s", releaseName, err)
		}

		flatMap, err := converter.Flatten(valuesYaml)
		if err != nil {
			return nil, fmt.Errorf("failed to get flat map url for release :%s", releaseName)
		}

		svcImage := &ServiceImages{
			ServiceName: svcName,
			Images:      nil,
		}

		allModules, err := commonrepo.NewBuildColl().List(&commonrepo.BuildListOption{ProductName: productName})
		if err != nil {
			return nil, fmt.Errorf("failed to list builds for project: %s, err: %s", productName, err)
		}

		containerNameSet := sets.NewString()
		for _, build := range allModules {
			for _, target := range build.Targets {
				if target.ServiceName == svcName {
					containerNameSet.Insert(target.ServiceModule)
				}
			}
		}

		for _, container := range prodSvc.Containers {
			if container.ImagePath == nil {
				return nil, fmt.Errorf("failed to parse image for container:%s", container.Image)
			}

			imageSearchRule := &template.ImageSearchingRule{
				Repo:      container.ImagePath.Repo,
				Namespace: container.ImagePath.Namespace,
				Image:     container.ImagePath.Image,
				Tag:       container.ImagePath.Tag,
			}
			pattern := imageSearchRule.GetSearchingPattern()
			imageUrl, err := commonutil.GeneImageURI(pattern, flatMap)
			if err != nil {
				return nil, fmt.Errorf("failed to get image url for container:%s", container.Image)
			}

			svcImage.Images = append(svcImage.Images, &ImageData{
				imageUrl,
				util.GetImageNameFromContainerInfo(container.ImageName, container.Name),
				commonservice.ExtractImageTag(imageUrl),
				containerNameSet.Has(container.ImageName),
			})
		}
		ret.ServiceImages = append(ret.ServiceImages, svcImage)
	}
	return ret, nil
}
