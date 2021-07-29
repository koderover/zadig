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
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/yaml"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	templatemodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/gerrit"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/kube"
	s3service "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/s3"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/helmclient"
	"github.com/koderover/zadig/pkg/tool/log"
	s3tool "github.com/koderover/zadig/pkg/tool/s3"
	"github.com/koderover/zadig/pkg/types"
	"github.com/koderover/zadig/pkg/util"
)

type HelmService struct {
	Services  []*models.Service `json:"services"`
	FileInfos []*types.FileInfo `json:"file_infos"`
}

type HelmServiceReq struct {
	ProductName string   `json:"product_name"`
	Visibility  string   `json:"visibility"`
	Type        string   `json:"type"`
	CreateBy    string   `json:"create_by"`
	CodehostID  int      `json:"codehost_id"`
	RepoOwner   string   `json:"repo_owner"`
	RepoName    string   `json:"repo_name"`
	BranchName  string   `json:"branch_name"`
	FilePaths   []string `json:"file_paths"`
	Revision    int64    `json:"revision"`
	SrcPath     string   `json:"src_path"`
}

type HelmServiceArgs struct {
	ProductName      string             `json:"product_name"`
	CreateBy         string             `json:"create_by"`
	HelmServiceInfos []*HelmServiceInfo `json:"helm_service_infos"`
}

type HelmServiceInfo struct {
	ServiceName string `json:"service_name"`
	FilePath    string `json:"file_path"`
	FileName    string `json:"file_name"`
	FileContent string `json:"file_content"`
}

type HelmServiceModule struct {
	ServiceModules []*ServiceModule `json:"service_modules"`
	Service        *models.Service  `json:"service,omitempty"`
}

type Chart struct {
	APIVersion string `json:"apiVersion"`
	Name       string `json:"name"`
	Version    string `json:"version"`
	AppVersion string `json:"appVersion"`
}

func ListHelmServices(productName string, log *zap.SugaredLogger) (*HelmService, error) {
	helmService := &HelmService{
		Services:  []*models.Service{},
		FileInfos: []*types.FileInfo{},
	}

	opt := &commonrepo.ServiceFindOption{
		ProductName:   productName,
		Type:          setting.HelmDeployType,
		ExcludeStatus: setting.ProductStatusDeleting,
	}

	services, err := commonrepo.NewServiceColl().List(opt)
	if err != nil {
		log.Errorf("[helmService.list] err:%v", err)
		return nil, e.ErrListTemplate.AddErr(err)
	}
	helmService.Services = services

	if len(services) > 0 {
		chartFilePath := services[0].LoadPath
		base, err := gerrit.GetGerritWorkspaceBasePath(services[0].RepoName)
		_, serviceFileErr := os.Stat(path.Join(base, chartFilePath))
		if err != nil || os.IsNotExist(serviceFileErr) {
			if err = commonservice.DownloadService(base, services[0].ServiceName); err != nil {
				return helmService, e.ErrListTemplate.AddErr(err)
			}
		}

		files, err := ioutil.ReadDir(path.Join(base, chartFilePath))
		if err != nil {
			return nil, e.ErrListTemplate.AddDesc(err.Error())
		}

		fis := make([]*types.FileInfo, 0)
		for _, file := range files {
			if file.Name() == ".git" && file.IsDir() {
				continue
			}
			fi := &types.FileInfo{
				Parent:  "/",
				Name:    file.Name(),
				Size:    file.Size(),
				Mode:    file.Mode(),
				ModTime: file.ModTime().Unix(),
				IsDir:   file.IsDir(),
			}
			fis = append(fis, fi)
		}
		helmService.FileInfos = fis
	}
	return helmService, nil
}

func GetHelmServiceModule(serviceName, productName string, revision int64, log *zap.SugaredLogger) (*HelmServiceModule, error) {
	serviceTemplate, err := commonservice.GetServiceTemplate(serviceName, setting.HelmDeployType, productName, setting.ProductStatusDeleting, revision, log)
	if err != nil {
		return nil, err
	}
	helmServiceModule := new(HelmServiceModule)
	serviceModules := make([]*ServiceModule, 0)
	for _, container := range serviceTemplate.Containers {
		serviceModule := new(ServiceModule)
		serviceModule.Container = container
		buildObj, _ := commonrepo.NewBuildColl().Find(&commonrepo.BuildFindOption{ProductName: productName, ServiceName: serviceName, Targets: []string{container.Name}})
		if buildObj != nil {
			serviceModule.BuildName = buildObj.Name
		}
		serviceModules = append(serviceModules, serviceModule)
	}
	helmServiceModule.Service = serviceTemplate
	helmServiceModule.ServiceModules = serviceModules
	return helmServiceModule, err
}

func GetFilePath(serviceName, productName, dir string, log *zap.SugaredLogger) ([]*types.FileInfo, error) {
	if dir == "" {
		dir = "/"
	}
	serviceTemplate, err := commonservice.GetServiceTemplate(serviceName, setting.HelmDeployType, productName, setting.ProductStatusDeleting, 0, log)
	if err != nil {
		return nil, err
	}
	fis := make([]*types.FileInfo, 0)
	base, err := gerrit.GetGerritWorkspaceBasePath(serviceTemplate.RepoName)
	_, serviceFileErr := os.Stat(path.Join(base, serviceTemplate.LoadPath))
	if err != nil || os.IsNotExist(serviceFileErr) {
		if err = commonservice.DownloadService(base, serviceTemplate.ServiceName); err != nil {
			return fis, e.ErrFilePath.AddDesc(err.Error())
		}
	}
	files, err := ioutil.ReadDir(path.Join(base, serviceTemplate.LoadPath, dir))
	if err != nil {
		return fis, e.ErrFilePath.AddDesc(err.Error())
	}

	for _, file := range files {
		if file.Name() == ".git" && file.IsDir() {
			continue
		}
		fi := &types.FileInfo{
			Parent:  dir,
			Name:    file.Name(),
			Size:    file.Size(),
			Mode:    file.Mode(),
			ModTime: file.ModTime().Unix(),
			IsDir:   file.IsDir(),
		}

		fis = append(fis, fi)
	}
	return fis, nil
}

func GetFileContent(serviceName, productName, filePath, fileName string, log *zap.SugaredLogger) (string, error) {
	serviceTemplate, err := commonservice.GetServiceTemplate(serviceName, setting.HelmDeployType, productName, setting.ProductStatusDeleting, 0, log)
	if err != nil {
		return "", e.ErrFileContent.AddDesc(err.Error())
	}
	base, err := gerrit.GetGerritWorkspaceBasePath(serviceTemplate.RepoName)
	_, serviceFileErr := os.Stat(path.Join(base, serviceTemplate.LoadPath))
	if err != nil || os.IsNotExist(serviceFileErr) {
		if err = commonservice.DownloadService(base, serviceTemplate.ServiceName); err != nil {
			return "", e.ErrFileContent.AddDesc(err.Error())
		}
	}

	fileContent, err := ioutil.ReadFile(path.Join(base, serviceTemplate.LoadPath, filePath, fileName))
	if err != nil {
		return "", e.ErrFileContent.AddDesc(err.Error())
	}

	return string(fileContent), nil
}

func CreateHelmService(args *HelmServiceReq, log *zap.SugaredLogger) error {
	args.Type = setting.HelmDeployType
	args.Visibility = setting.PrivateVisibility
	helmRenderCharts := make([]*templatemodels.RenderChart, 0, len(args.FilePaths))
	for _, filePath := range args.FilePaths {
		base, err := gerrit.GetGerritWorkspaceBasePath(args.RepoName)
		if err != nil {
			log.Errorf("GetGerritWorkspaceBasePath err:%v", err)
			return e.ErrCreateTemplate.AddErr(err)
		}
		files, err := ioutil.ReadDir(path.Join(base, filePath))
		if err != nil {
			log.Errorf("ReadDir err:%v", err)
			return e.ErrCreateTemplate.AddErr(err)
		}
		var containChartYaml, containValuesYaml, containTemplates bool
		var serviceName, valuesYaml, chartVersion string
		var valuesMap map[string]interface{}
		for _, file := range files {
			if file.Name() == setting.ChartYaml {
				yamlFile, err := ioutil.ReadFile(path.Join(base, filePath, setting.ChartYaml))
				if err != nil {
					return e.ErrCreateTemplate.AddDesc(fmt.Sprintf("读取%s失败", setting.ChartYaml))
				}
				chart := new(Chart)
				if err = yaml.Unmarshal(yamlFile, chart); err != nil {
					return e.ErrCreateTemplate.AddDesc(fmt.Sprintf("解析%s失败", setting.ChartYaml))
				}
				serviceName = chart.Name
				chartVersion = chart.Version
				containChartYaml = true
			} else if file.Name() == setting.ValuesYaml {
				yamlFileContent, err := ioutil.ReadFile(path.Join(base, filePath, setting.ValuesYaml))
				if err != nil {
					return e.ErrCreateTemplate.AddDesc(fmt.Sprintf("读取%s失败", setting.ValuesYaml))
				}

				if err = yaml.Unmarshal(yamlFileContent, &valuesMap); err != nil {
					return e.ErrCreateTemplate.AddDesc(fmt.Sprintf("解析%s失败", setting.ValuesYaml))
				}

				valuesYaml = string(yamlFileContent)
				containValuesYaml = true
			} else if file.Name() == setting.TemplatesDir {
				containTemplates = true
			}
		}
		if !containChartYaml || !containValuesYaml || !containTemplates {
			return e.ErrCreateTemplate.AddDesc(fmt.Sprintf("%s不是合法的chart目录,目录中必须包含%s/%s/%s目录等请检查!", filePath, setting.ValuesYaml, setting.ChartYaml, setting.TemplatesDir))
		}

		helmRenderCharts = append(helmRenderCharts, &templatemodels.RenderChart{
			ServiceName:  serviceName,
			ChartVersion: chartVersion,
			ValuesYaml:   valuesYaml,
		})

		opt := &commonrepo.ServiceFindOption{
			ServiceName:   serviceName,
			ExcludeStatus: setting.ProductStatusDeleting,
		}
		serviceTmpl, notFoundErr := commonrepo.NewServiceColl().Find(opt)
		if notFoundErr == nil {
			if serviceTmpl.ProductName != args.ProductName {
				return e.ErrInvalidParam.AddDesc(fmt.Sprintf("项目 [%s] %s", serviceTmpl.ProductName, "有相同的服务名称存在,请检查!"))
			}
		}

		serviceTemplate := fmt.Sprintf(setting.ServiceTemplateCounterName, serviceName, args.Type)
		rev, err := commonrepo.NewCounterColl().GetNextSeq(serviceTemplate)
		if err != nil {
			return fmt.Errorf("helmService.create get next helm service revision error: %v", err)
		}
		args.Revision = rev
		if err := commonrepo.NewServiceColl().Delete(serviceName, args.Type, "", setting.ProductStatusDeleting, args.Revision); err != nil {
			log.Errorf("helmService.create delete %s error: %v", serviceName, err)
		}
		containerList := recursionGetImage(valuesMap)
		if len(containerList) == 0 {
			_, containerList = recursionGetImageByColon(valuesMap)
		}
		serviceObj := &models.Service{
			ServiceName: serviceName,
			Type:        args.Type,
			Revision:    args.Revision,
			ProductName: args.ProductName,
			Visibility:  args.Visibility,
			CreateTime:  time.Now().Unix(),
			CreateBy:    args.CreateBy,
			Containers:  containerList,
			CodehostID:  args.CodehostID,
			RepoOwner:   args.RepoOwner,
			RepoName:    args.RepoName,
			BranchName:  args.BranchName,
			LoadPath:    filePath,
			SrcPath:     args.SrcPath,
			HelmChart: &models.HelmChart{
				Name:       serviceName,
				Version:    chartVersion,
				ValuesYaml: valuesYaml,
			},
		}

		// exec lint and dry run
		if err = helmChartDryRun(chartVersion, valuesYaml, serviceName, path.Join(base, filePath), log); err != nil {
			return e.ErrHelmDryRunFailed.AddDesc(fmt.Sprintf("具体错误信息: %s", err.Error()))
		}

		if err := commonrepo.NewServiceColl().Create(serviceObj); err != nil {
			log.Errorf("helmService.Create serviceName:%s error:%v", serviceName, err)
			return e.ErrCreateTemplate.AddDesc(err.Error())
		}

		if err = uploadService(base, serviceName, filePath); err != nil {
			return e.ErrCreateTemplate.AddDesc(err.Error())
		}

		if notFoundErr != nil {
			if productTempl, err := commonservice.GetProductTemplate(args.ProductName, log); err == nil {
				if len(productTempl.Services) > 0 && !sets.NewString(productTempl.Services[0]...).Has(serviceName) {
					productTempl.Services[0] = append(productTempl.Services[0], serviceName)
				} else {
					productTempl.Services = [][]string{{serviceName}}
				}
				//更新项目模板
				err = templaterepo.NewProductColl().Update(args.ProductName, productTempl)
				if err != nil {
					log.Errorf("helmService.Create Update productTmpl error: %v", err)
					return e.ErrCreateTemplate.AddDesc(err.Error())
				}
			}
		}
	}
	go func() {
		compareHelmVariable(helmRenderCharts, args, log)
	}()

	return nil
}

func helmChartDryRun(chartVersion, valuesYaml, serviceName, chartPath string, log *zap.SugaredLogger) error {
	restConfig, err := kube.GetRESTConfig("")
	if err != nil {
		log.Errorf("GetRESTConfig err: %s", err)
		return err
	}
	helmClient, err := helmclient.NewClientFromRestConf(restConfig, config.Namespace())
	if err != nil {
		log.Errorf("NewClientFromRestConf err: %s", err)
		return err
	}
	// Add a random to exclude resource conflicts caused by the same name
	name := fmt.Sprintf("%s-%s-%s", config.Namespace(), serviceName, util.GetRandomString(6))
	chartSpec := &helmclient.ChartSpec{
		ReleaseName: name,
		ChartName:   name,
		Namespace:   config.Namespace(),
		Version:     chartVersion,
		ValuesYaml:  valuesYaml,
		DryRun:      true,
	}

	err = helmClient.InstallOrUpgradeChart(context.Background(), chartSpec,
		&helmclient.ChartOption{ChartPath: chartPath}, log)

	return err
}

func UpdateHelmService(args *HelmServiceArgs, log *zap.SugaredLogger) error {
	serviceNames := sets.NewString()
	modifyServices := make([]*models.Service, 0)
	for _, helmServiceInfo := range args.HelmServiceInfos {
		opt := &commonrepo.ServiceFindOption{
			ProductName:   args.ProductName,
			ServiceName:   helmServiceInfo.ServiceName,
			Type:          setting.HelmDeployType,
			ExcludeStatus: setting.ProductStatusDeleting,
		}

		preServiceTmpl, err := commonrepo.NewServiceColl().Find(opt)
		if err != nil {
			return e.ErrUpdateTemplate.AddDesc(err.Error())
		}

		if !serviceNames.Has(helmServiceInfo.ServiceName) {
			serviceNames.Insert(helmServiceInfo.ServiceName)
			modifyServices = append(modifyServices, preServiceTmpl)
		}

		base, err := gerrit.GetGerritWorkspaceBasePath(preServiceTmpl.RepoName)
		_, serviceFileErr := os.Stat(path.Join(base, preServiceTmpl.LoadPath))
		if err != nil || os.IsNotExist(serviceFileErr) {
			if err = commonservice.DownloadService(base, helmServiceInfo.ServiceName); err != nil {
				return e.ErrUpdateTemplate.AddDesc(err.Error())
			}
		}
		if err = ioutil.WriteFile(filepath.Join(base, preServiceTmpl.LoadPath, helmServiceInfo.FilePath, helmServiceInfo.FileName), []byte(helmServiceInfo.FileContent), 0644); err != nil {
			log.Errorf("WriteFile filepath:%s err:%v", filepath.Join(base, helmServiceInfo.ServiceName, helmServiceInfo.FilePath, helmServiceInfo.FileName), err)
			return e.ErrUpdateTemplate.AddDesc(err.Error())
		}
		if helmServiceInfo.FileName == setting.ValuesYaml && preServiceTmpl.HelmChart.ValuesYaml != helmServiceInfo.FileContent {
			var valuesMap map[string]interface{}
			if err = yaml.Unmarshal([]byte(helmServiceInfo.FileContent), &valuesMap); err != nil {
				return e.ErrCreateTemplate.AddDesc("values.yaml解析失败")
			}

			containerList := recursionGetImage(valuesMap)
			if len(containerList) == 0 {
				_, containerList = recursionGetImageByColon(valuesMap)
			}
			preServiceTmpl.Containers = containerList
			preServiceTmpl.HelmChart.ValuesYaml = helmServiceInfo.FileContent

			//修改helm renderset
			renderOpt := &commonrepo.RenderSetFindOption{Name: args.ProductName}
			if rs, err := commonrepo.NewRenderSetColl().Find(renderOpt); err == nil {
				for _, chartInfo := range rs.ChartInfos {
					if chartInfo.ServiceName == helmServiceInfo.ServiceName {
						chartInfo.ValuesYaml = helmServiceInfo.FileContent
						break
					}
				}
				if err = commonrepo.NewRenderSetColl().Update(rs); err != nil {
					log.Errorf("[renderset.update] err:%v", err)
				}
			}
		} else if helmServiceInfo.FileName == setting.ChartYaml {
			chart := new(Chart)
			if err = yaml.Unmarshal([]byte(helmServiceInfo.FileContent), chart); err != nil {
				return e.ErrCreateTemplate.AddDesc(fmt.Sprintf("解析%s失败", setting.ChartYaml))
			}
			if preServiceTmpl.HelmChart.Version != chart.Version {
				preServiceTmpl.HelmChart.Version = chart.Version

				//修改helm renderset
				renderOpt := &commonrepo.RenderSetFindOption{Name: args.ProductName}
				if rs, err := commonrepo.NewRenderSetColl().Find(renderOpt); err == nil {
					for _, chartInfo := range rs.ChartInfos {
						if chartInfo.ServiceName == helmServiceInfo.ServiceName {
							chartInfo.ChartVersion = chart.Version
							break
						}
					}
					if err = commonrepo.NewRenderSetColl().Update(rs); err != nil {
						log.Errorf("[renderset.update] err:%v", err)
					}
				}
			}
		}

		preServiceTmpl.CreateBy = args.CreateBy
		serviceTemplate := fmt.Sprintf(setting.ServiceTemplateCounterName, helmServiceInfo.ServiceName, setting.HelmDeployType)
		rev, err := commonrepo.NewCounterColl().GetNextSeq(serviceTemplate)
		if err != nil {
			return fmt.Errorf("get next helm service revision error: %v", err)
		}

		preServiceTmpl.Revision = rev
		if err := commonrepo.NewServiceColl().Delete(helmServiceInfo.ServiceName, setting.HelmDeployType, "", setting.ProductStatusDeleting, preServiceTmpl.Revision); err != nil {
			log.Errorf("helmService.update delete %s error: %v", helmServiceInfo.ServiceName, err)
		}

		if err := commonrepo.NewServiceColl().Create(preServiceTmpl); err != nil {
			log.Errorf("helmService.update serviceName:%s error:%v", helmServiceInfo.ServiceName, err)
			return e.ErrUpdateTemplate.AddDesc(err.Error())
		}
	}

	for _, serviceObj := range modifyServices {
		base, _ := gerrit.GetGerritWorkspaceBasePath(serviceObj.RepoName)
		if err := uploadService(base, serviceObj.ServiceName, serviceObj.LoadPath); err != nil {
			return e.ErrUpdateTemplate.AddDesc(err.Error())
		}
	}

	return nil
}

// compareHelmVariable 比较helm变量是否有改动，是否需要添加新的renderSet
func compareHelmVariable(chartInfos []*templatemodels.RenderChart, args *HelmServiceReq, log *zap.SugaredLogger) {
	// 对比上个版本的renderset，新增一个版本
	latestChartInfos := make([]*templatemodels.RenderChart, 0)
	renderOpt := &commonrepo.RenderSetFindOption{Name: args.ProductName}
	if latestDefaultRenderSet, err := commonrepo.NewRenderSetColl().Find(renderOpt); err == nil {
		latestChartInfos = latestDefaultRenderSet.ChartInfos
	}

	currentChartInfoMap := make(map[string]*templatemodels.RenderChart)
	for _, chartInfo := range chartInfos {
		currentChartInfoMap[chartInfo.ServiceName] = chartInfo
	}

	mixtureChartInfos := make([]*templatemodels.RenderChart, 0)
	for _, latestChartInfo := range latestChartInfos {
		//如果新的里面存在就拿新的数据替换，不存在就还使用老的数据
		if currentChartInfo, isExist := currentChartInfoMap[latestChartInfo.ServiceName]; isExist {
			mixtureChartInfos = append(mixtureChartInfos, currentChartInfo)
			delete(currentChartInfoMap, latestChartInfo.ServiceName)
			continue
		}
		mixtureChartInfos = append(mixtureChartInfos, latestChartInfo)
	}

	//把新增的服务添加到新的slice里面
	for _, chartInfo := range currentChartInfoMap {
		mixtureChartInfos = append(mixtureChartInfos, chartInfo)
	}

	//添加renderset
	if err := commonservice.CreateHelmRenderSet(
		&models.RenderSet{
			Name:        args.ProductName,
			Revision:    0,
			ProductTmpl: args.ProductName,
			UpdateBy:    args.CreateBy,
			ChartInfos:  mixtureChartInfos,
		}, log,
	); err != nil {
		log.Errorf("helmService.Create CreateHelmRenderSet error: %v", err)
	}
}

// 递归通过repository和tag获取服务组件
func recursionGetImage(jsonValues map[string]interface{}) []*models.Container {
	ret := make([]*models.Container, 0)
	for jsonKey, jsonValue := range jsonValues {
		if levelMap, ok := jsonValue.(map[string]interface{}); ok {
			ret = append(ret, recursionGetImage(levelMap)...)
		} else if repository, isStr := jsonValue.(string); isStr {
			if strings.Contains(jsonKey, "repository") {
				serviceContainer := new(models.Container)
				if imageTag, isExist := jsonValues["tag"]; isExist {
					if imageTag != "" {
						serviceContainer.Image = fmt.Sprintf("%s:%v", repository, imageTag)
						imageStr := strings.Split(repository, "/")
						if len(imageStr) > 1 {
							serviceContainer.Name = imageStr[len(imageStr)-1]
						}
						ret = append(ret, serviceContainer)
					}
				}
			}
		}
	}
	return ret
}

func recursionGetImageByColon(jsonValues map[string]interface{}) ([]string, []*models.Container) {
	imageRegEx := regexp.MustCompile(config.ImageRegexString)
	ret := make([]*models.Container, 0)
	banList := sets.NewString()

	for _, jsonValue := range jsonValues {
		if levelMap, ok := jsonValue.(map[string]interface{}); ok {
			imageList, recursiveRet := recursionGetImageByColon(levelMap)
			ret = append(ret, recursiveRet...)
			banList.Insert(imageList...)
		} else if imageName, isStr := jsonValue.(string); isStr {
			if strings.Contains(imageName, ":") && imageRegEx.MatchString(imageName) &&
				!strings.Contains(imageName, "http") && !strings.Contains(imageName, "https") {
				serviceContainer := new(models.Container)
				serviceContainer.Image = imageName
				imageArr := strings.Split(imageName, "/")
				if len(imageArr) >= 1 {
					imageTagArr := strings.Split(imageArr[len(imageArr)-1], ":")
					serviceContainer.Name = imageTagArr[0]
				}
				if !banList.Has(imageName) {
					banList.Insert(imageName)
					ret = append(ret, serviceContainer)
				}
			}
		}
	}
	return banList.List(), ret
}

func uploadService(base, serviceName, filePath string) error {
	//将当前文件目录压缩上传到s3
	tarFilePath := path.Join(base, fmt.Sprintf("%s.tar.gz", serviceName))
	if err := util.Tar(path.Join(base, filePath), tarFilePath); err != nil {
		log.Errorf("%s目录压缩失败", path.Join(base, filePath))
		return err
	}
	s3Storage, err := s3service.FindDefaultS3()
	if err != nil {
		log.Errorf("获取默认的s3配置失败 err:%v", err)
		return err
	}
	forcedPathStyle := false
	if s3Storage.Provider == setting.ProviderSourceSystemDefault {
		forcedPathStyle = true
	}
	client, err := s3tool.NewClient(s3Storage.Endpoint, s3Storage.Ak, s3Storage.Sk, s3Storage.Insecure, forcedPathStyle)
	if err != nil {
		log.Errorf("Failed to get s3 client, error is: %+v", err)
		return err
	}
	subFolderName := serviceName + "-" + setting.HelmDeployType
	if s3Storage.Subfolder != "" {
		s3Storage.Subfolder = fmt.Sprintf("%s/%s/%s", s3Storage.Subfolder, subFolderName, "service")
	} else {
		s3Storage.Subfolder = fmt.Sprintf("%s/%s", subFolderName, "service")
	}
	objectKey := s3Storage.GetObjectPath(fmt.Sprintf("%s.tar.gz", serviceName))
	if err = client.Upload(s3Storage.Bucket, tarFilePath, objectKey); err != nil {
		log.Errorf("s3上传文件失败 err:%v", err)
		return err
	}
	if err = os.Remove(tarFilePath); err != nil {
		log.Errorf("remove file err:%v", err)
	}
	return nil
}
