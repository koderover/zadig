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

package render

import (
	"errors"
	"fmt"
	"reflect"
	"sort"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	templatemodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

type KVPair struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
}

func listTmplRenderKeys(productTmplName string, log *zap.SugaredLogger) ([]*templatemodels.RenderKV, map[string]*templatemodels.ServiceInfo, error) {
	//如果没找到对应产品，则kv为空
	prodTmpl, err := template.NewProductColl().Find(productTmplName)
	if err != nil {
		errMsg := fmt.Sprintf("[ProductTmpl.Find] %s error: %v", productTmplName, err)
		log.Warn(errMsg)
		return nil, nil, nil
	}

	kvs, err := ListServicesRenderKeys(prodTmpl.AllServiceInfos(), log)
	return kvs, prodTmpl.AllServiceInfoMap(), err
}

func GetRenderSet(renderName string, revision int64, isDefault bool, envName string, log *zap.SugaredLogger) (*commonmodels.RenderSet, error) {
	// 未指定renderName返回空的renderSet
	if renderName == "" {
		return &commonmodels.RenderSet{}, nil
	}
	opt := &commonrepo.RenderSetFindOption{
		Name:      renderName,
		Revision:  revision,
		IsDefault: isDefault,
		EnvName:   envName,
	}
	resp, found, err := commonrepo.NewRenderSetColl().FindRenderSet(opt)
	if err != nil {
		return nil, err
	} else if !found {
		return &commonmodels.RenderSet{}, nil
	}

	return resp, nil
}

func GetRenderSetInfo(renderName string, revision int64) (*commonmodels.RenderSet, error) {
	opt := &commonrepo.RenderSetFindOption{
		Name:     renderName,
		Revision: revision,
	}
	resp, err := commonrepo.NewRenderSetColl().Find(opt)
	if err != nil {
		return resp, err
	}

	return resp, nil
}

// ValidateRenderSet 检查指定renderSet是否能覆盖产品所有需要渲染的值
func ValidateRenderSet(productName, renderName, envName string, serviceInfo *templatemodels.ServiceInfo, log *zap.SugaredLogger) (*commonmodels.RenderSet, error) {
	resp := &commonmodels.RenderSet{ProductTmpl: productName}
	var err error
	if renderName != "" {
		opt := &commonrepo.RenderSetFindOption{Name: renderName, ProductTmpl: productName, EnvName: envName}
		resp, err = commonrepo.NewRenderSetColl().Find(opt)
		if err != nil {
			log.Errorf("find renderset[%s] error: %v", renderName, err)
			return resp, err
		}
	}
	return resp, nil
}

func mergeServiceVariables(newVariables []*templatemodels.ServiceRender, oldVariables []*templatemodels.ServiceRender) []*templatemodels.ServiceRender {
	allVarMap := make(map[string]*templatemodels.ServiceRender)
	for _, sv := range oldVariables {
		allVarMap[sv.ServiceName] = sv
	}
	for _, sv := range newVariables {
		allVarMap[sv.ServiceName] = sv
	}
	ret := make([]*templatemodels.ServiceRender, 0)
	for _, sv := range allVarMap {
		ret = append(ret, sv)
	}
	return ret
}

func CreateRenderSetByMerge(args *commonmodels.RenderSet, log *zap.SugaredLogger) (*commonmodels.RenderSet, error) {
	opt := &commonrepo.RenderSetFindOption{Name: args.Name, ProductTmpl: args.ProductTmpl, EnvName: args.EnvName}
	rs, err := commonrepo.NewRenderSetColl().Find(opt)
	if rs != nil && err == nil {
		if rs.K8sServiceRenderDiff(args) {
			args.IsDefault = rs.IsDefault
		} else {
			args.Revision = rs.Revision
			return args, nil
		}
		args.ServiceVariables = mergeServiceVariables(args.ServiceVariables, rs.ServiceVariables)
	}
	err = createRenderset(args, log)
	return args, err
}

func CreateRenderSet(args *commonmodels.RenderSet, log *zap.SugaredLogger) error {
	return createRenderset(args, log)
}

func ForceCreateReaderSet(args *commonmodels.RenderSet, log *zap.SugaredLogger) error {
	return createRenderset(args, log)
}

// CreateK8sHelmRenderSet creates renderset for k8s/helm projects
func CreateK8sHelmRenderSet(args *commonmodels.RenderSet, log *zap.SugaredLogger) error {
	opt := &commonrepo.RenderSetFindOption{
		Name:        args.Name,
		ProductTmpl: args.ProductTmpl,
		EnvName:     args.EnvName,
	}
	rs, err := commonrepo.NewRenderSetColl().Find(opt)
	if rs != nil && err == nil {
		if rs.HelmRenderDiff(args) || !reflect.DeepEqual(rs.YamlData, args.YamlData) || rs.K8sServiceRenderDiff(args) || rs.Diff(args) {
			args.IsDefault = rs.IsDefault
		} else {
			args.Revision = rs.Revision
			return nil
		}
	}
	return ForceCreateReaderSet(args, log)
}

func CreateDefaultHelmRenderset(args *commonmodels.RenderSet, log *zap.SugaredLogger) error {
	args.IsDefault = true
	return createRenderset(args, log)
}

func createRenderset(args *commonmodels.RenderSet, log *zap.SugaredLogger) error {
	if err := ensureRenderSetArgs(args); err != nil {
		log.Error(err)
		return e.ErrCreateRenderSet.AddDesc(err.Error())
	}
	if err := commonrepo.NewRenderSetColl().Create(args); err != nil {
		errMsg := fmt.Sprintf("[RenderSet.Create] %s error: %v", args.Name, err)
		log.Error(errMsg)
		return e.ErrCreateRenderSet.AddDesc(errMsg)
	}
	return nil
}

func UpdateRenderSet(args *commonmodels.RenderSet, log *zap.SugaredLogger) error {
	err := commonrepo.NewRenderSetColl().Update(args)
	if err != nil {
		errMsg := fmt.Sprintf("[RenderSet.update] %s error: %+v", args.Name, err)
		log.Error(errMsg)
		return e.ErrUpdateRenderSet.AddDesc(errMsg)
	}

	return nil
}

func ListServicesRenderKeys(services []*templatemodels.ServiceInfo, log *zap.SugaredLogger) ([]*templatemodels.RenderKV, error) {
	renderSvcMap := make(map[string][]string)
	resp := make([]*templatemodels.RenderKV, 0)

	serviceTmpls, err := commonrepo.NewServiceColl().ListMaxRevisionsForServices(services, setting.K8SDeployType)
	if err != nil {
		errMsg := fmt.Sprintf("[serviceTmpl.ListMaxRevisions] error: %v", err)
		log.Error(errMsg)
		return resp, fmt.Errorf(errMsg)
	}

	for _, serviceTmpl := range serviceTmpls {
		findRenderAlias(serviceTmpl.ServiceName, serviceTmpl.Yaml, renderSvcMap)
	}

	for key, val := range renderSvcMap {
		rk := &templatemodels.RenderKV{
			Alias:    key,
			Services: val,
		}
		rk.SetKeys()
		rk.RemoveDupServices()

		resp = append(resp, rk)
	}

	sort.SliceStable(resp, func(i, j int) bool { return resp[i].Key < resp[j].Key })
	return resp, nil
}

func DeleteRenderSet(productName string, log *zap.SugaredLogger) error {
	if err := commonrepo.NewRenderSetColl().Delete(productName); err != nil {
		errMsg := fmt.Sprintf("[RenderSet.Delete] %s error: %v", productName, err)
		log.Error(errMsg)
		return e.ErrDeleteRenderSet.AddDesc(errMsg)
	}
	return nil
}

func ValidateKVs(kvs []*templatemodels.RenderKV, services []*templatemodels.ServiceInfo, log *zap.SugaredLogger) error {
	resp := make(map[string][]string)
	keys, err := ListServicesRenderKeys(services, log)
	if err != nil {
		return fmt.Errorf("service.ListServicesRenderKeys to list %v %v", services, err)
	}

	for _, key := range keys {
		resp[key.Key] = key.Services
	}

	kvMap := make(map[string]string)
	for _, kv := range kvs {
		kvMap[kv.Key] = kv.Value
	}

	for key := range resp {
		if _, ok := kvMap[key]; !ok {
			return fmt.Errorf("key [%s] does not exist", key)
		}
	}
	return nil
}

func findRenderAlias(serviceName, value string, rendSvc map[string][]string) {
	aliases := config.RenderTemplateAlias.FindAllString(value, -1)
	for _, alias := range aliases {
		rendSvc[alias] = append(rendSvc[alias], serviceName)
	}
}

func listTmplRenderKeysMap(productTmplName string, log *zap.SugaredLogger) (map[string][]string, map[string]*templatemodels.ServiceInfo, error) {
	resp := make(map[string][]string)
	keys, serviceMap, err := listTmplRenderKeys(productTmplName, log)
	if err != nil {
		return nil, nil, err
	}

	for _, key := range keys {
		resp[key.Key] = key.Services
	}

	return resp, serviceMap, nil
}

func ensureRenderSetArgs(args *commonmodels.RenderSet) error {
	if args == nil {
		return errors.New("nil RenderSet")
	}

	if len(args.Name) == 0 {
		return errors.New("empty render set name")
	}

	// 设置新的版本号
	rev, err := commonrepo.NewCounterColl().GetNextSeq("renderset:" + args.Name)
	if err != nil {
		return fmt.Errorf("get next render set revision error: %v", err)
	}

	args.Revision = rev
	return nil
}
