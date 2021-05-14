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
	"sort"

	commonmodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	commonrepo "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	commonservice "github.com/koderover/zadig/lib/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/lib/setting"
	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/tool/xlog"
)

type SvcDiffResult struct {
	Current TmplYaml `json:"current,omitempty"`
	Latest  TmplYaml `json:"latest,omitempty"`
}

type ConfigDiffResult struct {
	Current TmplConfig `json:"current,omitempty"`
	Latest  TmplConfig `json:"latest,omitempty"`
}

type TmplYaml struct {
	Yaml     string `json:"yaml,omitempty"`
	UpdateBy string `json:"update_by,omitempty"`
	Revision int64  `json:"revision,omitempty"`
}

type TmplConfig struct {
	Data []ConfigTmplData `json:"data,omitempty"`
}

type ConfigTmplData struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// ConfigDiff 获得服务模板当前版本和最新版本的对比
func ConfigDiff(envName, productName, serviceName, configName string, log *xlog.Logger) (*ConfigDiffResult, error) {
	resp := &ConfigDiffResult{}
	opt := &commonrepo.ProductFindOptions{Name: productName, EnvName: envName}
	productInfo, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		log.Errorf("[%s][%s]find current configmaps error: %v", envName, productName, err)
		return resp, e.ErrFindProduct.AddDesc("查找product失败")
	}
	configInfo := &commonmodels.ServiceConfig{}
	for _, serviceGroup := range productInfo.Services {
		for _, service := range serviceGroup {
			if service.ServiceName == serviceName {
				for _, config := range service.Configs {
					if config.ConfigName == configName {
						configInfo = config
					}
				}
			}
		}
	}
	configOpt := &commonrepo.ConfigFindOption{
		ConfigName:  configName,
		ServiceName: serviceName,
		Revision:    configInfo.Revision,
	}
	oldConfig, err := commonrepo.NewConfigColl().Find(configOpt)
	if err != nil {
		log.Errorf("[%s][%s][%s]find config template [%d] error: %v", envName, productName, serviceName, configInfo.Revision, err)
		return resp, e.ErrGetConfigMap.AddDesc("查找config template失败")
	}
	configOpt.Revision = 0
	newConfig, err := commonrepo.NewConfigColl().Find(configOpt)
	if err != nil {
		log.Errorf("[%s][%s][%s]find max revision config template error: %v", envName, productName, serviceName, err)
		return resp, e.ErrGetConfigMap.AddDesc("查找config template失败")
	}
	oldRender := &commonmodels.RenderSet{}
	newRender := &commonmodels.RenderSet{}
	if productInfo.Render != nil {
		oldRender, err = commonservice.GetRenderSet(productInfo.Render.Name, productInfo.Render.Revision, log)
		if err != nil {
			return resp, err
		}
		newRender, err = commonservice.GetRenderSet(productInfo.Render.Name, 0, log)
		if err != nil {
			return resp, err
		}
	}
	resp.Current = getConfigTmplData(oldConfig, oldRender)
	resp.Latest = getConfigTmplData(newConfig, newRender)
	return resp, nil
}

// ServiceDiff 获得服务模板当前版本和最新版本的对比
func ServiceDiff(envName, productName, serviceName string, log *xlog.Logger) (*SvcDiffResult, error) {
	resp := new(SvcDiffResult)
	opt := &commonrepo.ProductFindOptions{Name: productName, EnvName: envName}
	productInfo, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		log.Errorf("[%s][%s]find current configmaps error: %v", envName, productName, err)
		return resp, e.ErrFindProduct.AddDesc("查找product失败")
	}
	serviceInfo := &commonmodels.ProductService{}
	for _, serviceGroup := range productInfo.Services {
		for _, service := range serviceGroup {
			if service.ServiceName == serviceName {
				serviceInfo = service
			}
		}
	}
	svcOpt := &commonrepo.ServiceFindOption{
		ServiceName:   serviceName,
		Revision:      serviceInfo.Revision,
		ExcludeStatus: setting.ProductStatusDeleting,
	}
	oldService, err := commonrepo.NewServiceColl().Find(svcOpt)
	if err != nil {
		log.Errorf("[%s][%s][%s]find config template [%d] error: %v", envName, productName, serviceName, serviceInfo.Revision, err)
		return resp, e.ErrGetService.AddDesc("查找service template失败")
	}
	svcOpt.Revision = 0
	newService, err := commonrepo.NewServiceColl().Find(svcOpt)
	if err != nil {
		log.Errorf("[%s][%s][%s]find max revision config template error: %v", envName, productName, serviceName, err)
		return resp, e.ErrGetService.AddDesc("查找service template失败")
	}
	oldRender := &commonmodels.RenderSet{}
	newRender := &commonmodels.RenderSet{}
	if productInfo.Render != nil {
		oldRender, err = commonservice.GetRenderSet(productInfo.Render.Name, productInfo.Render.Revision, log)
		if err != nil {
			return resp, err
		}
		newRender, err = commonservice.GetRenderSet(productInfo.Render.Name, 0, log)
		if err != nil {
			return resp, err
		}
	}
	resp.Current.Yaml = commonservice.RenderValueForString(oldService.Yaml, oldRender)
	resp.Current.Revision = oldService.Revision
	resp.Current.UpdateBy = oldService.CreateBy
	resp.Latest.Yaml = commonservice.RenderValueForString(newService.Yaml, newRender)
	resp.Latest.Revision = newService.Revision
	resp.Latest.UpdateBy = newService.CreateBy
	return resp, nil
}

func getConfigTmplData(config *commonmodels.Config, render *commonmodels.RenderSet) TmplConfig {
	resp := TmplConfig{}
	var keys []string
	configData := make(map[string]string, 0)
	for _, data := range config.ConfigData {
		keys = append(keys, data.Key)
		if _, ok := configData[data.Key]; !ok {
			configData[data.Key] = data.Value
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		renderedValue := commonservice.RenderValueForString(configData[key], render)
		resp.Data = append(resp.Data, ConfigTmplData{Key: key, Value: renderedValue})
	}
	return resp
}
