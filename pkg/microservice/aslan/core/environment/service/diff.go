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

	"go.uber.org/zap"

	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/repository"
	commontypes "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/types"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

type SvcDiffResult struct {
	ReleaseName       string   `json:"release_name"`
	ServiceName       string   `json:"service_name"`
	ChartName         string   `json:"chart_name"`
	DeployedFromChart bool     `json:"deployed_from_chart"`
	Current           TmplYaml `json:"current"`
	Latest            TmplYaml `json:"latest"`
	Error             string   `json:"error"`
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
	serviceInfo := productInfo.GetServiceMap()[serviceName]
	if serviceInfo == nil {
		return resp, fmt.Errorf("service %s not found in environment", serviceName)
	}

	svcOpt := &commonrepo.ServiceFindOption{
		ServiceName: serviceName,
		ProductName: serviceInfo.ProductName,
		Revision:    serviceInfo.Revision,
	}
	oldService, err := repository.QueryTemplateService(svcOpt, productInfo.Production)
	if err != nil {
		log.Errorf("[%s][%s][%s]find config template [%d] error: %v", envName, productName, serviceName, serviceInfo.Revision, err)
		return resp, e.ErrGetService.AddDesc("查找service template失败")
	}

	svcOpt.Revision = 0
	newService, err := repository.QueryTemplateService(svcOpt, productInfo.Production)
	if err != nil {
		log.Errorf("[%s][%s][%s]find max revision config template error: %v", envName, productName, serviceName, err)
		return resp, e.ErrGetService.AddDesc("查找service template失败")
	}

	svcRender := serviceInfo.GetServiceRender()

	resp.Current.Yaml, err = kube.RenderServiceYaml(oldService.Yaml, productName, serviceName, svcRender)
	if err != nil {
		log.Error("failed to RenderServiceYaml, err: %s", err)
		return nil, err
	}

	resp.Current.Revision = oldService.Revision
	resp.Current.UpdateBy = oldService.CreateBy

	mergedYaml, mergedServiceVariableKVs, err := commontypes.MergeRenderAndServiceTemplateVariableKVs(svcRender.OverrideYaml.RenderVariableKVs, newService.ServiceVariableKVs)
	if err != nil {
		return nil, fmt.Errorf("failed to merge render and service variable kvs serviceName: %s, error: %v", svcRender.ServiceName, err)
	}

	svcRender.OverrideYaml.YamlContent = mergedYaml
	svcRender.OverrideYaml.RenderVariableKVs = mergedServiceVariableKVs

	resp.Latest.Yaml, err = kube.RenderServiceYaml(newService.Yaml, productName, serviceName, svcRender)
	if err != nil {
		log.Error("failed to RenderServiceYaml, err: %s", err)
		return nil, err
	}

	resp.Latest.Revision = newService.Revision
	resp.Latest.UpdateBy = newService.CreateBy
	return resp, nil
}
