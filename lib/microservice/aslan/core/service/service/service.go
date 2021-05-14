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
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/yaml"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	templatemodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models/template"
	commonrepo "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	templaterepo "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo/template"
	commonservice "github.com/koderover/zadig/lib/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/service/codehost"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/service/command"
	s3service "github.com/koderover/zadig/lib/microservice/aslan/core/common/service/s3"
	"github.com/koderover/zadig/lib/microservice/aslan/internal/log"
	"github.com/koderover/zadig/lib/setting"
	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/tool/gerrit"
	"github.com/koderover/zadig/lib/tool/xlog"
	"github.com/koderover/zadig/lib/util"
)

type ServiceOption struct {
	ServiceModules []*ServiceModule           `json:"service_module"`
	SystemVariable []*Variable                `json:"system_variable"`
	CustomVariable []*templatemodels.RenderKV `json:"custom_variable"`
	Yaml           string                     `json:"yaml"`
	Service        *commonmodels.Service      `json:"service,omitempty"`
}

type ServiceModule struct {
	*commonmodels.Container
	BuildName string `json:"build_name"`
}

type Variable struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type YamlPreviewForPorts struct {
	Kind string `bson:"-"           json:"kind"`
	Spec *struct {
		Ports []struct {
			Name string `bson:"-"           json:"name"`
			Port int    `bson:"-"           json:"port"`
		} `bson:"-"           json:"ports"`
	} `bson:"-"           json:"spec"`
}

type KubeResourceKind struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
}

type KubeResource struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		Template struct {
			Spec struct {
				Containers []map[string]interface{} `yaml:"containers"`
			} `yaml:"spec"`
		} `yaml:"template"`
	} `yaml:"spec"`
}

type CronjobResource struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		Template struct {
			Spec struct {
				Template struct {
					Spec struct {
						Containers []map[string]interface{} `yaml:"containers"`
					} `yaml:"spec"`
				} `yaml:"template"`
			} `yaml:"spec"`
		} `yaml:"jobTemplate"`
	} `yaml:"spec"`
}

type YamlPreview struct {
	Kind string `bson:"-"           json:"kind"`
}

type YamlValidatorReq struct {
	ServiceName string `json:"service_name"`
	Yaml        string `json:"yaml,omitempty"`
}

type ServiceObject struct {
	ServiceName string `json:"service_name"`
	ProductName string `json:"product_name"`
}

func ListServicesInExtenalEnv(tmpResp *commonservice.ServiceTmplResp, log *xlog.Logger) {
	opt := &commonrepo.ProductListOptions{
		Source:        setting.SourceFromExternal,
		ExcludeStatus: setting.ProductStatusDeleting,
	}
	products, err := commonrepo.NewProductColl().List(opt)
	if err != nil {
		log.Errorf("Product.List failed, source:%s, err:%v", setting.SourceFromExternal, err)
	} else {
		for _, prod := range products {
			services, _, err := commonservice.ListGroupsBySource(prod.EnvName, prod.ProductName, log)
			if err != nil {
				log.Errorf("ListGroupsBySource failed, envName:%s, productName:%s, source:%s, err:%v", prod.EnvName, prod.ProductName, setting.SourceFromExternal, err)
				continue
			}
			for _, service := range services {
				spmap := &commonservice.ServiceProductMap{
					Service:     service.ServiceName,
					Type:        service.Type,
					ProductName: service.ProductName,
					Product:     []string{service.ProductName},
				}
				tmpResp.Data = append(tmpResp.Data, spmap)
			}
		}
	}
	return
}

func GetServiceTemplateOption(serviceName, productName string, revision int64, log *xlog.Logger) (*ServiceOption, error) {
	service, err := commonservice.GetServiceTemplate(serviceName, setting.K8SDeployType, productName, setting.ProductStatusDeleting, revision, log)
	if err != nil {
		return nil, err
	}

	serviceOption, err := GetServiceOption(service, log)
	serviceOption.Service = service
	return serviceOption, err
}

func GetServiceOption(args *commonmodels.Service, log *xlog.Logger) (*ServiceOption, error) {
	serviceOption := new(ServiceOption)

	serviceModules := make([]*ServiceModule, 0)
	for _, container := range args.Containers {
		serviceModule := new(ServiceModule)
		serviceModule.Container = container
		buildObj, _ := commonrepo.NewBuildColl().Find(&commonrepo.BuildFindOption{ProductName: args.ProductName, ServiceName: args.ServiceName, Targets: []string{container.Name}})
		if buildObj != nil {
			serviceModule.BuildName = buildObj.Name
		}
		serviceModules = append(serviceModules, serviceModule)
	}
	serviceOption.ServiceModules = serviceModules
	serviceOption.SystemVariable = []*Variable{
		&Variable{
			Key:   "$Product$",
			Value: args.ProductName},
		&Variable{
			Key:   "$Service$",
			Value: args.ServiceName},
		&Variable{
			Key:   "$Namespace$",
			Value: ""},
		&Variable{
			Key:   "$EnvName$",
			Value: ""},
	}
	renderKVs, err := commonservice.ListServicesRenderKeys([][]string{{args.ServiceName}}, log)
	if err != nil {
		log.Errorf("ListServicesRenderKeys %s error: %v", args.ServiceName, err)
		return nil, e.ErrCreateTemplate.AddDesc(err.Error())
	}
	serviceOption.CustomVariable = renderKVs

	prodTmpl, err := templaterepo.NewProductColl().Find(args.ProductName)
	if err != nil {
		errMsg := fmt.Sprintf("[ProductTmpl.Find] %s error: %v", args.ProductName, err)
		log.Error(errMsg)
		return nil, e.ErrGetProduct.AddDesc(errMsg)
	}

	err = commonservice.FillProductTemplateVars([]*templatemodels.Product{prodTmpl}, log)
	if err != nil {
		return nil, err
	}
	if prodTmpl.Vars != nil {
		renderKVsMap := make(map[string]*templatemodels.RenderKV, 0)
		for _, serviceRenderKV := range renderKVs {
			renderKVsMap[serviceRenderKV.Key] = serviceRenderKV
		}
		productRenderEnvs := make([]*templatemodels.RenderKV, 0)
		for _, envVar := range prodTmpl.Vars {
			delete(renderKVsMap, envVar.Key)
			renderEnv := &templatemodels.RenderKV{
				Key:      envVar.Key,
				Value:    envVar.Value,
				Alias:    envVar.Alias,
				State:    envVar.State,
				Services: envVar.Services,
			}
			productRenderEnvs = append(productRenderEnvs, renderEnv)
		}

		for _, envVar := range renderKVsMap {
			envVar.State = config.KeyStateNew
			productRenderEnvs = append(productRenderEnvs, envVar)
		}
		serviceOption.CustomVariable = productRenderEnvs
	}

	if args.Source == setting.SourceFromGitlab || args.Source == setting.SourceFromGithub || args.Source == setting.SourceFromGerrit {
		serviceOption.Yaml = args.Yaml
	}
	return serviceOption, nil
}

func CreateServiceTemplate(userName string, args *commonmodels.Service, log *xlog.Logger) (*ServiceOption, error) {

	opt := &commonrepo.ServiceFindOption{
		ServiceName:   args.ServiceName,
		ExcludeStatus: setting.ProductStatusDeleting,
	}

	// 在更新数据库前检查是否有完全重复的Item，如果有，则退出。
	serviceTmpl, notFoundErr := commonrepo.NewServiceColl().Find(opt)
	if notFoundErr == nil {
		if serviceTmpl.ProductName != args.ProductName {
			return nil, e.ErrInvalidParam.AddDesc(fmt.Sprintf("项目 [%s] %s", serviceTmpl.ProductName, "有相同的服务名称存在,请检查!"))
		}
		if args.Type == setting.K8SDeployType && args.Source == serviceTmpl.Source {
			// 配置来源为zadig，对比配置内容是否变化，需要对比Yaml内容
			// 如果Source没有设置，默认认为是zadig平台管理配置方式
			if args.Source == setting.SourceFromZadig || args.Source == "" {
				if args.Yaml != "" && strings.Compare(serviceTmpl.Yaml, args.Yaml) == 0 {
					log.Info("Yaml config remains the same, quit creation.")
					return GetServiceOption(serviceTmpl, log)
				}
			}
			// 配置来源为Gitlab，对比配置的ChangeLog是否变化
			if args.Source == setting.SourceFromGitlab {
				if args.Commit != nil && serviceTmpl.Commit != nil && args.Commit.SHA == serviceTmpl.Commit.SHA {
					log.Info("gitlab change log remains the same, quit creation.")
					return GetServiceOption(serviceTmpl, log)
				}
				if args.LoadPath != serviceTmpl.LoadPath && serviceTmpl.LoadPath != "" {
					log.Errorf("Changing load path is not allowed")
					return nil, e.ErrCreateTemplate.AddDesc("不允许更改加载路径")
				}
			} else if args.Source == setting.SourceFromGithub {
				if args.Commit != nil && serviceTmpl.Commit != nil && args.Commit.SHA == serviceTmpl.Commit.SHA {
					log.Info("github change log remains the same, quit creation.")
					return GetServiceOption(serviceTmpl, log)
				}
				if args.LoadPath != serviceTmpl.LoadPath && serviceTmpl.LoadPath != "" {
					log.Errorf("Changing load path is not allowed")
					return nil, e.ErrCreateTemplate.AddDesc("不允许更改加载路径")
				}
			} else if args.Source == setting.SourceFromGerrit {
				if args.Yaml != "" && strings.Compare(serviceTmpl.Yaml, args.Yaml) == 0 {
					log.Info("gerrit Yaml config remains the same, quit creation.")
					return GetServiceOption(serviceTmpl, log)
				}
				if args.LoadPath != serviceTmpl.LoadPath && serviceTmpl.LoadPath != "" {
					log.Errorf("Changing load path is not allowed")
					return nil, e.ErrCreateTemplate.AddDesc("不允许更改加载路径")
				}
				if err := updateGerritWebhookByService(serviceTmpl, args); err != nil {
					log.Info("gerrit update webhook err :%v", err)
					return nil, err
				}
			}
		} else if args.Source == setting.SourceFromGerrit {
			err := GetGerritServiceYaml(args, log)
			if err != nil {
				log.Errorf("GetGerritServiceYaml from gerrit failed, error: %v", err)
				return nil, err
			}
			//创建gerrit webhook
			if err = createGerritWebhookByService(args.GerritCodeHostID, args.ServiceName, args.GerritRepoName, args.GerritBranchName); err != nil {
				log.Errorf("createGerritWebhookByService error: %v", err)
				return nil, err
			}
		}
	} else {
		if args.Source == setting.SourceFromGerrit {
			//创建gerrit webhook
			if err := createGerritWebhookByService(args.GerritCodeHostID, args.ServiceName, args.GerritRepoName, args.GerritBranchName); err != nil {
				log.Errorf("createGerritWebhookByService error: %v", err)
				return nil, err
			}
		}
	}

	// 校验args
	if err := ensureServiceTmpl(userName, args, log); err != nil {
		log.Errorf("ensureServiceTmpl error: %+v", err)
		return nil, e.ErrValidateTemplate.AddDesc(err.Error())
	}

	if err := commonrepo.NewServiceColl().Delete(args.ServiceName, args.Type, "", setting.ProductStatusDeleting, args.Revision); err != nil {
		log.Errorf("ServiceTmpl.delete %s error: %v", args.ServiceName, err)
	}

	if err := commonrepo.NewServiceColl().Create(args); err != nil {
		log.Errorf("ServiceTmpl.Create %s error: %v", args.ServiceName, err)
		return nil, e.ErrCreateTemplate.AddDesc(err.Error())
	}

	if notFoundErr != nil {
		if productTempl, err := commonservice.GetProductTemplate(args.ProductName, log); err == nil {
			//获取项目里面的所有服务
			if len(productTempl.Services) > 0 && !sets.NewString(productTempl.Services[0]...).Has(args.ServiceName) {
				productTempl.Services[0] = append(productTempl.Services[0], args.ServiceName)
			} else {
				productTempl.Services = [][]string{{args.ServiceName}}
			}
			//更新项目模板
			err = templaterepo.NewProductColl().Update(args.ProductName, productTempl)
			if err != nil {
				log.Errorf("CreateServiceTemplate Update %s error: %v", args.ServiceName, err)
				return nil, e.ErrCreateTemplate.AddDesc(err.Error())
			}
		}
	}

	return GetServiceOption(args, log)
}

func UpdateServiceTemplate(args *commonservice.ServiceTmplObject) error {
	if args.Visibility == "private" {
		servicesMap, err := distincProductServices("")
		if err != nil {
			errMsg := fmt.Sprintf("get distincProductServices error: %v", err)
			log.Error(errMsg)
			return e.ErrDeleteTemplate.AddDesc(errMsg)
		}

		if _, ok := servicesMap[args.ServiceName]; ok {
			for _, productName := range servicesMap[args.ServiceName] {
				if args.ProductName != productName {
					log.Error(fmt.Sprintf("service %s is already taken by %s",
						args.ServiceName, strings.Join(servicesMap[args.ServiceName], ",")))

					errMsg := fmt.Sprintf("共享服务 [%s] 已被 [%s] 等项目服务编排中使用，请解除引用后再修改",
						args.ServiceName, strings.Join(servicesMap[args.ServiceName], ","))
					return e.ErrInvalidParam.AddDesc(errMsg)
				}
			}

		}
	}
	updateArgs := &commonmodels.Service{
		ProductName: args.ProductName,
		ServiceName: args.ServiceName,
		Visibility:  args.Visibility,
		Revision:    args.Revision,
		Type:        args.Type,
		CreateBy:    args.Username,
		EnvConfigs:  args.EnvConfigs,
		EnvStatuses: args.EnvStatuses,
	}
	return commonrepo.NewServiceColl().Update(updateArgs)
}

func YamlValidator(args *YamlValidatorReq) []string {
	//validateWithCacheMap := make(map[string]*gojsonschema.Schema)
	errorDetails := make([]string, 0)
	if args.Yaml == "" {
		return errorDetails
	}
	yamlContent := util.ReplaceWrapLine(args.Yaml)
	KubeYamls := SplitYaml(yamlContent)
	totalErrorLineNum := 0
	for index, data := range KubeYamls {
		yamlDataArray := SplitYaml(data)
		for _, yamlData := range yamlDataArray {
			//验证格式
			if strings.Count(yamlData, "apiVersion") > 1 {
				tmpYamlDataArray := strings.Split(yamlData, "\napiVersion")
				tmpYamlData := 0
				for index, _ := range tmpYamlDataArray {
					tmpYamlData += strings.Count(tmpYamlDataArray[index], "\n") + 1
					if index != len(tmpYamlDataArray)-1 {
						if strings.Contains(tmpYamlDataArray[index], "---") {
							errorDetails = append(errorDetails, fmt.Sprintf("系统检测到%s %d %s %d %s", "在", totalErrorLineNum+tmpYamlData, "行和", totalErrorLineNum+tmpYamlData+1, "行之间---前后可能存在空格,请检查!"))
						} else if strings.Contains(tmpYamlDataArray[index], "-- -") || strings.Contains(tmpYamlDataArray[index], "- --") || strings.Contains(tmpYamlDataArray[index], "- - -") {
							errorDetails = append(errorDetails, fmt.Sprintf("%s %d %s %d %s", "在", totalErrorLineNum+tmpYamlData, "行和", totalErrorLineNum+tmpYamlData+1, "行之间---中间不能存在空格,请检查!"))
						} else {
							errorDetails = append(errorDetails, fmt.Sprintf("%s %d %s %d %s", "在", totalErrorLineNum+tmpYamlData, "行和", totalErrorLineNum+tmpYamlData+1, "行之间必须使用---进行拼接,请添加!"))
						}
					}
				}
				return errorDetails
			}
			resKind := new(KubeResourceKind)
			//在Unmarshal之前填充渲染变量{{.}}
			yamlData = config.RenderTemplateAlias.ReplaceAllLiteralString(yamlData, "ssssssss")
			// replace $Service$ with service name
			yamlData = config.ServiceNameAlias.ReplaceAllLiteralString(yamlData, args.ServiceName)

			if err := yaml.Unmarshal([]byte(yamlData), &resKind); err != nil {
				if index == 0 {
					errorDetails = append(errorDetails, err.Error())
					return errorDetails
				}
				if strings.Contains(err.Error(), "yaml: line") {
					re := regexp.MustCompile("[0-9]+")
					errorLineNums := re.FindAllString(err.Error(), -1)
					errorLineNum, _ := strconv.Atoi(errorLineNums[0])
					totalErrorLineNum = totalErrorLineNum + errorLineNum
					errMessage := re.ReplaceAllString(err.Error(), strconv.Itoa(totalErrorLineNum))

					errorDetails = append(errorDetails, errMessage)
					return errorDetails
				}
			}

			totalErrorLineNum += strings.Count(yamlData, "\n") + 2
		}
	}

	return errorDetails
}

func DeleteServiceTemplate(serviceName, serviceType, productName, isEnvTemplate, visibility string, log *xlog.Logger) error {
	openEnvTemplate, err := strconv.ParseBool(isEnvTemplate)
	if openEnvTemplate && visibility == "public" {
		servicesMap, err := distincProductServices("")
		if err != nil {
			errMsg := fmt.Sprintf("get distincProductServices error: %v", err)
			log.Error(errMsg)
			return e.ErrDeleteTemplate.AddDesc(errMsg)
		}

		if _, ok := servicesMap[serviceName]; ok {
			for _, svcProductName := range servicesMap[serviceName] {
				if productName != svcProductName {
					log.Error(fmt.Sprintf("service %s is already taken by %s",
						serviceName, strings.Join(servicesMap[serviceName], ",")))

					errMsg := fmt.Sprintf("共享服务 [%s] 已被 [%s] 等项目服务编排中使用，请解除引用后再删除",
						serviceName, strings.Join(servicesMap[serviceName], ","))
					return e.ErrInvalidParam.AddDesc(errMsg)
				}
			}
		}

		serviceEnvMap, err := distinctEnvServices(productName)
		if err != nil {
			errMsg := fmt.Sprintf("get distinctServiceEnv error: %v", err)
			log.Error(errMsg)
			return e.ErrDeleteTemplate.AddDesc(errMsg)
		}

		if _, ok := serviceEnvMap[serviceName]; ok {
			var envNames []string
			for _, env := range serviceEnvMap[serviceName] {
				envNames = append(envNames, env.EnvName+"-"+env.ProductName)
			}

			log.Error(fmt.Sprintf("failed to delete service, %s already exists in %s",
				serviceName, strings.Join(envNames, ", ")))

			errMsg := fmt.Sprintf("共享服务 %s 已存在于 %s 等环境中，请对这些环境进行更新后再删除",
				serviceName, strings.Join(envNames, ", "))
			return e.ErrInvalidParam.AddDesc(errMsg)
		}
	}

	err = commonrepo.NewServiceColl().UpdateStatus(serviceName, serviceType, productName, setting.ProductStatusDeleting)
	if err != nil {
		errMsg := fmt.Sprintf("[service.UpdateStatus] %s-%s error: %v", serviceName, serviceType, err)
		log.Error(errMsg)
		return e.ErrDeleteTemplate.AddDesc(errMsg)
	}
	// 删除服务时不删除counter，兼容服务删除后重建的场景
	//serviceTemplate := fmt.Sprintf(template.ServiceTemplateCounterName, serviceName, serviceType)
	//err = s.coll.Counter.Delete(serviceTemplate)
	//if err != nil {
	//	log.Errorf("Counter.Delete error: %v", err)
	//	return e.ErrDeleteCounter.AddDesc(err.Error())
	//}

	if serviceType == setting.HelmDeployType {
		// 更新helm renderset
		renderOpt := &commonrepo.RenderSetFindOption{Name: productName}
		if rs, err := commonrepo.NewRenderSetColl().Find(renderOpt); err == nil {
			chartInfos := make([]*templatemodels.RenderChart, 0)
			for _, chartInfo := range rs.ChartInfos {
				if chartInfo.ServiceName == serviceName {
					continue
				}
				chartInfos = append(chartInfos, chartInfo)
			}
			rs.ChartInfos = chartInfos
			_ = commonrepo.NewRenderSetColl().Update(rs)
		}
		// 把该服务相关的s3的数据从仓库删除
		s3Storage, _ := s3service.FindDefaultS3()
		if s3Storage != nil {
			subFolderName := serviceName + "-" + setting.HelmDeployType
			if s3Storage.Subfolder != "" {
				s3Storage.Subfolder = fmt.Sprintf("%s/%s/%s", s3Storage.Subfolder, subFolderName, "service")
			} else {
				s3Storage.Subfolder = fmt.Sprintf("%s/%s", subFolderName, "service")
			}
			s3service.RemoveFiles(s3Storage, []string{fmt.Sprintf("%s.tar.gz", serviceName)}, false)
		}
	}

	//删除环境模板
	if productTempl, err := commonservice.GetProductTemplate(productName, log); err == nil {
		newServices := make([][]string, len(productTempl.Services))
		for i, services := range productTempl.Services {
			for _, service := range services {
				if service != serviceName {
					newServices[i] = append(newServices[i], service)
				}
			}
		}
		productTempl.Services = newServices
		err = templaterepo.NewProductColl().Update(productName, productTempl)
		if err != nil {
			log.Errorf("DeleteServiceTemplate Update %s error: %v", serviceName, err)
			return e.ErrDeleteTemplate.AddDesc(err.Error())
		}
	}

	return nil
}

func ListServicePort(serviceName, serviceType, productName, excludeStatus string, revision int64, log *xlog.Logger) ([]int, error) {
	servicePorts := make([]int, 0)

	opt := &commonrepo.ServiceFindOption{
		ServiceName:   serviceName,
		Type:          serviceType,
		Revision:      revision,
		ProductName:   productName,
		ExcludeStatus: excludeStatus,
	}

	resp, err := commonrepo.NewServiceColl().Find(opt)
	if err != nil {
		errMsg := fmt.Sprintf("[ServiceTmpl.Find] %s error: %v", serviceName, err)
		log.Error(errMsg)
		return servicePorts, e.ErrGetTemplate.AddDesc(errMsg)
	}

	yamlNew := strings.Replace(resp.Yaml, " {{", " '{{", -1)
	yamlNew = strings.Replace(yamlNew, "}}\n", "}}'\n", -1)
	yamls := strings.Split(yamlNew, "---")
	for _, yamlStr := range yamls {
		data, err := yaml.YAMLToJSON([]byte(yamlStr))
		if err != nil {
			log.Errorf("convert yaml to json failed, yaml:%s, err:%v", yamlStr, err)
			continue
		}

		yamlPreview := YamlPreviewForPorts{}
		err = json.Unmarshal(data, &yamlPreview)
		if err != nil {
			log.Errorf("unmarshal yaml data failed, yaml:%s, err:%v", yamlStr, err)
			continue
		}

		if yamlPreview.Kind != "Service" || yamlPreview.Spec == nil {
			continue
		}
		for _, port := range yamlPreview.Spec.Ports {
			servicePorts = append(servicePorts, port.Port)
		}
	}

	return servicePorts, nil
}

func ListServiceTemplateNames(productName string, log *xlog.Logger) ([]ServiceObject, error) {
	var err error
	serviceObjects := make([]ServiceObject, 0)

	//获取该项目下的所有服务
	serviceTmpls, err := commonrepo.NewServiceColl().DistinctServices(&commonrepo.ServiceListOption{ExcludeStatus: setting.ProductStatusDeleting})
	if err != nil {
		log.Errorf("ServiceTmpl.DistinctServices error: %v", err)
		return serviceObjects, e.ErrListTemplate.AddDesc(err.Error())
	}

	for _, serviceTmpl := range serviceTmpls {
		var serviceObject ServiceObject
		serviceObject.ProductName = serviceTmpl.ProductName
		serviceObject.ServiceName = serviceTmpl.ServiceName

		if serviceTmpl.ProductName == productName {
			serviceObjects = append(serviceObjects, serviceObject)
			continue
		}

		opt := &commonrepo.ServiceFindOption{
			ServiceName:   serviceTmpl.ServiceName,
			Type:          serviceTmpl.Type,
			Revision:      serviceTmpl.Revision,
			ProductName:   serviceTmpl.ProductName,
			ExcludeStatus: setting.ProductStatusDeleting,
		}

		resp, err := commonrepo.NewServiceColl().Find(opt)
		if err != nil {
			log.Errorf("ServiceTmpl Find error: %v", err)
			return serviceObjects, e.ErrGetTemplate.AddDesc(err.Error())
		}

		if resp.Visibility == setting.PUBLICSERVICE {
			serviceObjects = append(serviceObjects, serviceObject)
		}
	}
	return serviceObjects, nil
}

func GetGerritServiceYaml(args *commonmodels.Service, log *xlog.Logger) error {
	//同步commit信息
	codehostDetail, err := syncGerritLatestCommit(args)
	if err != nil {
		log.Errorf("Sync change log from gerrit failed, error: %v", err)
		return err
	}

	base := path.Join(config.S3StoragePath(), args.GerritRepoName)
	if _, err := os.Stat(base); os.IsNotExist(err) {
		err = command.RunGitCmds(codehostDetail, setting.GerritDefaultOwner, args.GerritRepoName, args.GerritBranchName, args.GerritRemoteName)
		if err != nil {
			return err
		}
	}

	filePath, err := os.Stat(path.Join(base, args.GerritPath))
	if err != nil {
		return err
	}

	if filePath.IsDir() {
		fileInfos, err := ioutil.ReadDir(path.Join(base, args.GerritPath))
		if err != nil {
			return err
		}
		fileContents := make([]string, 0)
		for _, file := range fileInfos {
			if contentBytes, err := ioutil.ReadFile(path.Join(base, args.GerritPath, file.Name())); err == nil {
				fileContents = append(fileContents, string(contentBytes))
			} else {
				log.Errorf("CreateServiceTemplate gerrit ReadFile err:%v", err)
			}
		}
		args.Yaml = joinYamls(fileContents)
	} else {
		if contentBytes, err := ioutil.ReadFile(path.Join(base, args.GerritPath)); err == nil {
			args.Yaml = string(contentBytes)
		}
	}
	return nil
}

func ensureServiceTmpl(userName string, args *commonmodels.Service, log *xlog.Logger) error {
	if args == nil {
		return errors.New("service template arg is null")
	}
	if len(args.ServiceName) == 0 {
		return errors.New("service name is empty")
	}
	if !config.ServiceNameRegex.MatchString(args.ServiceName) {
		return fmt.Errorf("导入的文件目录和文件名称仅支持字母，数字， 中划线和 下划线")
	}
	if args.Type == setting.K8SDeployType {
		if args.Containers == nil {
			args.Containers = make([]*commonmodels.Container, 0)
		}
		// 配置来源为Gitlab，需要从Gitlab同步配置，并设置KubeYamls.
		if args.Source != setting.SourceFromGithub && args.Source != setting.SourceFromGitlab {
			// 拆分 all-in-one yaml文件
			// 替换分隔符
			args.Yaml = util.ReplaceWrapLine(args.Yaml)
			// 分隔符为\n---\n
			args.KubeYamls = SplitYaml(args.Yaml)
		}

		// 遍历args.KubeYamls，获取 Deployment 或者 StatefulSet 里面所有containers 镜像和名称
		if err := setCurrentContainerImages(args); err != nil {
			return err
		}
		log.Infof("find %d containers in service %s", len(args.Containers), args.ServiceName)
	}

	// 设置新的版本号
	serviceTemplate := fmt.Sprintf(setting.ServiceTemplateCounterName, args.ServiceName, args.Type)
	rev, err := commonrepo.NewCounterColl().GetNextSeq(serviceTemplate)
	if err != nil {
		return fmt.Errorf("get next service template revision error: %v", err)
	}

	args.Revision = rev

	return nil
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

// distincEnvServices 查询使用到服务模板的环境
func distinctEnvServices(productName string) (map[string][]*commonmodels.Product, error) {
	serviceMap := make(map[string][]*commonmodels.Product)
	envs, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{Name: productName})
	if err != nil {
		return serviceMap, err
	}

	for _, env := range envs {
		for _, group := range env.Services {
			for _, service := range group {
				if _, ok := serviceMap[service.ServiceName]; !ok {
					serviceMap[service.ServiceName] = []*commonmodels.Product{env}
				} else {
					serviceMap[service.ServiceName] = append(serviceMap[service.ServiceName], env)
				}
			}
		}
	}
	return serviceMap, nil
}

func setCurrentContainerImages(args *commonmodels.Service) error {
	var srvContainers []*commonmodels.Container
	for _, data := range args.KubeYamls {
		yamlDataArray := SplitYaml(data)
		for _, yamlData := range yamlDataArray {
			resKind := new(KubeResourceKind)
			//在Unmarshal之前填充渲染变量{{.}}
			yamlData = config.RenderTemplateAlias.ReplaceAllLiteralString(yamlData, "ssssssss")
			// replace $Service$ with service name
			yamlData = config.ServiceNameAlias.ReplaceAllLiteralString(yamlData, args.ServiceName)

			if err := yaml.Unmarshal([]byte(yamlData), &resKind); err != nil {
				return fmt.Errorf("unmarshal ResourceKind error: %v", err)
			}

			if resKind == nil {
				return errors.New("nil ReourceKind")
			}

			if resKind.Kind == setting.Deployment || resKind.Kind == setting.StatefulSet || resKind.Kind == setting.Job {
				containers, err := getContainers(yamlData)
				if err != nil {
					return fmt.Errorf("GetContainers error: %v", err)
				}

				srvContainers = append(srvContainers, containers...)
			} else if resKind.Kind == setting.CronJob {
				containers, err := getCronJobContainers(yamlData)
				if err != nil {
					return fmt.Errorf("GetCronjobContainers error: %v", err)
				}
				srvContainers = append(srvContainers, containers...)
			}
		}
	}

	args.Containers = uniqeSlice(srvContainers)

	return nil
}

func getContainers(data string) ([]*commonmodels.Container, error) {
	containers := make([]*commonmodels.Container, 0, 0)

	res := new(KubeResource)
	if err := yaml.Unmarshal([]byte(data), &res); err != nil {
		return containers, fmt.Errorf("unmarshal yaml data error: %v", err)
	}

	for _, val := range res.Spec.Template.Spec.Containers {

		if _, ok := val["name"]; !ok {
			return containers, errors.New("yaml file missing name key")
		}

		nameStr, ok := val["name"].(string)
		if !ok {
			return containers, errors.New("error name value")
		}

		if _, ok := val["image"]; !ok {
			return containers, errors.New("yaml file missing image key")
		}

		imageStr, ok := val["image"].(string)
		if !ok {
			return containers, errors.New("error image value")
		}

		container := &commonmodels.Container{
			Name:  nameStr,
			Image: imageStr,
		}

		containers = append(containers, container)
	}

	return containers, nil
}

func getCronJobContainers(data string) ([]*commonmodels.Container, error) {
	containers := make([]*commonmodels.Container, 0, 0)

	res := new(CronjobResource)
	if err := yaml.Unmarshal([]byte(data), &res); err != nil {
		return containers, fmt.Errorf("unmarshal yaml data error: %v", err)
	}

	for _, val := range res.Spec.Template.Spec.Template.Spec.Containers {

		if _, ok := val["name"]; !ok {
			return containers, errors.New("yaml file missing name key")
		}

		nameStr, ok := val["name"].(string)
		if !ok {
			return containers, errors.New("error name value")
		}

		if _, ok := val["image"]; !ok {
			return containers, errors.New("yaml file missing image key")
		}

		imageStr, ok := val["image"].(string)
		if !ok {
			return containers, errors.New("error image value")
		}

		container := &commonmodels.Container{
			Name:  nameStr,
			Image: imageStr,
		}

		containers = append(containers, container)
	}

	return containers, nil
}

func updateGerritWebhookByService(lastService, currentService *commonmodels.Service) error {
	var codehostDetail *codehost.Detail
	var err error
	if lastService != nil && lastService.Source == setting.SourceFromGerrit {
		codehostDetail, err = codehost.GetCodehostDetail(lastService.GerritCodeHostID)
		if err != nil {
			log.Errorf("updateGerritWebhookByService GetCodehostDetail err:%v", err)
			return err
		}
		webhookURLPrefix := fmt.Sprintf("%s/%s/%s/%s", codehostDetail.Address, "a/config/server/webhooks~projects", gerrit.Escape(lastService.GerritRepoName), "remotes")
		_, _ = gerrit.Do(fmt.Sprintf("%s/%s", webhookURLPrefix, gerrit.RemoteName), "DELETE", codehostDetail.OauthToken, util.GetRequestBody(&gerrit.GerritWebhook{}))
		_, err = gerrit.Do(fmt.Sprintf("%s/%s", webhookURLPrefix, lastService.ServiceName), "DELETE", codehostDetail.OauthToken, util.GetRequestBody(&gerrit.GerritWebhook{}))
		if err != nil {
			log.Errorf("updateGerritWebhookByService deleteGerritWebhook err:%v", err)
		}
	}
	var detail *codehost.Detail
	if lastService.GerritCodeHostID == currentService.GerritCodeHostID && codehostDetail != nil {
		detail = codehostDetail
	} else {
		detail, err = codehost.GetCodehostDetail(currentService.GerritCodeHostID)
	}

	webhookURL := fmt.Sprintf("%s/%s/%s/%s/%s", detail.Address, "a/config/server/webhooks~projects", gerrit.Escape(currentService.GerritRepoName), "remotes", currentService.ServiceName)
	if _, err := gerrit.Do(webhookURL, "GET", detail.OauthToken, nil); err != nil {
		log.Errorf("updateGerritWebhookByService getGerritWebhook err:%v", err)
		//创建webhook
		gerritWebhook := &gerrit.GerritWebhook{
			URL:       fmt.Sprintf("%s/%s%s", config.AslanAPIBase(), "gerritHook?name=", currentService.ServiceName),
			MaxTries:  setting.MaxTries,
			SslVerify: false,
		}
		_, err = gerrit.Do(webhookURL, "PUT", detail.OauthToken, util.GetRequestBody(gerritWebhook))
		if err != nil {
			log.Errorf("updateGerritWebhookByService addGerritWebhook err:%v", err)
			return err
		}
	}
	return nil
}

func createGerritWebhookByService(codehostID int, serviceName, repoName, branchName string) error {
	detail, err := codehost.GetCodehostDetail(codehostID)
	if err != nil {
		log.Errorf("createGerritWebhookByService GetCodehostDetail err:%v", err)
		return err
	}
	webhookURL := fmt.Sprintf("%s/%s/%s/%s/%s", detail.Address, "a/config/server/webhooks~projects", gerrit.Escape(repoName), "remotes", serviceName)
	if _, err := gerrit.Do(webhookURL, "GET", detail.OauthToken, nil); err != nil {
		log.Errorf("createGerritWebhookByService getGerritWebhook err:%v", err)
		//创建webhook
		gerritWebhook := &gerrit.GerritWebhook{
			URL:       fmt.Sprintf("%s/%s%s", config.AslanAPIBase(), "gerritHook?name=", serviceName),
			MaxTries:  setting.MaxTries,
			SslVerify: false,
		}
		_, err = gerrit.Do(webhookURL, "PUT", detail.OauthToken, util.GetRequestBody(gerritWebhook))
		if err != nil {
			log.Errorf("createGerritWebhookByService addGerritWebhook err:%v", err)
			return err
		}
	}
	return nil
}

func syncGerritLatestCommit(service *commonmodels.Service) (*codehost.Detail, error) {
	if service.GerritCodeHostID == 0 {
		return nil, fmt.Errorf("codehostId不能是空的")
	}
	if service.GerritRepoName == "" {
		return nil, fmt.Errorf("repoName不能是空的")
	}
	if service.GerritBranchName == "" {
		return nil, fmt.Errorf("branchName不能是空的")
	}
	ch, err := codehost.GetCodehostDetail(service.GerritCodeHostID)

	gerritCli := gerrit.NewClient(ch.Address, ch.OauthToken)
	commit, err := gerritCli.GetCommitByBranch(service.GerritRepoName, service.GerritBranchName)
	if err != nil {
		return nil, err
	}

	service.Commit = &commonmodels.Commit{
		SHA:     commit.Commit,
		Message: commit.Message,
	}
	return ch, nil
}

func joinYamls(files []string) string {
	return strings.Join(files, setting.YamlFileSeperator)
}

func SplitYaml(yaml string) []string {
	return strings.Split(yaml, setting.YamlFileSeperator)
}

func uniqeSlice(elements []*commonmodels.Container) []*commonmodels.Container {
	existing := make(map[string]bool)
	ret := make([]*commonmodels.Container, 0)
	for _, elem := range elements {
		key := elem.Name
		if _, ok := existing[key]; !ok {
			ret = append(ret, elem)
			existing[key] = true
		}
	}

	return ret
}
