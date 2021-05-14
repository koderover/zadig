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
	"errors"
	"fmt"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	templatemodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models/template"
	commonrepo "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo/template"
	"github.com/koderover/zadig/lib/setting"
	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/tool/xlog"
)

func ListTmplRenderKeys(productTmplName string, log *xlog.Logger) ([]*templatemodels.RenderKV, error) {
	resp := make([]*templatemodels.RenderKV, 0)
	//如果没找到对应产品，则kv为空
	prodTmpl, err := template.NewProductColl().Find(productTmplName)
	if err != nil {
		errMsg := fmt.Sprintf("[ProductTmpl.Find] %s error: %v", productTmplName, err)
		log.Warn(errMsg)
		return resp, nil
	}

	return ListServicesRenderKeys(prodTmpl.Services, log)
}

func ListRenderSets(productTmplName string, log *xlog.Logger) ([]*commonmodels.RenderSet, error) {
	opt := &commonrepo.RenderSetListOption{
		ProductTmpl: productTmplName,
	}

	resp, err := commonrepo.NewRenderSetColl().List(opt)
	if err != nil {
		errMsg := fmt.Sprintf("[RenderSet.List] error: %v", err)
		log.Error(errMsg)
		return nil, e.ErrListTemplate.AddDesc(errMsg)
	}

	sort.SliceStable(resp, func(i, j int) bool { return resp[i].Name < resp[j].Name })

	return resp, nil
}

func GetRenderSet(renderName string, revision int64, log *xlog.Logger) (*commonmodels.RenderSet, error) {
	resp := &commonmodels.RenderSet{}
	// 未指定renderName返回空的renderSet
	if renderName == "" {
		return &commonmodels.RenderSet{KVs: []*templatemodels.RenderKV{}}, nil
	}
	opt := &commonrepo.RenderSetFindOption{
		Name:     renderName,
		Revision: revision,
	}
	resp, err := commonrepo.NewRenderSetColl().Find(opt)
	if err != nil {
		return resp, err
	}
	err = SetRenderDataStatus(resp, log)
	if err != nil {
		return resp, err
	}

	return resp, nil
}

func GetRenderSetInfo(renderName string, revision int64) (*commonmodels.RenderSet, error) {
	resp := &commonmodels.RenderSet{}
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
func ValidateRenderSet(productName, renderName, ServiceName string, log *xlog.Logger) (*commonmodels.RenderSet, error) {
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
	if ServiceName == "" {
		if err := IsAllKeyCovered(resp, log); err != nil {
			log.Errorf("[%s]cover all key [%s] error: %v", productName, renderName, err)
			return resp, err
		}
	} else {
		//  单个服务是否全覆盖判断
		if err := IsAllKeyCoveredService(ServiceName, resp, log); err != nil {
			log.Errorf("[%s]cover all key [%s] error: %v", productName, renderName, err)
			return resp, err
		}
	}
	return resp, nil
}

func RelateRender(productName, renderName string, log *xlog.Logger) error {
	renderSet, err := commonrepo.NewRenderSetColl().Find(&commonrepo.RenderSetFindOption{Name: renderName})
	if err != nil {
		errMsg := fmt.Sprintf("[RenderSet.find] %s/%s error: %v", renderName, productName, err)
		log.Error(errMsg)
		return e.ErrGetTemplate.AddDesc(errMsg)
	}
	if err = ensureRenderSetArgs(renderSet); err != nil {
		log.Error(err)
		return e.ErrCreateRenderSet.AddDesc(err.Error())
	}
	renderSet.ProductTmpl = productName
	if err = IsAllKeyCovered(renderSet, log); err != nil {
		log.Error(err)
		return e.ErrCreateRenderSet.AddDesc(err.Error())
	}
	if err := commonrepo.NewRenderSetColl().Create(renderSet); err != nil {
		errMsg := fmt.Sprintf("[RenderSet.Create] %s error: %v", renderSet.Name, err)
		log.Error(errMsg)
		return e.ErrCreateRenderSet.AddDesc(errMsg)
	}
	return nil
}

func CreateRenderSet(args *commonmodels.RenderSet, log *xlog.Logger) error {
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

func UpdateRenderSet(args *commonmodels.RenderSet, log *xlog.Logger) error {
	err := commonrepo.NewRenderSetColl().Update(args)
	if err != nil {
		errMsg := fmt.Sprintf("[RenderSet.update] %s error: %+v", args.Name, err)
		log.Error(errMsg)
		return e.ErrUpdateRenderSet.AddDesc(errMsg)
	}

	return nil
}

func SetDefaultRenderSet(renderTmplName, productTmplName string, log *xlog.Logger) error {
	if err := commonrepo.NewRenderSetColl().SetDefault(renderTmplName, productTmplName); err != nil {
		errMsg := fmt.Sprintf("[RenderSet.SetDefault] %s/%s error: %v", renderTmplName, productTmplName, err)
		log.Error(errMsg)
		return e.ErrSetDefaultRenderSet.AddDesc(errMsg)
	}
	return nil
}

func ListServicesRenderKeys(services [][]string, log *xlog.Logger) ([]*templatemodels.RenderKV, error) {
	renderSvcMap := make(map[string][]string)
	resp := make([]*templatemodels.RenderKV, 0)
	svcSet := sets.NewString()
	for _, service := range services {
		svcSet.Insert(service...)
	}

	serviceTmpls, err := commonrepo.NewServiceColl().ListMaxRevisionsForServices(svcSet.UnsortedList(), setting.K8SDeployType)
	if err != nil {
		errMsg := fmt.Sprintf("[serviceTmpl.ListMaxRevisions] error: %v", err)
		log.Error(errMsg)
		return resp, fmt.Errorf(errMsg)
	}

	configTmpls, err := commonrepo.NewConfigColl().ListMaxRevisions("")
	if err != nil {
		errMsg := fmt.Sprintf("[configTmpl.ListMaxRevisions] error: %v", err)
		log.Error(errMsg)
		return resp, fmt.Errorf(errMsg)
	}

	for _, serviceTmpl := range serviceTmpls {
		findRenderAlias(serviceTmpl.ServiceName, serviceTmpl.Yaml, renderSvcMap)
		cfgTmpls := findConfigTmpl(serviceTmpl.ServiceName, configTmpls)
		for _, cfgTmpl := range cfgTmpls {
			for _, cd := range cfgTmpl.ConfigData {
				findRenderAlias(serviceTmpl.ServiceName, cd.Value, renderSvcMap)
			}
		}
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

func SetRenderDataStatus(rs *commonmodels.RenderSet, log *xlog.Logger) error {
	availableKeys, err := listTmplRenderKeysMap(rs.ProductTmpl, log)
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
		kv.Value, _ = getRenderSetValue(kv, log)
	}

	rs.KVs = respKVs

	rs.SetKVAlias()
	return nil
}

func UpdateSubRenderSet(name string, kvs []*templatemodels.RenderKV, log *xlog.Logger) error {
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

func DeleteRenderSet(productName string, log *xlog.Logger) error {
	if err := commonrepo.NewRenderSetColl().Delete(productName); err != nil {
		errMsg := fmt.Sprintf("[RenderSet.Delete] %s error: %v", productName, err)
		log.Error(errMsg)
		return e.ErrDeleteRenderSet.AddDesc(errMsg)
	}
	return nil
}

func ValidateKVs(kvs []*templatemodels.RenderKV, services [][]string, log *xlog.Logger) error {
	resp := make(map[string][]string)
	keys, err := ListServicesRenderKeys(services, log)
	if err != nil {
		return fmt.Errorf("service.ListServicesRenderKeys to list %v %v", services, err)
	}

	for _, key := range keys {
		resp[key.Key] = key.Services
	}

	kvMap := make(map[string]string, 0)
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
// 如果RenderKV的service有多个，暂时按第一个处理
func getRenderSetValue(kv *templatemodels.RenderKV, log *xlog.Logger) (string, error) {
	if len(kv.Services) == 0 {
		return "", nil
	}

	opt := &commonrepo.ServiceFindOption{
		ServiceName:   kv.Services[0],
		ExcludeStatus: setting.ProductStatusDeleting,
	}
	serviceTmpl, err := commonrepo.NewServiceColl().Find(opt)
	if err != nil {
		log.Errorf("serviceTmpl.Find failed, serviceName:%s, error:%v", kv.Services[0], err)
		return "", err
	}
	if serviceTmpl.Visibility != setting.PUBLICSERVICE {
		return "", nil
	}

	renderSetOpt := &commonrepo.RenderSetFindOption{
		Name:     serviceTmpl.ProductName,
		Revision: 0,
	}
	renderSet, err := commonrepo.NewRenderSetColl().Find(renderSetOpt)
	if err != nil {
		log.Errorf("RenderSet.Find failed, ProductName:%s, error:%v", serviceTmpl.ProductName, err)
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
		if _, ok := rendSvc[alias]; !ok {
			rendSvc[alias] = []string{serviceName}
		} else {
			rendSvc[alias] = append(rendSvc[alias], serviceName)
		}
	}
	return
}

func findK8sServiceTmpl(serviceName string, serviceTmpls []*commonmodels.Service) *commonmodels.Service {
	for _, serviceTmpl := range serviceTmpls {
		if serviceTmpl.ServiceName == serviceName && serviceTmpl.Type == setting.K8SDeployType {
			return serviceTmpl
		}
	}
	return &commonmodels.Service{}
}
func findConfigTmpl(serviceName string, configTmpls []*commonmodels.Config) []*commonmodels.Config {
	var resp []*commonmodels.Config
	for _, configTmpl := range configTmpls {
		if configTmpl.ServiceName == serviceName {
			resp = append(resp, configTmpl)
		}
	}
	return resp
}

func listTmplRenderKeysMap(productTmplName string, log *xlog.Logger) (map[string][]string, error) {
	resp := make(map[string][]string)
	keys, err := ListTmplRenderKeys(productTmplName, log)
	if err != nil {
		return nil, err
	}

	for _, key := range keys {
		resp[key.Key] = key.Services
	}

	return resp, nil
}

// IsAllKeyCovered 检查是否覆盖所有产品key
func IsAllKeyCovered(arg *commonmodels.RenderSet, log *xlog.Logger) error {
	// 允许不关联产品
	if arg.ProductTmpl == "" {
		return nil
	}
	availableKeys, err := listTmplRenderKeysMap(arg.ProductTmpl, log)
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
func IsAllKeyCoveredService(serviceName string, arg *commonmodels.RenderSet, log *xlog.Logger) error {
	renderSvcMap := make(map[string][]string)
	opt := &commonrepo.ServiceFindOption{
		ServiceName:   serviceName,
		Type:          setting.K8SDeployType,
		ExcludeStatus: setting.ProductStatusDeleting,
	}

	serviceTmpl, err := commonrepo.NewServiceColl().Find(opt)
	if err != nil {
		return err
	}

	findRenderAlias(serviceName, serviceTmpl.Yaml, renderSvcMap)

	cfgTmpl, err := GetConfigTemplateByService(serviceTmpl.ServiceName, log)
	if err != nil {
		return err
	}
	for _, cd := range cfgTmpl.ConfigData {
		findRenderAlias(serviceName, cd.Value, renderSvcMap)
	}
	kvMap := arg.GetKeyValueMap()
	for k := range renderSvcMap {
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
	log := xlog.NewDummy()
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
