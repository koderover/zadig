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
	"strings"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/yaml"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/webhook"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/codehost"
	"github.com/koderover/zadig/pkg/shared/poetry"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

type yamlPreview struct {
	Kind string `json:"kind"`
}

type ServiceTmplResp struct {
	Data  []*ServiceProductMap `json:"data"`
	Total int                  `json:"total"`
}

type ServiceTmplBuildObject struct {
	ServiceTmplObject *ServiceTmplObject  `json:"pm_service_tmpl"`
	Build             *commonmodels.Build `json:"build"`
}

type ServiceTmplObject struct {
	ProductName  string                        `json:"product_name"`
	ServiceName  string                        `json:"service_name"`
	Visibility   string                        `json:"visibility"`
	Revision     int64                         `json:"revision"`
	Type         string                        `json:"type"`
	Username     string                        `json:"username"`
	EnvConfigs   []*commonmodels.EnvConfig     `json:"env_configs"`
	EnvStatuses  []*commonmodels.EnvStatus     `json:"env_statuses,omitempty"`
	From         string                        `json:"from,omitempty"`
	HealthChecks []*commonmodels.PmHealthCheck `json:"health_checks"`
}

type ServiceProductMap struct {
	Service          string                    `json:"service_name"`
	Source           string                    `json:"source"`
	Type             string                    `json:"type"`
	Product          []string                  `json:"product"`
	ProductName      string                    `json:"product_name"`
	Containers       []*commonmodels.Container `json:"containers,omitempty"`
	Visibility       string                    `json:"visibility,omitempty"`
	CodehostID       int                       `json:"codehost_id"`
	RepoOwner        string                    `json:"repo_owner"`
	RepoName         string                    `json:"repo_name"`
	RepoUUID         string                    `json:"repo_uuid"`
	BranchName       string                    `json:"branch_name"`
	LoadPath         string                    `json:"load_path"`
	LoadFromDir      bool                      `json:"is_dir"`
	GerritRemoteName string                    `json:"gerrit_remote_name,omitempty"`
}

// ListServiceTemplate 列出服务模板
// 如果team == ""，则列出所有
func ListServiceTemplate(productName string, log *zap.SugaredLogger) (*ServiceTmplResp, error) {
	var err error
	resp := new(ServiceTmplResp)
	resp.Data = make([]*ServiceProductMap, 0)
	serviceNames := sets.NewString()

	serviceTmpls, err := commonrepo.NewServiceColl().DistinctServices(&commonrepo.ServiceListOption{ProductName: productName, IsSort: true, ExcludeStatus: setting.ProductStatusDeleting})
	if err != nil {
		log.Errorf("ServiceTmpl.ListServices error: %v", err)
		return resp, e.ErrListTemplate.AddDesc(err.Error())
	}
	for _, service := range serviceTmpls {
		serviceNames.Insert(service.ServiceName)
	}

	//把公共项目下的服务也添加到服务列表里面
	productTmpl, _ := templaterepo.NewProductColl().Find(productName)
	for _, services := range productTmpl.Services {
		for _, service := range services {
			if serviceNames.Has(service) {
				continue
			}
			services, err := commonrepo.NewServiceColl().DistinctServices(&commonrepo.ServiceListOption{ServiceName: service, ExcludeStatus: setting.ProductStatusDeleting})
			if err != nil {
				log.Errorf("ServiceTmpl.ListServices error: %v", err)
				return resp, e.ErrListTemplate.AddDesc(err.Error())
			}
			serviceTmpls = append(serviceTmpls, services...)
		}
	}

	productSvcs, err := distincProductServices("")
	if err != nil {
		log.Errorf("distinctProductServices error: %v", err)
		return resp, e.ErrListTemplate
	}

	for _, service := range serviceTmpls {
		//获取服务的containers
		opt := &commonrepo.ServiceFindOption{
			ServiceName: service.ServiceName,
			Type:        service.Type,
			Revision:    service.Revision,
			ProductName: service.ProductName,
		}
		serviceObject, err := commonrepo.NewServiceColl().Find(opt)
		if err != nil {
			log.Errorf("ServiceTmpl Find error: %v", err)
			continue
		}

		// FIXME: 兼容老数据，想办法干掉这个
		if serviceObject.Source == setting.SourceFromGitlab && serviceObject.CodehostID == 0 {
			gitlabAddress, err := GetGitlabAddress(serviceObject.SrcPath)
			if err != nil {
				log.Errorf("无法从原有数据中恢复加载信息, GetGitlabAddr failed err: %+v", err)
				return nil, e.ErrListTemplate.AddDesc(err.Error())
			}

			details, err := codehost.ListCodehostDetial()
			if err != nil {
				log.Errorf("无法从原有数据中恢复加载信息, listCodehostDetail failed err: %+v", err)
				return nil, e.ErrListTemplate.AddDesc(err.Error())
			}
			for _, detail := range details {
				if strings.Contains(detail.Address, gitlabAddress) {
					serviceObject.CodehostID = detail.ID
				}
			}
			_, owner, r, branch, loadPath, _, err := GetOwnerRepoBranchPath(serviceObject.SrcPath)
			if err != nil {
				log.Errorf("Failed to load info from url: %s, the error is: %+v", serviceObject.SrcPath, err)
				return nil, e.ErrListTemplate.AddDesc(fmt.Sprintf("Failed to load info from url: %s, the error is: %+v", serviceObject.SrcPath, err))
			}
			// 万一codehost被删了，找不到
			if serviceObject.CodehostID == 0 {
				log.Errorf("Failed to find the old code host info")
				return nil, e.ErrListTemplate.AddDesc("无法找到原有的codehost信息，请确认codehost仍然存在")
			}
			serviceObject.RepoOwner = owner
			serviceObject.RepoName = r
			serviceObject.BranchName = branch
			serviceObject.LoadPath = loadPath
			serviceObject.LoadFromDir = true
		} else if serviceObject.Source == setting.SourceFromGithub && serviceObject.RepoName == "" {
			address, owner, r, branch, loadPath, _, err := GetOwnerRepoBranchPath(serviceObject.SrcPath)
			if err != nil {
				return nil, err
			}

			detail, err := codehost.GetCodeHostInfo(
				&codehost.Option{CodeHostType: poetry.GitHubProvider, Address: address, Namespace: owner})
			if err != nil {
				log.Errorf("get github codeHostInfo failed, err:%v", err)
				return nil, err
			}
			serviceObject.CodehostID = detail.ID
			serviceObject.RepoOwner = owner
			serviceObject.RepoName = r
			serviceObject.BranchName = branch
			serviceObject.LoadPath = loadPath
			serviceObject.LoadFromDir = true
		}

		spmap := &ServiceProductMap{
			Service:          service.ServiceName,
			Type:             service.Type,
			Source:           service.Source,
			ProductName:      service.ProductName,
			Containers:       serviceObject.Containers,
			Product:          []string{},
			Visibility:       serviceObject.Visibility,
			CodehostID:       serviceObject.CodehostID,
			RepoOwner:        serviceObject.RepoOwner,
			RepoName:         serviceObject.RepoName,
			RepoUUID:         serviceObject.RepoUUID,
			BranchName:       serviceObject.BranchName,
			LoadFromDir:      serviceObject.LoadFromDir,
			LoadPath:         serviceObject.LoadPath,
			GerritRemoteName: service.GerritRemoteName,
		}

		if _, ok := productSvcs[service.ServiceName]; ok {
			spmap.Product = productSvcs[service.ServiceName]
		}

		resp.Data = append(resp.Data, spmap)
	}

	return resp, nil
}

func distincProductServices(productName string) (map[string][]string, error) {
	serviceMap := make(map[string][]string)
	products, err := templaterepo.NewProductColl().List(productName)
	if err != nil {
		return serviceMap, err
	}

	for _, product := range products {
		for _, group := range product.Services {
			for _, service := range group {
				if _, ok := serviceMap[service]; !ok {
					serviceMap[service] = []string{product.ProductName}
				} else {
					serviceMap[service] = append(serviceMap[service], product.ProductName)
				}
			}
		}
	}
	return serviceMap, nil
}

func GetServiceTemplate(serviceName, serviceType, productName, excludeStatus string, revision int64, log *zap.SugaredLogger) (*commonmodels.Service, error) {
	opt := &commonrepo.ServiceFindOption{
		ServiceName: serviceName,
		Type:        serviceType,
		Revision:    revision,
		ProductName: productName,
	}
	if excludeStatus != "" {
		opt.ExcludeStatus = excludeStatus
	}

	resp, err := commonrepo.NewServiceColl().Find(opt)
	if err != nil {
		errMsg := fmt.Sprintf("[ServiceTmpl.Find] %s error: %v", serviceName, err)
		log.Error(errMsg)
		return resp, e.ErrGetTemplate.AddDesc(errMsg)
	}

	if resp.Source == setting.SourceFromGitlab && resp.RepoName == "" {
		if gitlabAddress, err := GetGitlabAddress(resp.SrcPath); err == nil {
			if details, err := codehost.ListCodehostDetial(); err == nil {
				for _, detail := range details {
					if strings.Contains(detail.Address, gitlabAddress) {
						resp.GerritCodeHostID = detail.ID
						resp.CodehostID = detail.ID
					}
				}
				_, owner, r, branch, loadPath, pathType, err := GetOwnerRepoBranchPath(resp.SrcPath)
				if err != nil {
					log.Errorf("Failed to load info from url: %s, the error is: %+v", resp.SrcPath, err)
					return nil, e.ErrGetService.AddDesc(fmt.Sprintf("Failed to load info from url: %s, the error is: %+v", resp.SrcPath, err))
				}
				// 万一codehost被删了，找不到
				if resp.CodehostID == 0 {
					log.Errorf("Failed to find the old code host info")
					return nil, e.ErrListTemplate.AddDesc("无法找到原有的codehost信息，请确认codehost仍然存在")
				}
				resp.RepoOwner = owner
				resp.RepoName = r
				resp.BranchName = branch
				resp.LoadPath = loadPath
				resp.LoadFromDir = pathType == "tree"
				return resp, nil
			}
			errMsg := fmt.Sprintf("[ServiceTmpl.Find]  ListCodehostDetail %s error: %v", serviceName, err)
			log.Error(errMsg)
		} else {
			errMsg := fmt.Sprintf("[ServiceTmpl.Find]  GetGitlabAddress %s error: %v", serviceName, err)
			log.Error(errMsg)
		}

	} else if resp.Source == setting.SourceFromGithub && resp.GerritCodeHostID == 0 {
		address, owner, r, branch, loadPath, _, err := GetOwnerRepoBranchPath(resp.SrcPath)
		if err != nil {
			return nil, err
		}

		detail, err := codehost.GetCodeHostInfo(
			&codehost.Option{CodeHostType: poetry.GitHubProvider, Address: address, Namespace: owner})
		if err != nil {
			log.Errorf("get github codeHostInfo failed, err:%v", err)
			return nil, err
		}
		resp.CodehostID = detail.ID
		resp.RepoOwner = owner
		resp.RepoName = r
		resp.BranchName = branch
		resp.LoadPath = loadPath
		resp.LoadFromDir = true
		return resp, nil

	} else if resp.Source == setting.SourceFromGUI {
		yamls := strings.Split(resp.Yaml, "---")
		for _, y := range yamls {
			data, err := yaml.YAMLToJSON([]byte(y))
			if err != nil {
				log.Errorf("convert yaml to json failed, yaml:%s, err:%v", y, err)
				return nil, err
			}

			var result interface{}
			err = json.Unmarshal(data, &result)
			if err != nil {
				log.Errorf("unmarshal yaml data failed, yaml:%s, err:%v", y, err)
				return nil, err
			}

			yamlPreview := yamlPreview{}
			err = json.Unmarshal(data, &yamlPreview)
			if err != nil {
				log.Errorf("unmarshal yaml data failed, yaml:%s, err:%v", y, err)
				return nil, err
			}

			if resp.GUIConfig == nil {
				resp.GUIConfig = new(commonmodels.GUIConfig)
			}

			switch yamlPreview.Kind {
			case "Deployment":
				resp.GUIConfig.Deployment = result
			case "Ingress":
				resp.GUIConfig.Ingress = result
			case "Service":
				resp.GUIConfig.Service = result
			}
		}
	}

	return resp, nil
}

func UpdatePmServiceTemplate(username string, args *ServiceTmplBuildObject, log *zap.SugaredLogger) error {
	//该请求来自环境中的服务更新时，from=createEnv
	if args.ServiceTmplObject.From == "" {
		if err := UpdateBuild(username, args.Build, log); err != nil {
			return err
		}
	}

	//先比较healthcheck是否有变动
	preService, err := GetServiceTemplate(args.ServiceTmplObject.ServiceName, setting.PMDeployType, args.ServiceTmplObject.ProductName, setting.ProductStatusDeleting, args.ServiceTmplObject.Revision, log)
	if err != nil {
		return err
	}

	//更新服务
	serviceTemplate := fmt.Sprintf(setting.ServiceTemplateCounterName, preService.ServiceName, setting.PMDeployType)
	rev, err := commonrepo.NewCounterColl().GetNextSeq(serviceTemplate)
	if err != nil {
		return err
	}
	preService.HealthChecks = args.ServiceTmplObject.HealthChecks
	preService.EnvConfigs = args.ServiceTmplObject.EnvConfigs
	preService.Revision = rev
	preService.CreateBy = username
	preService.BuildName = args.Build.Name

	if err := commonrepo.NewServiceColl().Delete(preService.ServiceName, setting.PMDeployType, "", setting.ProductStatusDeleting, preService.Revision); err != nil {
		return err
	}

	if err := commonrepo.NewServiceColl().Create(preService); err != nil {
		return err
	}
	return nil
}

func DeleteServiceWebhookByName(serviceName string, logger *zap.SugaredLogger) {
	svc, err := commonrepo.NewServiceColl().Find(&commonrepo.ServiceFindOption{ServiceName: serviceName, Type: setting.K8SDeployType})
	if err != nil {
		logger.Errorf("Failed to get service %s, error: %s", serviceName, err)
		return
	}
	ProcessServiceWebhook(nil, svc, serviceName, logger)
}

func ProcessServiceWebhook(updated, current *commonmodels.Service, serviceName string, logger *zap.SugaredLogger) {
	var action string
	var updatedHooks, currentHooks []*webhook.WebHook
	if updated != nil {
		if updated.Source == setting.SourceFromZadig || updated.Source == setting.SourceFromGerrit || updated.Source == "" {
			return
		}
		action = "add"
		address := getAddressFromPath(updated.SrcPath, updated.RepoOwner, updated.RepoName, logger.Desugar())
		if address == "" {
			return
		}
		updatedHooks = append(updatedHooks, &webhook.WebHook{Owner: updated.RepoOwner, Repo: updated.RepoName, Address: address, Name: "trigger", CodeHostID: updated.CodehostID})
	}
	if current != nil {
		if current.Source == setting.SourceFromZadig || current.Source == setting.SourceFromGerrit || current.Source == "" {
			return
		}
		action = "remove"
		address := getAddressFromPath(current.SrcPath, current.RepoOwner, current.RepoName, logger.Desugar())
		if address == "" {
			return
		}
		currentHooks = append(currentHooks, &webhook.WebHook{Owner: current.RepoOwner, Repo: current.RepoName, Address: address, Name: "trigger", CodeHostID: current.CodehostID})
	}
	if updated != nil && current != nil {
		action = "update"
	}

	logger.Debugf("Start to %s webhook for service %s", action, serviceName)
	err := ProcessWebhook(updatedHooks, currentHooks, webhook.ServicePrefix+serviceName, logger)
	if err != nil {
		logger.Errorf("Failed to process WebHook, error: %s", err)
	}

}

func getAddressFromPath(path, owner, repo string, logger *zap.Logger) string {
	res := strings.Split(path, fmt.Sprintf("/%s/%s/", owner, repo))
	if len(res) != 2 {
		logger.With(zap.String("path", path), zap.String("owner", owner), zap.String("repo", repo)).DPanic("Invalid path")
		return ""
	}

	return res[0]
}
