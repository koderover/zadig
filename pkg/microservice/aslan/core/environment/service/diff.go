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
	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
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

// GetServiceDiff 获得服务模板当前版本和最新版本的对比
func GetServiceDiff(envName, productName, serviceName string, log *zap.SugaredLogger) (*SvcDiffResult, error) {
	resp := new(SvcDiffResult)
	opt := &commonrepo.ProductFindOptions{Name: productName, EnvName: envName}
	productInfo, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		log.Errorf("[%s][%s]find current configmaps error: %v", envName, productName, err)
		return resp, e.ErrFindProduct.AddDesc("查找product失败")
	}
	var serviceInfo *commonmodels.ProductService
	for _, serviceGroup := range productInfo.Services {
		for _, service := range serviceGroup {
			if service.ServiceName == serviceName {
				serviceInfo = service
			}
		}
	}
	svcOpt := &commonrepo.ServiceFindOption{
		ServiceName:   serviceName,
		ProductName:   serviceInfo.ProductName,
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
		oldRender, err = commonservice.GetRenderSet(productInfo.Render.Name, productInfo.Render.Revision, false, envName, log)
		if err != nil {
			return resp, err
		}
		newRender, err = commonservice.GetRenderSet(productInfo.Render.Name, 0, false, envName, log)
		if err != nil {
			return resp, err
		}
	}
	//resp.Current.Yaml = commonservice.RenderValueForString(oldService.Yaml, oldRender)

	resp.Current.Yaml, err = kube.RenderServiceYaml(oldService.Yaml, "", "", oldRender, oldService.ServiceVars, oldService.VariableYaml)
	if err != nil {
		log.Error("failed to RenderServiceYaml, err: %s", err)
		return nil, err
	}

	resp.Current.Revision = oldService.Revision
	resp.Current.UpdateBy = oldService.CreateBy

	resp.Latest.Yaml, err = kube.RenderServiceYaml(newService.Yaml, "", "", newRender, newService.ServiceVars, newService.VariableYaml)
	if err != nil {
		log.Error("failed to RenderServiceYaml, err: %s", err)
		return nil, err
	}

	//resp.Latest.Yaml = commonservice.RenderValueForString(newService.Yaml, newRender)
	resp.Latest.Revision = newService.Revision
	resp.Latest.UpdateBy = newService.CreateBy
	return resp, nil
}
