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
	"errors"
	"fmt"
	"sort"
	"strings"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	templatemodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
)

type RepoConfig struct {
	CodehostID  int      `json:"codehostID,omitempty"`
	Owner       string   `json:"owner,omitempty"`
	Repo        string   `json:"repo,omitempty"`
	Branch      string   `json:"branch,omitempty"`
	ValuesPaths []string `json:"valuesPaths,omitempty"`
}

type KVPair struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type RenderChartArg struct {
	EnvName        string      `json:"envName,omitempty"`
	ServiceName    string      `json:"serviceName,omitempty"`
	ChartVersion   string      `json:"chartVersion,omitempty"`
	YamlSource     string      `json:"yamlSource,omitempty"`
	GitRepoConfig  *RepoConfig `json:"gitRepoConfig,omitempty"`
	OverrideValues []*KVPair   `json:"overrideValues,omitempty"`
	ValuesYAML     string      `json:"valuesYAML,omitempty"`
}

func (args *RenderChartArg) toOverrideValueString() string {
	if len(args.OverrideValues) == 0 {
		return ""
	}
	bs, err := json.Marshal(args.OverrideValues)
	if err != nil {
		log.Errorf("override values json marshal error")
		return ""
	}
	return string(bs)
}

func (args *RenderChartArg) fromOverrideValueString(valueStr string) {
	if valueStr == "" {
		args.OverrideValues = nil
		return
	}

	args.OverrideValues = make([]*KVPair, 0)
	err := json.Unmarshal([]byte(valueStr), &args.OverrideValues)
	if err != nil {
		log.Errorf("decode override value fail, ")
	}
}

func (args *RenderChartArg) toCustomValuesYaml() *templatemodels.OverrideYaml {
	switch args.YamlSource {
	case setting.ValuesYamlSourceFreeEdit:
		return &templatemodels.OverrideYaml{
			YamlSource:  args.YamlSource,
			YamlContent: args.ValuesYAML,
		}
	case setting.ValuesYamlSourceGitRepo:
		return &templatemodels.OverrideYaml{
			YamlSource:  args.YamlSource,
			YamlContent: args.ValuesYAML,
			ValuesPaths: args.GitRepoConfig.ValuesPaths,
			GitRepoConfig: &templatemodels.GitRepoConfig{
				CodehostID: args.GitRepoConfig.CodehostID,
				Owner:      args.GitRepoConfig.Owner,
				Repo:       args.GitRepoConfig.Repo,
				Branch:     args.GitRepoConfig.Branch,
			},
		}
	}
	return nil
}

func (args *RenderChartArg) fromCustomValueYaml(customValuesYaml *templatemodels.OverrideYaml) {
	if customValuesYaml == nil {
		return
	}
	args.YamlSource = customValuesYaml.YamlSource
	switch customValuesYaml.YamlSource {
	case setting.ValuesYamlSourceFreeEdit:
		args.ValuesYAML = customValuesYaml.YamlContent
	case setting.ValuesYamlSourceGitRepo:
		args.ValuesYAML = ""
		if customValuesYaml.GitRepoConfig != nil {
			args.GitRepoConfig = &RepoConfig{
				CodehostID:  customValuesYaml.GitRepoConfig.CodehostID,
				Owner:       customValuesYaml.GitRepoConfig.Owner,
				Repo:        customValuesYaml.GitRepoConfig.Repo,
				Branch:      customValuesYaml.GitRepoConfig.Branch,
				ValuesPaths: customValuesYaml.ValuesPaths,
			}
		}

	}
}

// FillRenderChartModel fill render chart model
func (args *RenderChartArg) FillRenderChartModel(chart *templatemodels.RenderChart, version string) {
	chart.ServiceName = args.ServiceName
	chart.ChartVersion = version
	chart.OverrideValues = args.toOverrideValueString()
	chart.OverrideYaml = args.toCustomValuesYaml()
}

// LoadFromRenderChartModel load from render chart model
func (args *RenderChartArg) LoadFromRenderChartModel(chart *templatemodels.RenderChart) {
	args.ServiceName = chart.ServiceName
	args.ChartVersion = chart.ChartVersion
	args.fromOverrideValueString(chart.OverrideValues)
	args.fromCustomValueYaml(chart.OverrideYaml)
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

func GetRenderSet(renderName string, revision int64, log *zap.SugaredLogger) (*commonmodels.RenderSet, error) {
	// 未指定renderName返回空的renderSet
	if renderName == "" {
		return &commonmodels.RenderSet{KVs: []*templatemodels.RenderKV{}}, nil
	}
	opt := &commonrepo.RenderSetFindOption{
		Name:     renderName,
		Revision: revision,
	}
	resp, found, err := commonrepo.NewRenderSetColl().FindRenderSet(opt)
	if err != nil {
		return nil, err
	} else if !found {
		return &commonmodels.RenderSet{}, nil
	}
	err = SetRenderDataStatus(resp, log)
	if err != nil {
		return resp, err
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
func ValidateRenderSet(productName, renderName string, serviceInfo *templatemodels.ServiceInfo, log *zap.SugaredLogger) (*commonmodels.RenderSet, error) {
	resp := &commonmodels.RenderSet{ProductTmpl: productName}
	var err error
	if renderName != "" {
		opt := &commonrepo.RenderSetFindOption{Name: renderName}
		resp, err = commonrepo.NewRenderSetColl().Find(opt)
		if err != nil {
			log.Errorf("find renderset[%s] error: %v", renderName, err)
			return resp, err
		}
	}
	if renderName != "" && resp.ProductTmpl != productName {
		log.Errorf("renderset[%s] not match product[%s]", renderName, productName)
		return resp, fmt.Errorf("renderset[%s] not match product[%s]", renderName, productName)
	}
	if serviceInfo == nil {
		if err := IsAllKeyCovered(resp, log); err != nil {
			log.Errorf("[%s]cover all key [%s] error: %v", productName, renderName, err)
			return resp, err
		}
	} else {
		//  单个服务是否全覆盖判断
		if err := IsAllKeyCoveredService(serviceInfo.Owner, serviceInfo.Name, resp, log); err != nil {
			log.Errorf("[%s]cover all key [%s] error: %v", productName, renderName, err)
			return resp, err
		}
	}
	return resp, nil
}

func CreateRenderSet(args *commonmodels.RenderSet, log *zap.SugaredLogger) error {
	opt := &commonrepo.RenderSetFindOption{Name: args.Name}
	rs, err := commonrepo.NewRenderSetColl().Find(opt)
	if rs != nil && err == nil {
		// 已经存在渲染配置集
		// 判断是否有修改
		if rs.Diff(args) {
			args.IsDefault = rs.IsDefault
		} else {
			return nil
		}
	}
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

// CreateHelmRenderSet 添加renderSet
func CreateHelmRenderSet(args *commonmodels.RenderSet, log *zap.SugaredLogger) error {
	opt := &commonrepo.RenderSetFindOption{Name: args.Name}
	rs, err := commonrepo.NewRenderSetColl().Find(opt)
	if rs != nil && err == nil {
		// 已经存在渲染配置集
		// 判断是否有修改
		if rs.HelmRenderDiff(args) {
			args.IsDefault = rs.IsDefault
		} else {
			return nil
		}
	}
	if err := ensureHelmRenderSetArgs(args); err != nil {
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

func SetRenderDataStatus(rs *commonmodels.RenderSet, log *zap.SugaredLogger) error {
	availableKeys, serviceMap, err := listTmplRenderKeysMap(rs.ProductTmpl, log)
	if err != nil {
		return err
	}

	respKVs := make([]*templatemodels.RenderKV, 0)
	for _, kv := range rs.KVs {

		_, ok := availableKeys[kv.Key]
		if ok {
			// 如果渲染配置KEY在服务和配置模板中存在
			presentKv := &templatemodels.RenderKV{
				Key:      kv.Key,
				Value:    kv.Value,
				State:    config.KeyStatePresent,
				Services: availableKeys[kv.Key],
			}
			respKVs = append(respKVs, presentKv)
		} else {
			// 如果渲染配置KEY在服务和配置模板中不存在, 说明KEY并没有使用到
			unusedKV := &templatemodels.RenderKV{
				Key:      kv.Key,
				Value:    kv.Value,
				State:    config.KeyStateUnused,
				Services: []string{},
			}
			respKVs = append(respKVs, unusedKV)
		}
	}

	kvMap := rs.GetKeyValueMap()
	for key, val := range availableKeys {
		if _, ok := kvMap[key]; !ok {
			// 在服务和配置模板中找到新配置
			newKV := &templatemodels.RenderKV{
				Key:      key,
				State:    config.KeyStateNew,
				Services: val,
			}
			respKVs = append(respKVs, newKV)
		}
	}

	// 如果kv来源于共享服务，需要从该共享服务所属项目的renderset中获取value
	for _, kv := range respKVs {
		if kv.Value != "" {
			continue
		}
		kv.Value, _ = getValueFromSharedRenderSet(kv, rs.ProductTmpl, serviceMap, log)
	}

	rs.KVs = respKVs

	rs.SetKVAlias()
	return nil
}

func UpdateSubRenderSet(name string, kvs []*templatemodels.RenderKV, log *zap.SugaredLogger) error {
	renderSets, err := commonrepo.NewRenderSetColl().List(&commonrepo.RenderSetListOption{ProductTmpl: name})
	if err != nil {
		return fmt.Errorf("service.UpdateSubRenderSet RenderSet.List %v", err)
	}

	for _, renderSet := range renderSets {
		if renderSet.IsDefault {
			continue
		}

		mapping := renderSet.GetKeyValueMap()

		newKvs := make([]*templatemodels.RenderKV, 0)

		for _, kv := range kvs {
			if v, ok := mapping[kv.Key]; ok {
				newKvs = append(newKvs, &templatemodels.RenderKV{Key: kv.Key, Value: v})
			} else {
				newKvs = append(newKvs, &templatemodels.RenderKV{Key: kv.Key, Value: kv.Value})
			}
		}

		renderSet.KVs = newKvs

		err = CreateRenderSet(renderSet, log)

		if err != nil {
			return fmt.Errorf("service.UpdateSubRenderSet UpddateExistRenderSet %v", err)
		}
	}

	return nil
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

func RenderValueForString(origin string, rs *commonmodels.RenderSet) string {
	if rs == nil {
		return origin
	}
	rs.SetKVAlias()
	for _, v := range rs.KVs {
		origin = strings.Replace(origin, v.Alias, v.Value, -1)
	}
	return origin
}

// getRenderSetValue 获取render set的value
func getValueFromSharedRenderSet(kv *templatemodels.RenderKV, productName string, serviceMap map[string]*templatemodels.ServiceInfo, log *zap.SugaredLogger) (string, error) {
	targetProduct := ""
	for _, serviceName := range kv.Services {
		info := serviceMap[serviceName]
		if info != nil && info.Owner != productName {
			targetProduct = info.Owner
			break
		}
	}
	if targetProduct == "" {
		return "", nil
	}

	renderSetOpt := &commonrepo.RenderSetFindOption{
		Name:     targetProduct,
		Revision: 0,
	}
	renderSet, err := commonrepo.NewRenderSetColl().Find(renderSetOpt)
	if err != nil {
		log.Errorf("RenderSet.Find failed, ProductName:%s, error:%v", targetProduct, err)
		return "", err
	}
	for _, originKv := range renderSet.KVs {
		if originKv.Key == kv.Key {
			return originKv.Value, nil
		}
	}

	return "", nil
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

// IsAllKeyCovered 检查是否覆盖所有产品key
func IsAllKeyCovered(arg *commonmodels.RenderSet, log *zap.SugaredLogger) error {
	// 允许不关联产品
	if arg.ProductTmpl == "" {
		return nil
	}
	availableKeys, _, err := listTmplRenderKeysMap(arg.ProductTmpl, log)
	if err != nil {
		return err
	}

	kvMap := arg.GetKeyValueMap()
	for key := range availableKeys {
		if _, ok := kvMap[key]; !ok {
			return fmt.Errorf("key [%s] does not exist", key)
		}
	}
	return nil
}

// IsAllKeyCoveredService 检查是否覆盖所有服务key
func IsAllKeyCoveredService(productName, serviceName string, arg *commonmodels.RenderSet, log *zap.SugaredLogger) error {
	opt := &commonrepo.ServiceFindOption{
		ServiceName:   serviceName,
		ProductName:   productName,
		Type:          setting.K8SDeployType,
		ExcludeStatus: setting.ProductStatusDeleting,
	}

	serviceTmpl, err := commonrepo.NewServiceColl().Find(opt)
	if err != nil {
		return err
	}

	renderAlias := config.RenderTemplateAlias.FindAllString(serviceTmpl.Yaml, -1)

	kvMap := arg.GetKeyValueMap()
	for _, k := range renderAlias {
		kv := templatemodels.RenderKV{Alias: k}
		kv.SetKeys()
		if _, ok := kvMap[kv.Key]; !ok {
			return fmt.Errorf("key [%s] does not exist", k)
		}
	}
	return nil
}

func ensureRenderSetArgs(args *commonmodels.RenderSet) error {
	if args == nil {
		return errors.New("nil RenderSet")
	}

	if len(args.Name) == 0 {
		return errors.New("empty render set name")
	}
	log := log.SugaredLogger()
	if err := IsAllKeyCovered(args, log); err != nil {
		return fmt.Errorf("[RenderSet.Create] %s error: %v", args.Name, err)
	}

	// 设置新的版本号
	rev, err := commonrepo.NewCounterColl().GetNextSeq("renderset:" + args.Name)
	if err != nil {
		return fmt.Errorf("get next render set revision error: %v", err)
	}

	args.Revision = rev
	return nil
}

// ensureHelmRenderSetArgs ...
func ensureHelmRenderSetArgs(args *commonmodels.RenderSet) error {
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
