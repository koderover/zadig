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

	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/yaml"

	commonmodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	commonrepo "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	templaterepo "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo/template"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/service/codehost"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/service/poetry"
	"github.com/koderover/zadig/lib/setting"
	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/tool/xlog"
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
	ProductName string                    `json:"product_name"`
	ServiceName string                    `json:"service_name"`
	Visibility  string                    `json:"visibility"`
	Revision    int64                     `json:"revision"`
	Type        string                    `json:"type"`
	Username    string                    `json:"username"`
	EnvConfigs  []*commonmodels.EnvConfig `json:"env_configs"`
	EnvStatuses []*commonmodels.EnvStatus `json:"env_statuses,omitempty"`
	From        string                    `json:"from,omitempty"`
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
	BranchName       string                    `json:"branch_name"`
	LoadPath         string                    `json:"load_path"`
	LoadFromDir      bool                      `json:"is_dir"`
	GerritRemoteName string                    `json:"gerrit_remote_name,omitempty"`
}

// ListServiceTemplate 列出服务模板
// 如果team == ""，则列出所有
func ListServiceTemplate(productName string, log *xlog.Logger) (*ServiceTmplResp, error) {
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
				&codehost.CodeHostOption{CodeHostType: poetry.GitHubProvider, Address: address, Namespace: owner})
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

func GetServiceTemplate(serviceName, serviceType, productName, excludeStatus string, revision int64, log *xlog.Logger) (*commonmodels.Service, error) {
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
			} else {
				errMsg := fmt.Sprintf("[ServiceTmpl.Find]  ListCodehostDetail %s error: %v", serviceName, err)
				log.Error(errMsg)
			}
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
			&codehost.CodeHostOption{CodeHostType: poetry.GitHubProvider, Address: address, Namespace: owner})
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
