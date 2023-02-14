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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"text/template"
	gotemplate "text/template"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/yaml"

	configbase "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	templatemodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/command"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/fs"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/pm"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/environment/service"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/client/systemconfig"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/gerrit"
	"github.com/koderover/zadig/pkg/tool/httpclient"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/util"
	"github.com/koderover/zadig/pkg/util/converter"
)

type ServiceOption struct {
	ServiceModules []*ServiceModule           `json:"service_module"`
	SystemVariable []*Variable                `json:"system_variable"`
	VariableYaml   string                     `json:"variable_yaml"`
	ServiceVars    []string                   `json:"service_vars"`
	VariableKVs    []*commonmodels.VariableKV `json:"variable_kvs"`
	Yaml           string                     `json:"yaml"`
	Service        *commonmodels.Service      `json:"service,omitempty"`
}

type ServiceModule struct {
	*commonmodels.Container
	BuildNames []string `json:"build_names"`
}

type Variable struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	Description string `json:"description,omitempty"`
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
	ServiceName  string `json:"service_name"`
	VariableYaml string `json:"variable_yaml"`
	Yaml         string `json:"yaml,omitempty"`
}

type YamlViewServiceTemplateReq struct {
	ServiceName string                     `json:"service_name"`
	ProjectName string                     `json:"project_name"`
	EnvName     string                     `json:"env_name"`
	Variables   []*templatemodels.RenderKV `json:"variables"`
}

type ReleaseNamingRule struct {
	NamingRule  string `json:"naming"`
	ServiceName string `json:"service_name"`
}

func ListServicesInExtenalEnv(tmpResp *commonservice.ServiceTmplResp, log *zap.SugaredLogger) {
	opt := &commonrepo.ProductListOptions{
		Source:        setting.SourceFromExternal,
		ExcludeStatus: []string{setting.ProductStatusDeleting, setting.ProductStatusUnknown},
	}
	products, err := commonrepo.NewProductColl().List(opt)
	if err != nil {
		log.Errorf("Product.List failed, source:%s, err:%v", setting.SourceFromExternal, err)
	} else {
		for _, prod := range products {
			_, services, err := commonservice.ListWorkloadsInEnv(prod.EnvName, prod.ProductName, "", 0, 0, log)
			if err != nil {
				log.Errorf("ListWorkloadsInEnv failed, envName:%s, productName:%s, source:%s, err:%v", prod.EnvName, prod.ProductName, setting.SourceFromExternal, err)
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
}

func GetServiceTemplateOption(serviceName, productName string, revision int64, log *zap.SugaredLogger) (*ServiceOption, error) {
	service, err := commonservice.GetServiceTemplate(serviceName, setting.K8SDeployType, productName, setting.ProductStatusDeleting, revision, log)
	if err != nil {
		return nil, err
	}

	serviceOption, err := GetServiceOption(service, log)
	if serviceOption != nil {
		serviceOption.Service = service
	}

	return serviceOption, err
}

// getTemplateMergedVariables gets merged variable yaml if service is created from yaml template
func getTemplateMergedVariables(args *commonmodels.Service) string {
	if args.TemplateID == "" {
		return args.VariableYaml
	}
	templateInfo, err := commonrepo.NewYamlTemplateColl().GetById(args.TemplateID)
	if err != nil {
		log.Errorf("failed to find template with id: %s for service: %s, err: %s", args.TemplateID, args.ServiceName, err)
		return ""
	}

	variableYaml, err := buildYamlTemplateVariables(args, templateInfo)
	if err != nil {
		log.Errorf("failed to extract template variables for service: %s, err: %s", args.ServiceName, err)
		return ""
	}
	return variableYaml
}

func GetServiceOption(args *commonmodels.Service, log *zap.SugaredLogger) (*ServiceOption, error) {
	serviceOption := new(ServiceOption)

	serviceModules := make([]*ServiceModule, 0)
	for _, container := range args.Containers {
		serviceModule := new(ServiceModule)
		serviceModule.Container = container
		serviceModule.ImageName = util.GetImageNameFromContainerInfo(container.ImageName, container.Name)
		buildObjs, err := commonrepo.NewBuildColl().List(&commonrepo.BuildListOption{ProductName: args.ProductName, ServiceName: args.ServiceName, Targets: []string{container.Name}})
		if err != nil {
			return nil, err
		}

		buildNames := sets.NewString()
		for _, buildObj := range buildObjs {
			buildNames.Insert(buildObj.Name)
		}
		serviceModule.BuildNames = buildNames.List()
		serviceModules = append(serviceModules, serviceModule)
	}
	serviceOption.ServiceModules = serviceModules
	serviceOption.SystemVariable = []*Variable{
		{
			Key:   "$Product$",
			Value: args.ProductName},
		{
			Key:   "$Service$",
			Value: args.ServiceName},
		{
			Key:   "$Namespace$",
			Value: ""},
		{
			Key:   "$EnvName$",
			Value: ""},
	}

	//serviceOption.VariableYaml = getTemplateMergedVariables(args)
	serviceOption.VariableYaml = args.VariableYaml
	serviceOption.ServiceVars = args.ServiceVars
	if len(serviceOption.VariableYaml) > 0 {
		flatMap, err := converter.YamlToFlatMap([]byte(serviceOption.VariableYaml))
		if err != nil {
			log.Errorf("failed to get flat map of variable, err: %s", err)
		} else {
			allKeys := sets.NewString()
			for k, v := range flatMap {
				serviceOption.VariableKVs = append(serviceOption.VariableKVs, &commonmodels.VariableKV{
					Key:   k,
					Value: v,
				})
				allKeys.Insert(k)
			}
			validServiceVars := make([]string, 0)
			for _, k := range serviceOption.ServiceVars {
				if allKeys.Has(k) {
					validServiceVars = append(validServiceVars, k)
				}
			}
			serviceOption.ServiceVars = validServiceVars
		}
	}

	if args.Source == setting.SourceFromGitlab || args.Source == setting.SourceFromGithub ||
		args.Source == setting.SourceFromGerrit || args.Source == setting.SourceFromCodeHub || args.Source == setting.SourceFromGitee {
		serviceOption.Yaml = args.Yaml
	}
	return serviceOption, nil
}

type K8sWorkloadsArgs struct {
	WorkLoads   []commonmodels.Workload `json:"workLoads"`
	EnvName     string                  `json:"env_name"`
	ClusterID   string                  `json:"cluster_id"`
	Namespace   string                  `json:"namespace"`
	ProductName string                  `json:"product_name"`
	RegistryID  string                  `json:"registry_id"`
}

func CreateK8sWorkLoads(ctx context.Context, requestID, userName string, args *K8sWorkloadsArgs, log *zap.SugaredLogger) error {
	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), args.ClusterID)
	if err != nil {
		log.Errorf("[%s] error: %v", args.Namespace, err)
		return err
	}
	// 检查环境是否存在，envName和productName唯一
	opt := &commonrepo.ProductFindOptions{Name: args.ProductName, EnvName: args.EnvName}
	if _, err := commonrepo.NewProductColl().Find(opt); err == nil {
		log.Errorf("[%s][P:%s] duplicate envName in the same project", args.EnvName, args.ProductName)
		return e.ErrCreateEnv.AddDesc(e.DuplicateEnvErrMsg)
	}

	// todo Add data filter
	var (
		workloadsTmp []commonmodels.Workload
		mu           sync.Mutex
	)

	serviceString := sets.NewString()
	services, _ := commonrepo.NewServiceColl().ListExternalWorkloadsBy(args.ProductName, "")
	for _, v := range services {
		serviceString.Insert(v.ServiceName)
	}
	//for _, workload := range workLoads {
	//	if serviceString.Has(workload.Name) {
	//		return e.ErrCreateTemplate.AddDesc(fmt.Sprintf("do not support import same service name: %s", workload.Name))
	//	}
	//}

	g := new(errgroup.Group)
	for _, workload := range args.WorkLoads {
		tempWorkload := workload
		g.Go(func() error {
			// If the service is already included in the database service template, add it to the new association table
			if serviceString.Has(tempWorkload.Name) {
				return commonrepo.NewServicesInExternalEnvColl().Create(&commonmodels.ServicesInExternalEnv{
					ProductName: args.ProductName,
					ServiceName: tempWorkload.Name,
					EnvName:     args.EnvName,
					Namespace:   args.Namespace,
					ClusterID:   args.ClusterID,
				})
			}

			var bs []byte
			switch tempWorkload.Type {
			case setting.Deployment:
				bs, _, err = getter.GetDeploymentYaml(args.Namespace, tempWorkload.Name, kubeClient)
			case setting.StatefulSet:
				bs, _, err = getter.GetStatefulSetYaml(args.Namespace, tempWorkload.Name, kubeClient)
			}

			if len(bs) == 0 || err != nil {
				log.Errorf("not found yaml %v", err)
				return e.ErrGetService.AddDesc(fmt.Sprintf("get deploy/sts failed err:%s", err))
			}

			mu.Lock()
			defer mu.Unlock()
			workloadsTmp = append(workloadsTmp, commonmodels.Workload{
				EnvName:     args.EnvName,
				Name:        tempWorkload.Name,
				Type:        tempWorkload.Type,
				ProductName: args.ProductName,
			})

			return CreateWorkloadTemplate(userName, &commonmodels.Service{
				ServiceName:  tempWorkload.Name,
				Yaml:         string(bs),
				ProductName:  args.ProductName,
				CreateBy:     userName,
				Type:         setting.K8SDeployType,
				WorkloadType: tempWorkload.Type,
				Source:       setting.SourceFromExternal,
				EnvName:      args.EnvName,
				Revision:     1,
			}, log)
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}

	// 没有环境，创建环境
	if _, err = commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    args.ProductName,
		EnvName: args.EnvName,
	}); err != nil {
		// no need to create renderset since renderset is not necessary in host projects
		if err := service.CreateProduct(userName, requestID, &commonmodels.Product{
			ProductName: args.ProductName,
			Source:      setting.SourceFromExternal,
			ClusterID:   args.ClusterID,
			RegistryID:  args.RegistryID,
			EnvName:     args.EnvName,
			Namespace:   args.Namespace,
			UpdateBy:    userName,
		}, log); err != nil {
			return e.ErrCreateProduct.AddDesc("create product Error for unknown reason")
		}
	}

	workLoadStat, err := commonrepo.NewWorkLoadsStatColl().Find(args.ClusterID, args.Namespace)
	if err != nil {
		workLoadStat = &commonmodels.WorkloadStat{
			ClusterID: args.ClusterID,
			Namespace: args.Namespace,
			Workloads: workloadsTmp,
		}
		return commonrepo.NewWorkLoadsStatColl().Create(workLoadStat)
	}

	workLoadStat.Workloads = replaceWorkloads(workLoadStat.Workloads, workloadsTmp, args.EnvName)
	return commonrepo.NewWorkLoadsStatColl().UpdateWorkloads(workLoadStat)
}

type ServiceWorkloadsUpdateAction struct {
	EnvName     string `json:"env_name"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	ProductName string `json:"product_name"`
	Operation   string `json:"operation"`
}

type UpdateWorkloadsArgs struct {
	WorkLoads []commonmodels.Workload `json:"workLoads"`
	ClusterID string                  `json:"cluster_id"`
	Namespace string                  `json:"namespace"`
}

func UpdateWorkloads(ctx context.Context, requestID, username, productName, envName string, args UpdateWorkloadsArgs, log *zap.SugaredLogger) error {
	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), args.ClusterID)
	if err != nil {
		log.Errorf("[%s] error: %s", args.Namespace, err)
		return err
	}
	workloadStat, err := commonrepo.NewWorkLoadsStatColl().Find(args.ClusterID, args.Namespace)
	if err != nil {
		log.Errorf("[%s][%s]NewWorkLoadsStatColl().Find %s", args.ClusterID, args.Namespace, err)
		return err
	}
	externalEnvServices, _ := commonrepo.NewServicesInExternalEnvColl().List(&commonrepo.ServicesInExternalEnvArgs{
		ProductName: productName,
		EnvName:     envName,
	})

	for _, externalEnvService := range externalEnvServices {
		workloadStat.Workloads = append(workloadStat.Workloads, commonmodels.Workload{
			ProductName: externalEnvService.ProductName,
			EnvName:     externalEnvService.EnvName,
			Name:        externalEnvService.ServiceName,
		})
	}

	diff := map[string]*ServiceWorkloadsUpdateAction{}
	originSet := sets.NewString()
	uploadSet := sets.NewString()
	for _, v := range workloadStat.Workloads {
		if v.ProductName == productName && v.EnvName == envName {
			originSet.Insert(v.Name)
		}
	}

	for _, v := range args.WorkLoads {
		uploadSet.Insert(v.Name)
	}
	// 判断是删除还是增加
	deleteString := originSet.Difference(uploadSet)
	addString := uploadSet.Difference(originSet)
	for _, v := range workloadStat.Workloads {
		if v.ProductName != productName {
			continue
		}
		if deleteString.Has(v.Name) {
			diff[v.Name] = &ServiceWorkloadsUpdateAction{
				EnvName:     envName,
				Name:        v.Name,
				Type:        v.Type,
				ProductName: v.ProductName,
				Operation:   "delete",
			}
		}
	}

	for _, v := range args.WorkLoads {
		if addString.Has(v.Name) {
			diff[v.Name] = &ServiceWorkloadsUpdateAction{
				EnvName:     envName,
				Name:        v.Name,
				Type:        v.Type,
				ProductName: v.ProductName,
				Operation:   "add",
			}
		}
	}

	otherExternalEnvServices, err := commonrepo.NewServicesInExternalEnvColl().List(&commonrepo.ServicesInExternalEnvArgs{
		ProductName:    productName,
		ExcludeEnvName: envName,
	})
	if err != nil {
		log.Errorf("failed to list external service, error:%s", err)
	}

	externalEnvServiceM := make(map[string]*commonmodels.ServicesInExternalEnv)
	for _, externalEnvService := range otherExternalEnvServices {
		externalEnvServiceM[externalEnvService.ServiceName] = externalEnvService
	}

	templateProductInfo, err := templaterepo.NewProductColl().Find(productName)
	if err != nil {
		log.Errorf("failed to find template product: %s error: %s", productName, err)
		return err
	}

	svcNeedAdd := sets.NewString()
	svcNeedDelete := sets.NewString()

	for _, v := range diff {
		switch v.Operation {
		// 删除workload的引用
		case "delete":
			if externalService, isExist := externalEnvServiceM[v.Name]; !isExist {
				if err = commonrepo.NewServiceColl().UpdateExternalServicesStatus(v.Name, productName, setting.ProductStatusDeleting, envName); err != nil {
					log.Errorf("UpdateStatus external services error:%s", err)
				}
			} else {
				// Update the env name in the service
				if err = commonrepo.NewServiceColl().UpdateExternalServiceEnvName(v.Name, productName, externalService.EnvName); err != nil {
					log.Errorf("UpdateEnvName external services error:%s", err)
				}
				// Delete the reference in the original service
				if err = commonrepo.NewServicesInExternalEnvColl().Delete(&commonrepo.ServicesInExternalEnvArgs{
					ProductName: externalService.ProductName,
					EnvName:     externalService.EnvName,
					ServiceName: externalService.ServiceName,
				}); err != nil {
					log.Errorf("delete service in external env envName:%s error:%s", externalService.EnvName, err)
				}
			}
			if err = commonrepo.NewServicesInExternalEnvColl().Delete(&commonrepo.ServicesInExternalEnvArgs{
				ProductName: productName,
				EnvName:     envName,
				ServiceName: v.Name,
			}); err != nil {
				log.Errorf("delete service in external env envName:%s error:%s", envName, err)
			}
			svcNeedDelete.Insert(v.Name)
		// 添加workload的引用
		case "add":
			var bs []byte
			switch v.Type {
			case setting.Deployment:
				bs, _, err = getter.GetDeploymentYaml(args.Namespace, v.Name, kubeClient)
			case setting.StatefulSet:
				bs, _, err = getter.GetStatefulSetYaml(args.Namespace, v.Name, kubeClient)
			}
			svcNeedAdd.Insert(v.Name)
			if len(bs) == 0 || err != nil {
				log.Errorf("UpdateK8sWorkLoads not found yaml %s", err)
				delete(diff, v.Name)
				continue
			}
			if err = CreateWorkloadTemplate(username, &commonmodels.Service{
				ServiceName:  v.Name,
				Yaml:         string(bs),
				ProductName:  productName,
				CreateBy:     username,
				Type:         setting.K8SDeployType,
				WorkloadType: v.Type,
				Source:       setting.SourceFromExternal,
				EnvName:      envName,
				Revision:     1,
			}, log); err != nil {
				log.Errorf("create service template failed err:%v", err)
				delete(diff, v.Name)
				continue
			}
		}
	}

	// for host services, services are stored in template_product.services[0]
	if len(templateProductInfo.Services) == 1 {

		func() {
			productServices, err := commonrepo.NewServiceColl().ListExternalWorkloadsBy(productName, "")
			if err != nil {
				log.Errorf("ListWorkloads ListExternalServicesBy err:%s", err)
				return
			}
			productServiceNames := sets.NewString()
			for _, productService := range productServices {
				productServiceNames.Insert(productService.ServiceName)
			}
			// add services in external env data
			servicesInExternalEnv, _ := commonrepo.NewServicesInExternalEnvColl().List(&commonrepo.ServicesInExternalEnvArgs{
				ProductName: productName,
			})
			for _, serviceInExternalEnv := range servicesInExternalEnv {
				productServiceNames.Insert(serviceInExternalEnv.ServiceName)
			}

			templateProductInfo.Services[0] = productServiceNames.List()
			err = templaterepo.NewProductColl().UpdateServiceOrchestration(templateProductInfo.ProductName, templateProductInfo.Services, templateProductInfo.UpdateBy)
			if err != nil {
				log.Errorf("failed to update service for product: %s, err: %s", templateProductInfo.ProductName, err)
			}
		}()

	}

	// 删除 && 增加
	workloadStat.Workloads = updateWorkloads(workloadStat.Workloads, diff, envName, productName)
	return commonrepo.NewWorkLoadsStatColl().UpdateWorkloads(workloadStat)
}

func updateWorkloads(existWorkloads []commonmodels.Workload, diff map[string]*ServiceWorkloadsUpdateAction, envName string, productName string) (result []commonmodels.Workload) {
	existWorkloadsMap := map[string]commonmodels.Workload{}
	for _, v := range existWorkloads {
		existWorkloadsMap[v.Name] = v
	}
	for _, v := range diff {
		switch v.Operation {
		case "add":
			vv := commonmodels.Workload{
				EnvName:     envName,
				Name:        v.Name,
				Type:        v.Type,
				ProductName: productName,
			}
			existWorkloadsMap[v.Name] = vv
		case "delete":
			delete(existWorkloadsMap, v.Name)
		}
	}
	for _, v := range existWorkloadsMap {
		result = append(result, v)
	}
	return result
}

func replaceWorkloads(existWorkloads []commonmodels.Workload, newWorkloads []commonmodels.Workload, envName string) []commonmodels.Workload {
	var result []commonmodels.Workload
	workloadMap := map[string]commonmodels.Workload{}
	for _, workload := range existWorkloads {
		if workload.EnvName != envName {
			workloadMap[workload.Name] = workload
			result = append(result, workload)
		}
	}

	for _, newWorkload := range newWorkloads {
		if _, ok := workloadMap[newWorkload.Name]; !ok {
			result = append(result, newWorkload)
		}
	}

	return result
}

// CreateWorkloadTemplate only use for workload
func CreateWorkloadTemplate(userName string, args *commonmodels.Service, log *zap.SugaredLogger) error {
	_, err := templaterepo.NewProductColl().Find(args.ProductName)
	if err != nil {
		log.Errorf("Failed to find project %s, err: %s", args.ProductName, err)
		return e.ErrInvalidParam.AddErr(err)
	}
	// 遍历args.KubeYamls，获取 Deployment 或者 StatefulSet 里面所有containers 镜像和名称
	args.KubeYamls = []string{args.Yaml}
	if err := setCurrentContainerImages(args); err != nil {
		log.Errorf("Failed tosetCurrentContainerImages %s, err: %s", args.ProductName, err)
		return err
	}
	opt := &commonrepo.ServiceFindOption{
		ServiceName:   args.ServiceName,
		ProductName:   args.ProductName,
		ExcludeStatus: setting.ProductStatusDeleting,
	}
	_, notFoundErr := commonrepo.NewServiceColl().Find(opt)
	if notFoundErr != nil {
		if productTempl, err := commonservice.GetProductTemplate(args.ProductName, log); err == nil {
			//获取项目里面的所有服务
			if len(productTempl.Services) > 0 && !sets.NewString(productTempl.Services[0]...).Has(args.ServiceName) {
				productTempl.Services[0] = append(productTempl.Services[0], args.ServiceName)
			} else if len(productTempl.Services) == 0 {
				productTempl.Services = [][]string{{args.ServiceName}}
			}
			//更新项目模板
			err = templaterepo.NewProductColl().Update(args.ProductName, productTempl)
			if err != nil {
				log.Errorf("CreateServiceTemplate Update %s error: %s", args.ServiceName, err)
				return e.ErrCreateTemplate.AddDesc(err.Error())
			}
		}
	} else {
		product, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
			Name:    args.ProductName,
			EnvName: args.EnvName,
		})
		if err != nil {
			return err
		}
		return commonrepo.NewServicesInExternalEnvColl().Create(&commonmodels.ServicesInExternalEnv{
			ProductName: args.ProductName,
			ServiceName: args.ServiceName,
			EnvName:     args.EnvName,
			Namespace:   product.Namespace,
			ClusterID:   product.ClusterID,
		})
		//return e.ErrCreateTemplate.AddDesc("do not support import same service name")
	}

	if err := commonrepo.NewServiceColl().Delete(args.ServiceName, args.Type, args.ProductName, setting.ProductStatusDeleting, 0); err != nil {
		log.Errorf("ServiceTmpl.delete %s error: %v", args.ServiceName, err)
	}

	if err := commonrepo.NewServiceColl().Create(args); err != nil {
		log.Errorf("ServiceTmpl.Create %s error: %v", args.ServiceName, err)
		return e.ErrCreateTemplate.AddDesc(err.Error())
	}
	return nil
}

// fillServiceVariable fill service.variableYaml and service.serviceVars by the previous revision
// services created by [spock, template] do not need to be filled
func fillServiceVariable(args *commonmodels.Service) {
	if args.Source == setting.SourceFromZadig || args.Source == setting.ServiceSourceTemplate {
		return
	}
	curRevision, err := commonrepo.NewServiceColl().Find(&commonrepo.ServiceFindOption{
		ServiceName:   args.ServiceName,
		Revision:      0,
		Type:          args.Type,
		ProductName:   args.ProductName,
		ExcludeStatus: setting.ProductStatusDeleting,
	})
	if err == nil && curRevision != nil {
		args.ServiceVars = curRevision.ServiceVars
		args.VariableYaml = curRevision.VariableYaml
	}
}

func CreateServiceTemplate(userName string, args *commonmodels.Service, force bool, log *zap.SugaredLogger) (*ServiceOption, error) {
	opt := &commonrepo.ServiceFindOption{
		ServiceName:   args.ServiceName,
		ProductName:   args.ProductName,
		ExcludeStatus: setting.ProductStatusDeleting,
	}

	project, err := templaterepo.NewProductColl().Find(args.ProductName)
	if err != nil {
		log.Errorf("Failed to find project %s, err: %s", args.ProductName, err)
		return nil, e.ErrInvalidParam.AddErr(err)
	}
	if _, ok := project.SharedServiceInfoMap()[args.ServiceName]; ok {
		return nil, e.ErrInvalidParam.AddDesc(fmt.Sprintf("A service with same name %s is already existing", args.ServiceName))
	}

	// 在更新数据库前检查是否有完全重复的Item，如果有，则退出。
	serviceTmpl, notFoundErr := commonrepo.NewServiceColl().Find(opt)
	if notFoundErr == nil && !force {
		return nil, fmt.Errorf("service:%s already exists", serviceTmpl.ServiceName)
	} else {
		if args.Source == setting.SourceFromGerrit {
			//创建gerrit webhook
			if err := createGerritWebhookByService(args.GerritCodeHostID, args.ServiceName, args.GerritRepoName, args.GerritBranchName); err != nil {
				log.Errorf("createGerritWebhookByService error: %v", err)
				return nil, err
			}
		}
	}

	// fill serviceVars and variableYaml
	fillServiceVariable(args)

	// 校验args
	if err := ensureServiceTmpl(userName, args, log); err != nil {
		log.Errorf("ensureServiceTmpl error: %+v", err)
		return nil, e.ErrValidateTemplate.AddDesc(err.Error())
	}

	if err := commonrepo.NewServiceColl().Delete(args.ServiceName, args.Type, args.ProductName, setting.ProductStatusDeleting, args.Revision); err != nil {
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
				log.Errorf("CreateServiceTemplate Update %s error: %s", args.ServiceName, err)
				return nil, e.ErrCreateTemplate.AddDesc(err.Error())
			}
		}
	}
	commonservice.ProcessServiceWebhook(args, serviceTmpl, args.ServiceName, log)

	err = service.AutoDeployYamlServiceToEnvs(userName, "", args, log)
	if err != nil {
		return nil, e.ErrCreateTemplate.AddErr(err)
	}

	return GetServiceOption(args, log)
}

func UpdateServiceVisibility(args *commonservice.ServiceTmplObject) error {
	currentService, err := commonrepo.NewServiceColl().Find(&commonrepo.ServiceFindOption{
		ProductName: args.ProductName,
		ServiceName: args.ServiceName,
		Revision:    args.Revision,
	})
	if err != nil {
		log.Errorf("Can not find service with option %+v. Error: %s", args, err)
		return err
	}

	if currentService.Visibility != args.Visibility && args.Visibility == "private" {
		projects, err := templaterepo.NewProductColl().ListWithOption(
			&templaterepo.ProductListOpt{ContainSharedServices: []*templatemodels.ServiceInfo{{Name: args.ServiceName, Owner: args.ProductName}}},
		)
		if err != nil {
			log.Errorf("Failed to list projects using service %s, err: %s", args.ServiceName, err)
			return err
		}
		if len(projects) > 0 {
			var names []string
			for _, p := range projects {
				names = append(names, p.ProductName)
			}
			log.Warnf("service %s is used by projects %s", args.ServiceName, strings.Join(names, ","))
			errMsg := fmt.Sprintf("共享服务 [%s] 已被 [%s] 等项目服务编排中使用，请解除引用后再修改", args.ServiceName, strings.Join(names, ","))
			return e.ErrInvalidParam.AddDesc(errMsg)
		}
	}

	envStatuses := make([]*commonmodels.EnvStatus, 0)
	// Remove environments and hosts that do not exist in the check status
	for _, envStatus := range args.EnvStatuses {
		var existEnv, existHost bool

		envName := envStatus.EnvName
		if _, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
			Name:    args.ProductName,
			EnvName: envName,
		}); err == nil {
			existEnv = true
		}

		op := commonrepo.FindPrivateKeyOption{
			Address: envStatus.Address,
			ID:      envStatus.HostID,
		}
		if _, err := commonrepo.NewPrivateKeyColl().Find(op); err == nil {
			existHost = true
		}

		if existEnv && existHost {
			envStatuses = append(envStatuses, envStatus)
		}
	}

	// for pm services, fill env status data
	if args.Type == setting.PMDeployType {
		validStatusMap := make(map[string]*commonmodels.EnvStatus)
		for _, status := range envStatuses {
			validStatusMap[fmt.Sprintf("%s-%s", status.EnvName, status.HostID)] = status
		}

		envStatus, err := pm.GenerateEnvStatus(currentService.EnvConfigs, log.SugaredLogger())
		if err != nil {
			log.Errorf("failed to generate env status")
			return err
		}
		defaultStatusMap := make(map[string]*commonmodels.EnvStatus)
		for _, status := range envStatus {
			defaultStatusMap[fmt.Sprintf("%s-%s", status.EnvName, status.HostID)] = status
		}

		for k, _ := range defaultStatusMap {
			if vv, ok := validStatusMap[k]; ok {
				defaultStatusMap[k] = vv
			}
		}

		envStatuses = make([]*commonmodels.EnvStatus, 0)
		for _, v := range defaultStatusMap {
			envStatuses = append(envStatuses, v)
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
		EnvStatuses: envStatuses,
	}
	return commonrepo.NewServiceColl().Update(updateArgs)
}

func containersChanged(oldContainers []*commonmodels.Container, newContainers []*commonmodels.Container) bool {
	if len(oldContainers) != len(newContainers) {
		return true
	}
	oldSet := sets.NewString()
	for _, container := range oldContainers {
		oldSet.Insert(container.Name)
	}
	newSet := sets.NewString()
	for _, container := range newContainers {
		newSet.Insert(container.Name)
	}
	return !oldSet.Equal(newSet)
}

func UpdateServiceVariables(args *commonservice.ServiceTmplObject) error {
	currentService, err := commonrepo.NewServiceColl().Find(&commonrepo.ServiceFindOption{
		ProductName: args.ProductName,
		ServiceName: args.ServiceName,
	})
	if err != nil {
		return e.ErrUpdateService.AddErr(fmt.Errorf("failed to get service info, err: %s", err))
	}
	if currentService.Type != setting.K8SDeployType {
		return e.ErrUpdateService.AddErr(fmt.Errorf("invalid service type: %v", currentService.Type))
	}

	currentService.ServiceVars = args.ServiceVars
	currentService.VariableYaml = args.VariableYaml

	// TODO validate service vars
	err = commonrepo.NewServiceColl().UpdateServiceVariables(currentService)
	if err != nil {
		return e.ErrUpdateService.AddErr(err)
	}

	// reparse service, check if container changes
	currentService.RenderedYaml, err = renderK8sSvcYaml(currentService.Yaml, args.ProductName, args.ServiceName, currentService.VariableYaml)
	if err != nil {
		return fmt.Errorf("failed to render yaml, err: %s", err)
	}

	currentService.RenderedYaml = util.ReplaceWrapLine(currentService.RenderedYaml)
	currentService.KubeYamls = SplitYaml(currentService.RenderedYaml)
	oldContainers := currentService.Containers
	if err := setCurrentContainerImages(currentService); err != nil {
		log.Errorf("failed to ser set container images, err: %s", err)
		//return err
	} else if containersChanged(oldContainers, currentService.Containers) {
		err = commonrepo.NewServiceColl().UpdateServiceContainers(currentService)
		if err != nil {
			log.Errorf("failed to update service containers")
		}
	}
	return nil
}

func UpdateServiceHealthCheckStatus(args *commonservice.ServiceTmplObject) error {
	currentService, err := commonrepo.NewServiceColl().Find(&commonrepo.ServiceFindOption{
		ProductName: args.ProductName,
		ServiceName: args.ServiceName,
		Revision:    args.Revision,
	})
	if err != nil {
		log.Errorf("Can not find service with option %+v. Error: %s", args, err)
		return err
	}
	var changeEnvStatus []*commonmodels.EnvStatus
	var changeEnvConfigs []*commonmodels.EnvConfig

	changeEnvConfigs = append(changeEnvConfigs, args.EnvConfigs...)
	envConfigsSet := sets.String{}
	for _, v := range changeEnvConfigs {
		envConfigsSet.Insert(v.EnvName)
	}
	for _, v := range currentService.EnvConfigs {
		if !envConfigsSet.Has(v.EnvName) {
			changeEnvConfigs = append(changeEnvConfigs, v)
		}
	}
	var privateKeys []*commonmodels.PrivateKey
	for _, envConfig := range args.EnvConfigs {
		privateKeys, err = commonrepo.NewPrivateKeyColl().ListHostIPByArgs(&commonrepo.ListHostIPArgs{IDs: envConfig.HostIDs})
		if err != nil {
			log.Errorf("ListNameByArgs ids err:%s", err)
			return err
		}

		privateKeysByLabels, err := commonrepo.NewPrivateKeyColl().ListHostIPByArgs(&commonrepo.ListHostIPArgs{Labels: envConfig.Labels})
		if err != nil {
			log.Errorf("ListNameByArgs labels err:%s", err)
			return err
		}
		privateKeys = append(privateKeys, privateKeysByLabels...)
	}
	privateKeysSet := sets.NewString()
	for _, v := range privateKeys {
		tmp := commonmodels.EnvStatus{
			HostID:  v.ID.Hex(),
			EnvName: args.EnvName,
			Address: v.IP,
		}
		if !privateKeysSet.Has(tmp.HostID) {
			changeEnvStatus = append(changeEnvStatus, &tmp)
			privateKeysSet.Insert(tmp.HostID)
		}
	}
	// get env status
	for _, v := range currentService.EnvStatuses {
		if v.EnvName != args.EnvName {
			changeEnvStatus = append(changeEnvStatus, v)
		}
	}
	// generate env status for this env
	updateArgs := &commonmodels.Service{
		ProductName: args.ProductName,
		ServiceName: args.ServiceName,
		Visibility:  args.Visibility,
		Revision:    args.Revision,
		Type:        args.Type,
		CreateBy:    args.Username,
		EnvConfigs:  changeEnvConfigs,
		EnvStatuses: changeEnvStatus,
	}
	return commonrepo.NewServiceColl().UpdateServiceHealthCheckStatus(updateArgs)
}

func extractHostIPs(privateKeys []*commonmodels.PrivateKey, ips sets.String) sets.String {
	for _, privateKey := range privateKeys {
		ips.Insert(privateKey.IP)
	}
	return ips
}

func getRenderedYaml(args *YamlValidatorReq) string {
	if len(args.VariableYaml) == 0 {
		return args.Yaml
	}
	// yaml with go template grammar, yaml should be rendered with variable yaml
	tmpl, err := gotemplate.New(fmt.Sprintf("%v", time.Now().Unix())).Parse(args.Yaml)
	if err != nil {
		log.Errorf("failed to parse as go template, err: %s", err)
		return args.Yaml
	}

	variableMap := make(map[string]interface{})
	err = yaml.Unmarshal([]byte(args.VariableYaml), &variableMap)
	if err != nil {
		log.Errorf("failed to get variable map, err: %s", err)
		return args.Yaml
	}

	buf := bytes.NewBufferString("")
	err = tmpl.Execute(buf, variableMap)
	if err != nil {
		log.Errorf("failed to execute template render, err: %s", err)
		return args.Yaml
	}
	return buf.String()
}

func YamlValidator(args *YamlValidatorReq) []string {
	errorDetails := make([]string, 0)
	if args.Yaml == "" {
		return errorDetails
	}
	yamlContent := util.ReplaceWrapLine(args.Yaml)
	yamlContent = getRenderedYaml(args)

	KubeYamls := SplitYaml(yamlContent)
	for _, data := range KubeYamls {
		yamlDataArray := SplitYaml(data)
		for _, yamlData := range yamlDataArray {
			//验证格式
			resKind := new(KubeResourceKind)
			//在Unmarshal之前填充渲染变量{{.}}
			yamlData = config.RenderTemplateAlias.ReplaceAllLiteralString(yamlData, "ssssssss")
			// replace $Service$ with service name
			yamlData = config.ServiceNameAlias.ReplaceAllLiteralString(yamlData, args.ServiceName)

			if err := yaml.Unmarshal([]byte(yamlData), &resKind); err != nil {
				// if this yaml contains go template grammar, the validation will be passed
				ot := template.New(args.ServiceName)
				_, errTemplate := ot.Parse(yamlData)
				if errTemplate == nil {
					continue
				} else {
					log.Errorf("failed to pase as template, err: %s", errTemplate)
				}
				errorDetails = append(errorDetails, fmt.Sprintf("Invalid yaml format. The content must be a series of valid Kubernetes resources. err: %s", err))
			}
		}
	}
	return errorDetails
}

// Deprecated
func YamlViewServiceTemplate(args *YamlViewServiceTemplateReq) (string, error) {
	opt := &commonrepo.ServiceFindOption{
		ServiceName:   args.ServiceName,
		ProductName:   args.ProjectName,
		ExcludeStatus: setting.ProductStatusDeleting,
	}
	svcTmpl, err := commonrepo.NewServiceColl().Find(opt)
	if err != nil {
		return "", err
	}

	renderSet := new(commonmodels.RenderSet)
	//renderSet.KVs = args.Variables
	//parsedYaml := commonservice.RenderValueForString(svcTmpl.Yaml, renderSet)
	parsedYaml, err := kube.RenderServiceYaml(svcTmpl.Yaml, args.ProjectName, svcTmpl.ServiceName, renderSet, svcTmpl.ServiceVars, svcTmpl.VariableYaml)
	if err != nil {
		log.Errorf("failed to render service yaml, err: %s", err)
		return "", err
	}

	if args.EnvName != "" {
		prod, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
			Name:    args.ProjectName,
			EnvName: args.EnvName,
		})
		if err != nil {
			return "", err
		}

		parsedYaml = kube.ParseSysKeys(prod.Namespace, prod.EnvName, prod.ProductName, args.ServiceName, parsedYaml)
		cerSvc := prod.GetServiceMap()
		svcInfo, found := cerSvc[args.ServiceName]
		if found {
			parsedYaml, err = replaceContainerImages(parsedYaml, svcTmpl.Containers, svcInfo.Containers)
			if err != nil {
				return "", err
			}
		}
	}

	return parsedYaml, nil
}

func replaceContainerImages(tmpl string, ori []*commonmodels.Container, replace []*commonmodels.Container) (string, error) {
	replaceMap := make(map[string]string)
	for _, container := range replace {
		replaceMap[container.Name] = container.Image
	}

	for _, container := range ori {
		imageRex, err := regexp.Compile("image:\\s*" + container.Image)
		if err != nil {
			return "", err
		}
		if _, ok := replaceMap[container.Name]; !ok {
			continue
		}
		tmpl = imageRex.ReplaceAllLiteralString(tmpl, fmt.Sprintf("image: %s", replaceMap[container.Name]))
	}

	return tmpl, nil
}

func UpdateReleaseNamingRule(userName, requestID, projectName string, args *ReleaseNamingRule, log *zap.SugaredLogger) error {
	serviceTemplate, err := commonrepo.NewServiceColl().Find(&commonrepo.ServiceFindOption{
		ServiceName:   args.ServiceName,
		Revision:      0,
		Type:          setting.HelmDeployType,
		ProductName:   projectName,
		ExcludeStatus: setting.ProductStatusDeleting,
	})
	if err != nil {
		return err
	}

	// check if namings rule changes for services deployed in envs
	if serviceTemplate.GetReleaseNaming() == args.NamingRule {
		products, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
			Name: projectName,
		})
		if err != nil {
			return fmt.Errorf("failed to list envs for product: %s, err: %s", projectName, err)
		}
		modified := false
		for _, product := range products {
			if pSvc, ok := product.GetServiceMap()[args.ServiceName]; ok && pSvc.Revision != serviceTemplate.Revision {
				modified = true
				break
			}
		}
		if !modified {
			return nil
		}
	}

	serviceTemplate.ReleaseNaming = args.NamingRule
	rev, err := getNextServiceRevision(projectName, args.ServiceName)
	if err != nil {
		return fmt.Errorf("failed to get service next revision, service %s, err: %s", args.ServiceName, err)
	}

	basePath := config.LocalServicePath(serviceTemplate.ProductName, serviceTemplate.ServiceName)
	if err = commonservice.PreLoadServiceManifests(basePath, serviceTemplate); err != nil {
		return fmt.Errorf("failed to load chart info for service %s, err: %s", serviceTemplate.ServiceName, err)
	}

	fsTree := os.DirFS(config.LocalServicePath(projectName, serviceTemplate.ServiceName))
	s3Base := config.ObjectStorageServicePath(projectName, serviceTemplate.ServiceName)
	err = fs.ArchiveAndUploadFilesToS3(fsTree, []string{fmt.Sprintf("%s-%d", serviceTemplate.ServiceName, rev)}, s3Base, log)
	if err != nil {
		return fmt.Errorf("failed to upload chart info for service %s, err: %s", serviceTemplate.ServiceName, err)
	}

	serviceTemplate.Revision = rev
	err = commonrepo.NewServiceColl().Create(serviceTemplate)
	if err != nil {
		return fmt.Errorf("failed to update relase naming for service: %s, err: %s", args.ServiceName, err)
	}

	go func() {
		// reinstall services in envs
		err = service.ReInstallHelmSvcInAllEnvs(projectName, serviceTemplate)
		if err != nil {
			title := fmt.Sprintf("服务 [%s] 重建失败", args.ServiceName)
			commonservice.SendErrorMessage(userName, title, requestID, err, log)
		}
	}()

	return nil
}

func DeleteServiceTemplate(serviceName, serviceType, productName, isEnvTemplate, visibility string, log *zap.SugaredLogger) error {
	openEnvTemplate, _ := strconv.ParseBool(isEnvTemplate)
	if openEnvTemplate && visibility == "public" {
		projects, err := templaterepo.NewProductColl().ListWithOption(
			&templaterepo.ProductListOpt{ContainSharedServices: []*templatemodels.ServiceInfo{{Name: serviceName, Owner: productName}}},
		)
		if err != nil {
			log.Errorf("Failed to list projects which are using service %s, err: %s", serviceName, err)
			return err
		}
		if len(projects) > 0 {
			var names []string
			for _, p := range projects {
				names = append(names, p.ProductName)
			}
			log.Warnf("service %s is used by projects %s", serviceName, strings.Join(names, ","))
			errMsg := fmt.Sprintf("共享服务 [%s] 已被 [%s] 等项目服务编排中使用，请解除引用后再删除", serviceName, strings.Join(names, ","))
			return e.ErrInvalidParam.AddDesc(errMsg)
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

	// 如果服务是PM类型，删除服务更新build的target信息
	if serviceType == setting.PMDeployType {
		if serviceTmpl, err := commonservice.GetServiceTemplate(
			serviceName, setting.PMDeployType, productName, setting.ProductStatusDeleting, 0, log,
		); err == nil {
			if serviceTmpl.BuildName != "" {
				updateTargets := make([]*commonmodels.ServiceModuleTarget, 0)
				if preBuild, err := commonrepo.NewBuildColl().Find(&commonrepo.BuildFindOption{Name: serviceTmpl.BuildName, ProductName: productName}); err == nil {
					for _, target := range preBuild.Targets {
						if target.ServiceName != serviceName {
							updateTargets = append(updateTargets, target)
						}
					}
					_ = commonrepo.NewBuildColl().UpdateTargets(serviceTmpl.BuildName, productName, updateTargets)
				}
			}
		}
	}

	err := commonrepo.NewServiceColl().UpdateStatus(serviceName, productName, setting.ProductStatusDeleting)
	if err != nil {
		errMsg := fmt.Sprintf("[service.UpdateStatus] %s-%s error: %v", serviceName, serviceType, err)
		log.Error(errMsg)
		return e.ErrDeleteTemplate.AddDesc(errMsg)
	}

	if serviceType == setting.HelmDeployType {
		// 更新helm renderset
		err = removeServiceFromRenderset(productName, productName, "", serviceName)
		if err != nil {
			log.Warnf("failed to update renderset: %s when deleting service: %s, err: %s", productName, serviceName, err.Error())
		}

		// 把该服务相关的s3的数据从仓库删除
		if err = fs.DeleteArchivedFileFromS3([]string{serviceName}, configbase.ObjectStorageServicePath(productName, serviceName), log); err != nil {
			log.Warnf("Failed to delete file %s, err: %s", serviceName, err)
		}
		//onBoarding流程时，需要删除预定的renderset中的已经存在的renderchart信息
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

		// still onBoarding, need to delete service from renderset
		if serviceType == setting.HelmDeployType && productTempl.OnboardingStatus != 0 {
			envNames := []string{"dev", "qa"}
			for _, envName := range envNames {
				rendersetName := commonservice.GetProductEnvNamespace(envName, productName, "")
				err := removeServiceFromRenderset(productName, rendersetName, envName, serviceName)
				if err != nil {
					log.Warnf("failed to update renderset: %s when deleting service: %s, err: %s", rendersetName, serviceName, err.Error())
				}
			}
		}
	}

	commonservice.DeleteServiceWebhookByName(serviceName, productName, log)

	return nil
}

// remove specific services from rendersets.chartinfos
func removeServiceFromRenderset(productName, renderName, envName, serviceName string) error {
	renderOpt := &commonrepo.RenderSetFindOption{Name: renderName, ProductTmpl: productName, EnvName: envName}
	if rs, err := commonrepo.NewRenderSetColl().Find(renderOpt); err == nil {
		chartInfos := make([]*templatemodels.ServiceRender, 0)
		for _, chartInfo := range rs.ChartInfos {
			if chartInfo.ServiceName == serviceName {
				continue
			}
			chartInfos = append(chartInfos, chartInfo)
		}
		rs.ChartInfos = chartInfos
		err = commonrepo.NewRenderSetColl().Update(rs)
		if err != nil {
			return err
		}
	}
	return nil
}

func ListServicePort(serviceName, serviceType, productName, excludeStatus string, revision int64, log *zap.SugaredLogger) ([]int, error) {
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

type svcInfo struct {
	ServiceName string `json:"service_name"`
	ProductName string `json:"product_name"`
}

// ListAvailablePublicServices returns all public services which are not shared in the given project.
func ListAvailablePublicServices(productName string, log *zap.SugaredLogger) ([]*svcInfo, error) {
	project, err := templaterepo.NewProductColl().Find(productName)
	if err != nil {
		log.Errorf("Can not find project %s, error: %s", productName, err)
		return nil, e.ErrListTemplate.AddDesc(err.Error())
	}

	services, err := commonrepo.NewServiceColl().ListMaxRevisions(&commonrepo.ServiceListOption{
		Visibility: setting.PublicService, ExcludeProject: productName,
		NotInServices: project.SharedServices,
	})
	if err != nil {
		log.Errorf("Can not list services, error: %s", err)
		return nil, e.ErrListTemplate.AddDesc(err.Error())
	}

	var res []*svcInfo

	for _, s := range services {
		res = append(res, &svcInfo{
			ServiceName: s.ServiceName,
			ProductName: s.ProductName,
		})
	}

	return res, nil
}

func GetGerritServiceYaml(args *commonmodels.Service, log *zap.SugaredLogger) error {
	//同步commit信息
	codehostDetail, err := syncGerritLatestCommit(args)
	if err != nil {
		log.Errorf("Sync change log from gerrit failed, error: %v", err)
		return err
	}

	base := path.Join(config.S3StoragePath(), args.GerritRepoName)
	if _, err := os.Stat(base); os.IsNotExist(err) {
		err = command.RunGitCmds(codehostDetail, setting.GerritDefaultOwner, setting.GerritDefaultOwner, args.GerritRepoName, args.GerritBranchName, args.GerritRemoteName)
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

func ensureServiceTmpl(userName string, args *commonmodels.Service, log *zap.SugaredLogger) error {
	if args == nil {
		return errors.New("service template arg is null")
	}
	if len(args.ServiceName) == 0 {
		return errors.New("service name is empty")
	}
	if !config.ServiceNameRegex.MatchString(args.ServiceName) {
		return fmt.Errorf("导入的文件目录和文件名称仅支持字母，数字，中划线和下划线")
	}
	args.CreateBy = userName
	if args.Type == setting.K8SDeployType {
		if args.Containers == nil {
			args.Containers = make([]*commonmodels.Container, 0)
		}
		if len(args.RenderedYaml) == 0 {
			args.RenderedYaml = args.Yaml
		}

		var err error
		args.RenderedYaml, err = renderK8sSvcYaml(args.RenderedYaml, args.ProductName, args.ServiceName, args.VariableYaml)
		if err != nil {
			return fmt.Errorf("failed to render yaml, err: %s", err)
		}

		// Only the gerrit/spock/external type needs to be processed by yaml
		if args.Source == setting.SourceFromGerrit || args.Source == setting.SourceFromZadig || args.Source == setting.SourceFromExternal || args.Source == setting.ServiceSourceTemplate || args.Source == setting.SourceFromGitee {
			// 拆分 all-in-one yaml文件
			// 替换分隔符
			args.Yaml = util.ReplaceWrapLine(args.Yaml)
			args.RenderedYaml = util.ReplaceWrapLine(args.RenderedYaml)

			// 分隔符为\n---\n
			args.KubeYamls = SplitYaml(args.RenderedYaml)
		}

		// since service may contain go-template grammar, errors may occur when parsing as k8s workloads
		// errors will only be logged here
		if err := setCurrentContainerImages(args); err != nil {
			log.Errorf("failed to ser set container images, err: %s", err)
			//return err
		}
		log.Infof("find %d containers in service %s", len(args.Containers), args.ServiceName)
	}

	// 设置新的版本号
	serviceTemplate := fmt.Sprintf(setting.ServiceTemplateCounterName, args.ServiceName, args.ProductName)
	rev, err := commonrepo.NewCounterColl().GetNextSeq(serviceTemplate)
	if err != nil {
		return fmt.Errorf("get next service template revision error: %v", err)
	}

	args.Revision = rev
	return nil
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
		for index, yamlData := range yamlDataArray {
			resKind := new(KubeResourceKind)
			//在Unmarshal之前填充渲染变量{{.}}
			yamlData = config.RenderTemplateAlias.ReplaceAllLiteralString(yamlData, "ssssssss")
			// replace $Service$ with service name
			yamlData = config.ServiceNameAlias.ReplaceAllLiteralString(yamlData, args.ServiceName)
			// replace $Product$ with product name
			yamlData = config.ProductNameAlias.ReplaceAllLiteralString(yamlData, args.ProductName)

			if err := yaml.Unmarshal([]byte(yamlData), &resKind); err != nil {
				return fmt.Errorf("unmarshal ResourceKind error: %v", err)
			}

			if resKind == nil {
				if index == 0 {
					continue
				}
				return errors.New("nil Resource Kind")
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

	args.Containers = uniqueSlice(srvContainers)
	return nil
}

func getContainers(data string) ([]*commonmodels.Container, error) {
	containers := make([]*commonmodels.Container, 0)

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
			Name:      nameStr,
			Image:     imageStr,
			ImageName: util.ExtractImageName(imageStr),
		}

		containers = append(containers, container)
	}

	return containers, nil
}

func getCronJobContainers(data string) ([]*commonmodels.Container, error) {
	containers := make([]*commonmodels.Container, 0)

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
			Name:      nameStr,
			Image:     imageStr,
			ImageName: util.ExtractImageName(imageStr),
		}

		containers = append(containers, container)
	}

	return containers, nil
}

func updateGerritWebhookByService(lastService, currentService *commonmodels.Service) error {
	var codehostDetail *systemconfig.CodeHost
	var err error
	if lastService != nil && lastService.Source == setting.SourceFromGerrit {
		codehostDetail, err = systemconfig.New().GetCodeHost(lastService.GerritCodeHostID)
		if err != nil {
			log.Errorf("updateGerritWebhookByService GetCodehostDetail err:%v", err)
			return err
		}
		cl := gerrit.NewHTTPClient(codehostDetail.Address, codehostDetail.AccessToken)
		webhookURLPrefix := fmt.Sprintf("/%s/%s/%s", "a/config/server/webhooks~projects", gerrit.Escape(lastService.GerritRepoName), "remotes")
		_, _ = cl.Delete(fmt.Sprintf("%s/%s", webhookURLPrefix, gerrit.RemoteName))
		_, err = cl.Delete(fmt.Sprintf("%s/%s", webhookURLPrefix, lastService.ServiceName))
		if err != nil {
			log.Errorf("updateGerritWebhookByService deleteGerritWebhook err:%v", err)
		}
	}
	var detail *systemconfig.CodeHost
	if lastService.GerritCodeHostID == currentService.GerritCodeHostID && codehostDetail != nil {
		detail = codehostDetail
	} else {
		detail, _ = systemconfig.New().GetCodeHost(currentService.GerritCodeHostID)
	}

	cl := gerrit.NewHTTPClient(detail.Address, detail.AccessToken)
	webhookURL := fmt.Sprintf("/%s/%s/%s/%s", "a/config/server/webhooks~projects", gerrit.Escape(currentService.GerritRepoName), "remotes", currentService.ServiceName)
	if _, err := cl.Get(webhookURL); err != nil {
		log.Errorf("updateGerritWebhookByService getGerritWebhook err:%v", err)
		//创建webhook
		gerritWebhook := &gerrit.Webhook{
			URL:       fmt.Sprintf("%s?name=%s", config.WebHookURL(), currentService.ServiceName),
			MaxTries:  setting.MaxTries,
			SslVerify: false,
		}
		_, err = cl.Put(webhookURL, httpclient.SetBody(gerritWebhook))
		if err != nil {
			log.Errorf("updateGerritWebhookByService addGerritWebhook err:%v", err)
			return err
		}
	}
	return nil
}

func createGerritWebhookByService(codehostID int, serviceName, repoName, branchName string) error {
	detail, err := systemconfig.New().GetCodeHost(codehostID)
	if err != nil {
		log.Errorf("createGerritWebhookByService GetCodehostDetail err:%v", err)
		return err
	}

	cl := gerrit.NewHTTPClient(detail.Address, detail.AccessToken)
	webhookURL := fmt.Sprintf("/%s/%s/%s/%s", "a/config/server/webhooks~projects", gerrit.Escape(repoName), "remotes", serviceName)
	if _, err := cl.Get(webhookURL); err != nil {
		log.Errorf("createGerritWebhookByService getGerritWebhook err:%v", err)
		//创建webhook
		gerritWebhook := &gerrit.Webhook{
			URL:       fmt.Sprintf("%s?name=%s", config.WebHookURL(), serviceName),
			MaxTries:  setting.MaxTries,
			SslVerify: false,
		}
		_, err = cl.Put(webhookURL, httpclient.SetBody(gerritWebhook))
		if err != nil {
			log.Errorf("createGerritWebhookByService addGerritWebhook err:%v", err)
			return err
		}
	}
	return nil
}

func syncGerritLatestCommit(service *commonmodels.Service) (*systemconfig.CodeHost, error) {
	if service.GerritCodeHostID == 0 {
		return nil, fmt.Errorf("codehostId不能是空的")
	}
	if service.GerritRepoName == "" {
		return nil, fmt.Errorf("repoName不能是空的")
	}
	if service.GerritBranchName == "" {
		return nil, fmt.Errorf("branchName不能是空的")
	}
	ch, _ := systemconfig.New().GetCodeHost(service.GerritCodeHostID)

	gerritCli := gerrit.NewClient(ch.Address, ch.AccessToken, config.ProxyHTTPSAddr(), ch.EnableProxy)
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

func uniqueSlice(elements []*commonmodels.Container) []*commonmodels.Container {
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
