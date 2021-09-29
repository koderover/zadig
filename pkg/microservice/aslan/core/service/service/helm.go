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
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/27149chen/afero"
	"github.com/hashicorp/go-multierror"
	"github.com/otiai10/copy"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/yaml"

	configbase "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	templatemodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	fsservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/fs"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/templatestore/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/types"
	yamlutil "github.com/koderover/zadig/pkg/util/yaml"
)

type HelmService struct {
	Services  []*models.Service `json:"services"`
	FileInfos []*types.FileInfo `json:"file_infos"`
}

type HelmServiceReq struct {
	ProductName string   `json:"product_name"`
	CreateBy    string   `json:"create_by"`
	CodehostID  int      `json:"codehost_id"`
	RepoOwner   string   `json:"repo_owner"`
	RepoName    string   `json:"repo_name"`
	BranchName  string   `json:"branch_name"`
	FilePaths   []string `json:"file_paths"`
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

type helmServiceCreationArgs struct {
	ServiceName string
	FilePath    string
	ProductName string
	CreateBy    string
	CodehostID  int
	Owner       string
	Repo        string
	Branch      string
	RepoLink    string
}

func ListHelmServices(productName string, log *zap.SugaredLogger) (*HelmService, error) {
	helmService := &HelmService{
		Services:  []*models.Service{},
		FileInfos: []*types.FileInfo{},
	}

	opt := &commonrepo.ServiceListOption{
		ProductName: productName,
		Type:        setting.HelmDeployType,
	}

	services, err := commonrepo.NewServiceColl().ListMaxRevisions(opt)
	if err != nil {
		log.Errorf("[helmService.list] err:%v", err)
		return nil, e.ErrListTemplate.AddErr(err)
	}
	helmService.Services = services

	if len(services) > 0 {
		fis, err := loadServiceFileInfos(services[0].ProductName, services[0].ServiceName, "")
		if err != nil {
			log.Errorf("Failed to load service file info, err: %s", err)
			return nil, e.ErrListTemplate.AddErr(err)
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

func GetFilePath(serviceName, productName, dir string, _ *zap.SugaredLogger) ([]*types.FileInfo, error) {
	return loadServiceFileInfos(productName, serviceName, dir)
}

func GetFileContent(serviceName, productName, filePath, fileName string, log *zap.SugaredLogger) (string, error) {
	base := config.LocalServicePath(productName, serviceName)

	svc, err := commonrepo.NewServiceColl().Find(&commonrepo.ServiceFindOption{
		ProductName: productName,
		ServiceName: serviceName,
	})
	if err != nil {
		return "", e.ErrFileContent.AddDesc(err.Error())
	}

	err = commonservice.PreLoadServiceManifests(base, svc)
	if err != nil {
		return "", e.ErrFileContent.AddDesc(err.Error())
	}

	file := filepath.Join(base, serviceName, filePath, fileName)
	fileContent, err := os.ReadFile(file)
	if err != nil {
		log.Errorf("Failed to read file %s, err: %s", file, err)
		return "", e.ErrFileContent.AddDesc(err.Error())
	}

	return string(fileContent), nil
}

func CreateOrUpdateHelmService(projectName string, args *HelmServiceCreationArgs, logger *zap.SugaredLogger) error {
	switch args.Source {
	case LoadFromRepo, LoadFromPublicRepo:
		return CreateOrUpdateHelmServiceFromGitRepo(projectName, args, logger)
	case LoadFromChartTemplate:
		return CreateOrUpdateHelmServiceFromChartTemplate(projectName, args, logger)
	default:
		return fmt.Errorf("invalid source")
	}
}

func CreateOrUpdateHelmServiceFromChartTemplate(projectName string, args *HelmServiceCreationArgs, logger *zap.SugaredLogger) error {
	templateArgs, ok := args.CreateFrom.(*CreateFromChartTemplate)
	if !ok {
		return fmt.Errorf("invalid argument")
	}

	// get chart template from local disk
	chart, err := mongodb.NewChartColl().Get(templateArgs.TemplateName)
	if err != nil {
		logger.Errorf("Failed to get chart template %s, err: %s", templateArgs.TemplateName, err)
		return err
	}

	localBase := configbase.LocalChartTemplatePath(templateArgs.TemplateName)
	s3Base := configbase.ObjectStorageChartTemplatePath(templateArgs.TemplateName)
	if err = fsservice.PreloadFiles(templateArgs.TemplateName, localBase, s3Base, logger); err != nil {
		return err
	}

	// deal with values, values will come from template, git repo and user defined one,
	// if one key exists in more than one values source, the latter one will override the former one.
	var values [][]byte
	base := filepath.Base(chart.Path)
	defaultValuesFile := filepath.Join(localBase, base, setting.ValuesYaml)
	defaultValues, _ := os.ReadFile(defaultValuesFile)
	if len(defaultValues) > 0 {
		values = append(values, defaultValues)
	}
	for _, path := range templateArgs.ValuesPaths {
		v, err := fsservice.DownloadFileFromSource(&fsservice.DownloadFromSourceArgs{
			CodehostID: templateArgs.CodehostID,
			Owner:      templateArgs.Owner,
			Repo:       templateArgs.Repo,
			Path:       path,
			Branch:     templateArgs.Branch,
		})
		if err != nil {
			return err
		}

		values = append(values, v)
	}

	if len(templateArgs.ValuesYAML) > 0 {
		values = append(values, []byte(templateArgs.ValuesYAML))
	}

	// copy template to service path and update the values.yaml
	from := filepath.Join(localBase, base)
	to := filepath.Join(config.LocalServicePath(projectName, args.Name), args.Name)
	if err = copy.Copy(from, to); err != nil {
		logger.Errorf("Failed to copy file from %s to %s, err: %s", from, to, err)
		return err
	}

	merged, err := yamlutil.Merge(values)
	if err != nil {
		logger.Errorf("Failed to merge values, err: %s", err)
		return err
	}

	if err = os.WriteFile(filepath.Join(to, setting.ValuesYaml), merged, 0644); err != nil {
		logger.Errorf("Failed to write values, err: %s", err)
		return err
	}

	fsTree := os.DirFS(config.LocalServicePath(projectName, args.Name))
	ServiceS3Base := config.ObjectStorageServicePath(projectName, args.Name)
	if err = fsservice.ArchiveAndUploadFilesToS3(fsTree, args.Name, ServiceS3Base, logger); err != nil {
		logger.Errorf("Failed to upload files for service %s in project %s, err: %s", args.Name, projectName, err)
		return err
	}

	svc, err := createOrUpdateHelmService(
		fsTree,
		&helmServiceCreationArgs{
			ServiceName: args.Name,
			FilePath:    to,
			ProductName: projectName,
			CreateBy:    args.CreatedBy,
			CodehostID:  templateArgs.CodehostID,
			Owner:       templateArgs.Owner,
			Repo:        templateArgs.Repo,
			Branch:      templateArgs.Branch,
		},
		logger,
	)
	if err != nil {
		logger.Errorf("Failed to create service %s in project %s, error: %s", args.Name, projectName, err)
		return err
	}

	if svc.HelmChart != nil {
		compareHelmVariable([]*templatemodels.RenderChart{
			{ServiceName: args.Name,
				ChartVersion: svc.HelmChart.Version,
				ValuesYaml:   svc.HelmChart.ValuesYaml,
			},
		}, projectName, args.CreatedBy, logger)
	}

	return nil
}

func CreateOrUpdateHelmServiceFromGitRepo(projectName string, args *HelmServiceCreationArgs, log *zap.SugaredLogger) error {
	var err error
	var repoLink string
	repoArgs, ok := args.CreateFrom.(*CreateFromRepo)
	if !ok {
		publicArgs, ok := args.CreateFrom.(*CreateFromPublicRepo)
		if !ok {
			return fmt.Errorf("invalid argument")
		}

		repoArgs, err = PublicRepoToPrivateRepoArgs(publicArgs)
		if err != nil {
			log.Errorf("Failed to parse repo args %+v, err: %s", publicArgs, err)
			return err
		}

		repoLink = publicArgs.RepoLink
	}
	helmRenderCharts := make([]*templatemodels.RenderChart, 0, len(repoArgs.Paths))
	var errs *multierror.Error

	var wg wait.Group
	var mux sync.RWMutex
	for _, p := range repoArgs.Paths {
		filePath := strings.TrimLeft(p, "/")
		wg.Start(func() {
			var finalErr error

			defer func() {
				if finalErr != nil {
					mux.Lock()
					errs = multierror.Append(errs, finalErr)
					mux.Unlock()
				}
			}()

			log.Infof("Loading chart under path %s", filePath)

			var serviceName string
			fsTree, err := fsservice.DownloadFilesFromSource(
				&fsservice.DownloadFromSourceArgs{CodehostID: repoArgs.CodehostID, Owner: repoArgs.Owner, Repo: repoArgs.Repo, Path: filePath, Branch: repoArgs.Branch, RepoLink: repoLink},
				func(chartTree afero.Fs) (string, error) {
					chartName, _, err := readChartYAML(afero.NewIOFS(chartTree), filepath.Base(filePath), log)
					serviceName = chartName
					return chartName, err
				})
			if err != nil {
				log.Errorf("Failed to download files from source, err %s", err)
				finalErr = e.ErrCreateTemplate.AddErr(err)
				return
			}

			log.Info("Found valid chart, Starting to save and upload files")

			// save files to disk and upload them to s3
			if err = commonservice.SaveAndUploadService(projectName, serviceName, fsTree); err != nil {
				log.Errorf("Failed to save or upload files for service %s in project %s, error: %s", serviceName, projectName, err)
				finalErr = e.ErrCreateTemplate.AddErr(err)
				return
			}

			svc, err := createOrUpdateHelmService(
				fsTree,
				&helmServiceCreationArgs{
					ServiceName: serviceName,
					FilePath:    filePath,
					ProductName: projectName,
					CreateBy:    args.CreatedBy,
					CodehostID:  repoArgs.CodehostID,
					Owner:       repoArgs.Owner,
					Repo:        repoArgs.Repo,
					Branch:      repoArgs.Branch,
					RepoLink:    repoLink,
				},
				log,
			)
			if err != nil {
				log.Errorf("Failed to create service %s in project %s, error: %s", serviceName, projectName, err)
				finalErr = e.ErrCreateTemplate.AddErr(err)
				return
			}

			if svc.HelmChart != nil {
				helmRenderCharts = append(helmRenderCharts, &templatemodels.RenderChart{
					ServiceName:  serviceName,
					ChartVersion: svc.HelmChart.Version,
					ValuesYaml:   svc.HelmChart.ValuesYaml,
				})
			}
		})
	}

	wg.Wait()

	compareHelmVariable(helmRenderCharts, projectName, args.CreatedBy, log)

	return errs.ErrorOrNil()
}

func readChartYAML(chartTree fs.FS, base string, logger *zap.SugaredLogger) (string, string, error) {
	chartFile, err := fs.ReadFile(chartTree, filepath.Join(base, setting.ChartYaml))
	if err != nil {
		logger.Errorf("Failed to read %s, err: %s", setting.ChartYaml, err)
		return "", "", err
	}
	chart := new(Chart)
	if err = yaml.Unmarshal(chartFile, chart); err != nil {
		log.Errorf("Failed to unmarshal yaml %s, err: %s", setting.ChartYaml, err)
		return "", "", err
	}

	return chart.Name, chart.Version, nil
}

func readValuesYAML(chartTree fs.FS, base string, logger *zap.SugaredLogger) ([]byte, map[string]interface{}, error) {
	content, err := fs.ReadFile(chartTree, filepath.Join(base, setting.ValuesYaml))
	if err != nil {
		logger.Errorf("Failed to read %s, err: %s", setting.ValuesYaml, err)
		return nil, nil, err
	}

	valuesMap := make(map[string]interface{})
	if err = yaml.Unmarshal(content, &valuesMap); err != nil {
		logger.Errorf("Failed to unmarshal yaml %s, err: %s", setting.ValuesYaml, err)
		return nil, nil, err
	}

	return content, valuesMap, nil
}

func createOrUpdateHelmService(fsTree fs.FS, args *helmServiceCreationArgs, logger *zap.SugaredLogger) (*models.Service, error) {
	chartName, chartVersion, err := readChartYAML(fsTree, args.ServiceName, logger)
	if err != nil {
		logger.Errorf("Failed to read chart.yaml, err %s", err)
		return nil, err
	}

	values, valuesMap, err := readValuesYAML(fsTree, args.ServiceName, logger)
	if err != nil {
		logger.Errorf("Failed to read values.yaml, err %s", err)
		return nil, err
	}
	valuesYaml := string(values)

	serviceTemplate := fmt.Sprintf(setting.ServiceTemplateCounterName, args.ServiceName, args.ProductName)
	rev, err := commonrepo.NewCounterColl().GetNextSeq(serviceTemplate)
	if err != nil {
		logger.Errorf("Failed to get next revision for service %s, err: %s", args.ServiceName, err)
		return nil, err
	}
	if err = commonrepo.NewServiceColl().Delete(args.ServiceName, setting.HelmDeployType, args.ProductName, setting.ProductStatusDeleting, rev); err != nil {
		logger.Warnf("Failed to delete stale service %s with revision %d, err: %s", args.ServiceName, rev, err)
	}

	containerList, err := commonservice.ParseImagesForProductService(valuesMap, args.ServiceName, args.ProductName)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse service from yaml")
	}

	serviceObj := &models.Service{
		ServiceName: args.ServiceName,
		Type:        setting.HelmDeployType,
		Revision:    rev,
		ProductName: args.ProductName,
		Visibility:  setting.PrivateVisibility,
		CreateTime:  time.Now().Unix(),
		CreateBy:    args.CreateBy,
		Containers:  containerList,
		CodehostID:  args.CodehostID,
		RepoOwner:   args.Owner,
		RepoName:    args.Repo,
		BranchName:  args.Branch,
		LoadPath:    args.FilePath,
		SrcPath:     args.RepoLink,
		HelmChart: &models.HelmChart{
			Name:       chartName,
			Version:    chartVersion,
			ValuesYaml: valuesYaml,
		},
	}

	log.Infof("Starting to create service %s with revision %d", args.ServiceName, rev)

	if err = commonrepo.NewServiceColl().Create(serviceObj); err != nil {
		log.Errorf("Failed to create service %s error: %s", args.ServiceName, err)
		return nil, err
	}

	if err = templaterepo.NewProductColl().AddService(args.ProductName, args.ServiceName); err != nil {
		log.Errorf("Failed to add service %s to project %s, err: %s", args.ProductName, args.ServiceName, err)
		return nil, err
	}

	return serviceObj, nil
}

func loadServiceFileInfos(productName, serviceName, dir string) ([]*types.FileInfo, error) {
	base := config.LocalServicePath(productName, serviceName)

	svc, err := commonrepo.NewServiceColl().Find(&commonrepo.ServiceFindOption{
		ProductName: productName,
		ServiceName: serviceName,
	})
	if err != nil {
		return nil, e.ErrFilePath.AddDesc(err.Error())
	}

	err = commonservice.PreLoadServiceManifests(base, svc)
	if err != nil {
		return nil, e.ErrFilePath.AddDesc(err.Error())
	}
	var fis []*types.FileInfo
	files, err := os.ReadDir(filepath.Join(base, serviceName, dir))
	if err != nil {
		return nil, e.ErrFilePath.AddDesc(err.Error())
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

// UpdateHelmService TODO need to be deprecated
func UpdateHelmService(args *HelmServiceArgs, log *zap.SugaredLogger) error {
	var serviceNames []string
	for _, helmServiceInfo := range args.HelmServiceInfos {
		serviceNames = append(serviceNames, helmServiceInfo.ServiceName)

		opt := &commonrepo.ServiceFindOption{
			ProductName: args.ProductName,
			ServiceName: helmServiceInfo.ServiceName,
			Type:        setting.HelmDeployType,
		}
		preServiceTmpl, err := commonrepo.NewServiceColl().Find(opt)
		if err != nil {
			return e.ErrUpdateTemplate.AddDesc(err.Error())
		}

		base := config.LocalServicePath(args.ProductName, helmServiceInfo.ServiceName)
		if err = commonservice.PreLoadServiceManifests(base, preServiceTmpl); err != nil {
			return e.ErrUpdateTemplate.AddDesc(err.Error())
		}

		filePath := filepath.Join(base, helmServiceInfo.ServiceName, helmServiceInfo.FilePath, helmServiceInfo.FileName)
		if err = os.WriteFile(filePath, []byte(helmServiceInfo.FileContent), 0644); err != nil {
			log.Errorf("Failed to write file %s, err: %s", filePath, err)
			return e.ErrUpdateTemplate.AddDesc(err.Error())
		}

		// TODO：use yaml compare instead of just comparing the characters
		if helmServiceInfo.FileName == setting.ValuesYaml && preServiceTmpl.HelmChart.ValuesYaml != helmServiceInfo.FileContent {
			var valuesMap map[string]interface{}
			if err = yaml.Unmarshal([]byte(helmServiceInfo.FileContent), &valuesMap); err != nil {
				return e.ErrCreateTemplate.AddDesc("values.yaml解析失败")
			}

			containerList, err := commonservice.ParseImagesForProductService(valuesMap, preServiceTmpl.ServiceName, preServiceTmpl.ProductName)
			if err != nil {
				return e.ErrUpdateTemplate.AddErr(errors.Wrapf(err, "failed to parse images from yaml"))
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
		serviceTemplate := fmt.Sprintf(setting.ServiceTemplateCounterName, helmServiceInfo.ServiceName, preServiceTmpl.ProductName)
		rev, err := commonrepo.NewCounterColl().GetNextSeq(serviceTemplate)
		if err != nil {
			return fmt.Errorf("get next helm service revision error: %v", err)
		}

		preServiceTmpl.Revision = rev
		if err := commonrepo.NewServiceColl().Delete(helmServiceInfo.ServiceName, setting.HelmDeployType, args.ProductName, setting.ProductStatusDeleting, preServiceTmpl.Revision); err != nil {
			log.Errorf("helmService.update delete %s error: %v", helmServiceInfo.ServiceName, err)
		}

		if err := commonrepo.NewServiceColl().Create(preServiceTmpl); err != nil {
			log.Errorf("helmService.update serviceName:%s error:%v", helmServiceInfo.ServiceName, err)
			return e.ErrUpdateTemplate.AddDesc(err.Error())
		}
	}

	for _, serviceName := range serviceNames {
		s3Base := config.ObjectStorageServicePath(args.ProductName, serviceName)
		if err := fsservice.ArchiveAndUploadFilesToS3(os.DirFS(config.LocalServicePath(args.ProductName, serviceName)), serviceName, s3Base, log); err != nil {
			return e.ErrUpdateTemplate.AddDesc(err.Error())
		}
	}

	return nil
}

// compareHelmVariable 比较helm变量是否有改动，是否需要添加新的renderSet
func compareHelmVariable(chartInfos []*templatemodels.RenderChart, productName, createdBy string, log *zap.SugaredLogger) {
	// 对比上个版本的renderset，新增一个版本
	latestChartInfos := make([]*templatemodels.RenderChart, 0)
	renderOpt := &commonrepo.RenderSetFindOption{Name: productName}
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
			Name:        productName,
			Revision:    0,
			ProductTmpl: productName,
			UpdateBy:    createdBy,
			ChartInfos:  mixtureChartInfos,
		}, log,
	); err != nil {
		log.Errorf("helmService.Create CreateHelmRenderSet error: %v", err)
	}
}
