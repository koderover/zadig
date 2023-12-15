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
	"os"
	"strings"
	"sync"
	"text/template"
	gotemplate "text/template"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/yaml"

	configbase "github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	templatemodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/fs"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/notify"
	commontypes "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/types"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/environment/service"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/systemconfig"
	kubeclient "github.com/koderover/zadig/v2/pkg/shared/kube/client"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/gerrit"
	"github.com/koderover/zadig/v2/pkg/tool/httpclient"
	"github.com/koderover/zadig/v2/pkg/tool/kube/getter"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/util"
	yamlutil "github.com/koderover/zadig/v2/pkg/util/yaml"
)

type ServiceOption struct {
	ServiceModules     []*ServiceModule                 `json:"service_module"`
	SystemVariable     []*Variable                      `json:"system_variable"`
	VariableYaml       string                           `json:"variable_yaml"`
	ServiceVariableKVs []*commontypes.ServiceVariableKV `json:"service_variable_kvs"`
	Yaml               string                           `json:"yaml"`
	Service            *commonmodels.Service            `json:"service,omitempty"`
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

	serviceOption.VariableYaml = args.VariableYaml
	serviceOption.ServiceVariableKVs = args.ServiceVariableKVs

	if args.Source == setting.SourceFromGitlab || args.Source == setting.SourceFromGithub ||
		args.Source == setting.SourceFromGerrit || args.Source == setting.SourceFromGitee {
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

	clientset, err := kubeclient.GetClientset(config.HubServerAddress(), args.ClusterID)
	if err != nil {
		log.Errorf("get client set error: %v", err)
		return err
	}
	versionInfo, err := clientset.Discovery().ServerVersion()
	if err != nil {
		log.Errorf("get server version error: %v", err)
		return err
	}

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

	session := mongotool.Session()
	defer session.EndSession(context.TODO())

	serviceInExternalEnvCol := commonrepo.NewServiceInExternalEnvWithSess(session)
	productCol := commonrepo.NewProductCollWithSession(session)
	workloadStatCol := commonrepo.NewWorkLoadsStatCollWithSession(session)

	mongotool.StartTransaction(session)

	g := new(errgroup.Group)
	for _, workload := range args.WorkLoads {
		tempWorkload := workload
		g.Go(func() error {
			// If the service is already included in the database service template, add it to the new association table
			if serviceString.Has(tempWorkload.Name) {
				return serviceInExternalEnvCol.Create(&commonmodels.ServicesInExternalEnv{
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
				bs, _, err = getter.GetDeploymentYamlFormat(args.Namespace, tempWorkload.Name, kubeClient)
			case setting.StatefulSet:
				bs, _, err = getter.GetStatefulSetYamlFormat(args.Namespace, tempWorkload.Name, kubeClient)
			case setting.CronJob:
				bs, _, err = getter.GetCronJobYamlFormat(args.Namespace, tempWorkload.Name, kubeClient, service.VersionLessThan121(versionInfo))
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
			}, session, log)
		})
	}
	if err := g.Wait(); err != nil {
		mongotool.AbortTransaction(session)
		return err
	}

	// 没有环境，创建环境
	if _, err = productCol.Find(&commonrepo.ProductFindOptions{
		Name:    args.ProductName,
		EnvName: args.EnvName,
	}); err != nil {
		if err := service.CreateProduct(userName, requestID, &service.ProductCreateArg{&commonmodels.Product{
			ProductName: args.ProductName,
			Source:      setting.SourceFromExternal,
			ClusterID:   args.ClusterID,
			RegistryID:  args.RegistryID,
			EnvName:     args.EnvName,
			Namespace:   args.Namespace,
			UpdateBy:    userName,
			IsExisted:   true,
		}, session}, log); err != nil {
			mongotool.AbortTransaction(session)
			return e.ErrCreateProduct.AddDesc("create product Error for unknown reason")
		}
	}

	workLoadStat, err := workloadStatCol.Find(args.ClusterID, args.Namespace)
	if err != nil {
		workLoadStat = &commonmodels.WorkloadStat{
			ClusterID: args.ClusterID,
			Namespace: args.Namespace,
			Workloads: workloadsTmp,
		}
		err = workloadStatCol.Create(workLoadStat)
		if err != nil {
			mongotool.AbortTransaction(session)
			return e.ErrCreateProduct.AddErr(err)
		}
	} else {
		workLoadStat.Workloads = replaceWorkloads(workLoadStat.Workloads, workloadsTmp, args.EnvName)
		err = workloadStatCol.UpdateWorkloads(workLoadStat)
		if err != nil {
			mongotool.AbortTransaction(session)
			return e.ErrCreateProduct.AddErr(err)
		}
	}
	return mongotool.CommitTransaction(session)
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
	clientset, err := kubeclient.GetClientset(config.HubServerAddress(), args.ClusterID)
	if err != nil {
		log.Errorf("get client set error: %v", err)
		return err
	}
	versionInfo, err := clientset.Discovery().ServerVersion()
	if err != nil {
		log.Errorf("get server version error: %v", err)
		return err
	}

	session := mongotool.Session()
	defer session.EndSession(context.TODO())

	serviceInExternalEnvCol := commonrepo.NewServiceInExternalEnvWithSess(session)
	templateProductColl := templaterepo.NewProductCollWithSess(session)
	serviceColl := commonrepo.NewServiceCollWithSession(session)
	workloadStatCol := commonrepo.NewWorkLoadsStatCollWithSession(session)

	mongotool.StartTransaction(session)

	workloadStat, err := workloadStatCol.Find(args.ClusterID, args.Namespace)
	if err != nil {
		log.Errorf("[%s][%s]NewWorkLoadsStatColl().Find %s", args.ClusterID, args.Namespace, err)
		mongotool.AbortTransaction(session)
		return err
	}
	externalEnvServices, _ := serviceInExternalEnvCol.List(&commonrepo.ServicesInExternalEnvArgs{
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

	otherExternalEnvServices, err := serviceInExternalEnvCol.List(&commonrepo.ServicesInExternalEnvArgs{
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

	templateProductInfo, err := templateProductColl.Find(productName)
	if err != nil {
		log.Errorf("failed to find template product: %s error: %s", productName, err)
		mongotool.AbortTransaction(session)
		return err
	}

	svcNeedAdd := sets.NewString()
	svcNeedDelete := sets.NewString()

	for _, v := range diff {
		switch v.Operation {
		// 删除workload的引用
		case "delete":
			if externalService, isExist := externalEnvServiceM[v.Name]; !isExist {
				if err = serviceColl.UpdateExternalServicesStatus(v.Name, productName, setting.ProductStatusDeleting, envName); err != nil {
					log.Errorf("UpdateStatus external services error:%s", err)
				}
			} else {
				// Update the env name in the service
				if err = serviceColl.UpdateExternalServiceEnvName(v.Name, productName, externalService.EnvName); err != nil {
					log.Errorf("UpdateEnvName external services error:%s", err)
				}
				// Delete the reference in the original service
				if err = serviceInExternalEnvCol.Delete(&commonrepo.ServicesInExternalEnvArgs{
					ProductName: externalService.ProductName,
					EnvName:     externalService.EnvName,
					ServiceName: externalService.ServiceName,
				}); err != nil {
					log.Errorf("delete service in external env envName:%s error:%s", externalService.EnvName, err)
				}
			}
			if err = serviceInExternalEnvCol.Delete(&commonrepo.ServicesInExternalEnvArgs{
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
				bs, _, err = getter.GetDeploymentYamlFormat(args.Namespace, v.Name, kubeClient)
			case setting.StatefulSet:
				bs, _, err = getter.GetStatefulSetYamlFormat(args.Namespace, v.Name, kubeClient)
			case setting.CronJob:
				bs, _, err = getter.GetCronJobYamlFormat(args.Namespace, v.Name, kubeClient, service.VersionLessThan121(versionInfo))
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
			}, session, log); err != nil {
				log.Errorf("create service template failed err:%v", err)
				delete(diff, v.Name)
				continue
			}
		}
	}

	// for host services, services are stored in template_product.services[0]
	if len(templateProductInfo.Services) == 1 {

		func() {
			productServices, err := serviceColl.ListExternalWorkloadsBy(productName, "")
			if err != nil {
				log.Errorf("ListWorkloadDetails ListExternalServicesBy err:%s", err)
				return
			}
			productServiceNames := sets.NewString()
			for _, productService := range productServices {
				productServiceNames.Insert(productService.ServiceName)
			}
			// add services in external env data
			servicesInExternalEnv, _ := serviceInExternalEnvCol.List(&commonrepo.ServicesInExternalEnvArgs{
				ProductName: productName,
			})
			for _, serviceInExternalEnv := range servicesInExternalEnv {
				productServiceNames.Insert(serviceInExternalEnv.ServiceName)
			}

			templateProductInfo.Services[0] = productServiceNames.List()
			err = templateProductColl.UpdateServiceOrchestration(templateProductInfo.ProductName, templateProductInfo.Services, templateProductInfo.UpdateBy)
			if err != nil {
				log.Errorf("failed to update service for product: %s, err: %s", templateProductInfo.ProductName, err)
			}
		}()

	}

	// 删除 && 增加
	workloadStat.Workloads = updateWorkloads(workloadStat.Workloads, diff, envName, productName)
	err = workloadStatCol.UpdateWorkloads(workloadStat)
	if err != nil {
		mongotool.AbortTransaction(session)
		return err
	}
	return mongotool.CommitTransaction(session)
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
func CreateWorkloadTemplate(userName string, args *commonmodels.Service, session mongo.Session, log *zap.SugaredLogger) error {
	_, err := templaterepo.NewProductColl().Find(args.ProductName)
	if err != nil {
		log.Errorf("Failed to find project %s, err: %s", args.ProductName, err)
		return e.ErrInvalidParam.AddErr(err)
	}
	// 遍历args.KubeYamls，获取 Deployment 或者 StatefulSet 里面所有containers 镜像和名称
	args.KubeYamls = []string{args.Yaml}
	if err := commonutil.SetCurrentContainerImages(args); err != nil {
		log.Errorf("Failed tosetCurrentContainerImages %s, err: %s", args.ProductName, err)
		return err
	}
	opt := &commonrepo.ServiceFindOption{
		ServiceName:   args.ServiceName,
		ProductName:   args.ProductName,
		ExcludeStatus: setting.ProductStatusDeleting,
	}

	serviceColl := commonrepo.NewServiceCollWithSession(session)

	_, notFoundErr := serviceColl.Find(opt)
	if notFoundErr != nil {
		if productTempl, err := commonservice.GetProductTemplate(args.ProductName, log); err == nil {
			//获取项目里面的所有服务
			if len(productTempl.Services) > 0 && !sets.NewString(productTempl.Services[0]...).Has(args.ServiceName) {
				productTempl.Services[0] = append(productTempl.Services[0], args.ServiceName)
			} else if len(productTempl.Services) == 0 {
				productTempl.Services = [][]string{{args.ServiceName}}
			}
			//更新项目模板
			err = templaterepo.NewProductCollWithSess(session).Update(args.ProductName, productTempl)
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
		return commonrepo.NewServiceInExternalEnvWithSess(session).Create(&commonmodels.ServicesInExternalEnv{
			ProductName: args.ProductName,
			ServiceName: args.ServiceName,
			EnvName:     args.EnvName,
			Namespace:   product.Namespace,
			ClusterID:   product.ClusterID,
		})
		//return e.ErrCreateTemplate.AddDesc("do not support import same service name")
	}

	if err := serviceColl.Delete(args.ServiceName, args.Type, args.ProductName, setting.ProductStatusDeleting, 0); err != nil {
		log.Errorf("ServiceTmpl.delete %s error: %v", args.ServiceName, err)
	}

	if err := serviceColl.Create(args); err != nil {
		log.Errorf("ServiceTmpl.Create %s error: %v", args.ServiceName, err)
		return e.ErrCreateTemplate.AddDesc(err.Error())
	}
	return nil
}

// fillServiceVariable fill and merge service.variableYaml and service.serviceVariableKVs by the previous revision
func fillServiceVariable(args *commonmodels.Service, curRevision *commonmodels.Service) error {
	if args.Source == setting.ServiceSourceTemplate {
		return nil
	}

	extractVariableYaml, err := yamlutil.ExtractVariableYaml(args.Yaml)
	if err != nil {
		return fmt.Errorf("failed to extract variable yaml from service yaml, err: %w", err)
	}
	extractServiceVariableKVs, err := commontypes.YamlToServiceVariableKV(extractVariableYaml, nil)
	if err != nil {
		return fmt.Errorf("failed to convert variable yaml to service variable kv, err: %w", err)
	}

	if args.Source == setting.SourceFromZadig {
		args.VariableYaml, args.ServiceVariableKVs, err = commontypes.MergeServiceVariableKVsIfNotExist(args.ServiceVariableKVs, extractServiceVariableKVs)
		if err != nil {
			return fmt.Errorf("failed to merge service variables, err %w", err)
		}
	} else if curRevision != nil {
		args.VariableYaml, args.ServiceVariableKVs, err = commontypes.MergeServiceVariableKVsIfNotExist(curRevision.ServiceVariableKVs, extractServiceVariableKVs)
		if err != nil {
			return fmt.Errorf("failed to merge service variables, err %w", err)
		}
	} else {
		args.VariableYaml = extractVariableYaml
		args.ServiceVariableKVs = extractServiceVariableKVs
	}

	return nil
}

func CreateServiceTemplate(userName string, args *commonmodels.Service, force bool, log *zap.SugaredLogger) (*ServiceOption, error) {
	opt := &commonrepo.ServiceFindOption{
		ServiceName:   args.ServiceName,
		Revision:      0,
		Type:          args.Type,
		ProductName:   args.ProductName,
		ExcludeStatus: setting.ProductStatusDeleting,
	}

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

	// fill serviceVars and variableYaml and serviceVariableKVs
	err := fillServiceVariable(args, serviceTmpl)
	if err != nil {
		return nil, fmt.Errorf("failed to fill service variable, err: %w", err)
	}

	// 校验args
	if err := ensureServiceTmpl(userName, args, log); err != nil {
		log.Errorf("ensureServiceTmpl error: %+v", err)
		return nil, e.ErrValidateTemplate.AddDesc(err.Error())
	}

	if err := commonrepo.NewServiceColl().Delete(args.ServiceName, args.Type, args.ProductName, setting.ProductStatusDeleting, args.Revision); err != nil {
		log.Errorf("ServiceTmpl.delete %s error: %v", args.ServiceName, err)
	}

	// create a new revision of template service
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

func CreateProductionServiceTemplate(userName string, args *commonmodels.Service, force bool, log *zap.SugaredLogger) (*ServiceOption, error) {
	opt := &commonrepo.ServiceFindOption{
		ServiceName:   args.ServiceName,
		Revision:      0,
		Type:          args.Type,
		ProductName:   args.ProductName,
		ExcludeStatus: setting.ProductStatusDeleting,
	}

	// Check for completely duplicate items before updating the database, and if so, exit.
	serviceTmpl, notFoundErr := commonrepo.NewProductionServiceColl().Find(opt)
	if notFoundErr == nil && !force {
		return nil, fmt.Errorf("production_service:%s already exists", serviceTmpl.ServiceName)
	} else {
		if args.Source == setting.SourceFromGerrit {
			// create gerrit webhook
			if err := createGerritWebhookByService(args.GerritCodeHostID, args.ServiceName, args.GerritRepoName, args.GerritBranchName); err != nil {
				log.Errorf("createGerritWebhookByService error: %v", err)
				return nil, err
			}
		}
	}

	// fill serviceVars and variableYaml and serviceVariableKVs
	err := fillServiceVariable(args, serviceTmpl)
	if err != nil {
		return nil, err
	}

	// check args
	args.Production = true
	if err := ensureServiceTmpl(userName, args, log); err != nil {
		log.Errorf("ensureProductionServiceTmpl error: %+v", err)
		return nil, e.ErrValidateTemplate.AddDesc(err.Error())
	}

	if err := commonrepo.NewProductionServiceColl().DeleteByOptions(commonrepo.ProductionServiceDeleteOption{
		ServiceName: args.ServiceName,
		Type:        args.Type,
		ProductName: args.ProductName,
		Revision:    args.Revision,
		Status:      setting.ProductStatusDeleting,
	}); err != nil {
		log.Errorf("ProductionServiceTmpl.delete %s error: %v", args.ServiceName, err)
	}

	// create a new revision of template service
	if err := commonrepo.NewProductionServiceColl().Create(args); err != nil {
		log.Errorf("ProductionServiceTmpl.Create %s error: %v", args.ServiceName, err)
		return nil, e.ErrCreateTemplate.AddDesc(err.Error())
	}

	if notFoundErr != nil {
		if productTempl, err := commonservice.GetProductTemplate(args.ProductName, log); err == nil {
			// get all services in the project
			if len(productTempl.ProductionServices) > 0 && !sets.NewString(productTempl.ProductionServices[0]...).Has(args.ServiceName) {
				productTempl.ProductionServices[0] = append(productTempl.ProductionServices[0], args.ServiceName)
			} else {
				productTempl.ProductionServices = [][]string{{args.ServiceName}}
			}
			// update project template
			err = templaterepo.NewProductColl().Update(args.ProductName, productTempl)
			if err != nil {
				log.Errorf("CreateProductionServiceTemplate Update %s error: %s", args.ServiceName, err)
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

//func UpdateServiceVisibility(args *commonservice.ServiceTmplObject) error {
//	currentService, err := commonrepo.NewServiceColl().Find(&commonrepo.ServiceFindOption{
//		ProductName: args.ProductName,
//		ServiceName: args.ServiceName,
//		Revision:    args.Revision,
//	})
//	if err != nil {
//		log.Errorf("Can not find service with option %+v. Error: %s", args, err)
//		return err
//	}
//
//	envStatuses := make([]*commonmodels.EnvStatus, 0)
//	// Remove environments and hosts that do not exist in the check status
//	for _, envStatus := range args.EnvStatuses {
//		var existEnv, existHost bool
//
//		envName := envStatus.EnvName
//		if _, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
//			Name:    args.ProductName,
//			EnvName: envName,
//		}); err == nil {
//			existEnv = true
//		}
//
//		op := commonrepo.FindPrivateKeyOption{
//			Address: envStatus.Address,
//			ID:      envStatus.HostID,
//		}
//		if _, err := commonrepo.NewPrivateKeyColl().Find(op); err == nil {
//			existHost = true
//		}
//
//		if existEnv && existHost {
//			envStatuses = append(envStatuses, envStatus)
//		}
//	}
//
//	// for pm services, fill env status data
//	if args.Type == setting.PMDeployType {
//		validStatusMap := make(map[string]*commonmodels.EnvStatus)
//		for _, status := range envStatuses {
//			validStatusMap[fmt.Sprintf("%s-%s", status.EnvName, status.HostID)] = status
//		}
//
//		envStatus, err := pm.GenerateEnvStatus(currentService.EnvConfigs, log.SugaredLogger())
//		if err != nil {
//			log.Errorf("failed to generate env status")
//			return err
//		}
//		defaultStatusMap := make(map[string]*commonmodels.EnvStatus)
//		for _, status := range envStatus {
//			defaultStatusMap[fmt.Sprintf("%s-%s", status.EnvName, status.HostID)] = status
//		}
//
//		for k, _ := range defaultStatusMap {
//			if vv, ok := validStatusMap[k]; ok {
//				defaultStatusMap[k] = vv
//			}
//		}
//
//		envStatuses = make([]*commonmodels.EnvStatus, 0)
//		for _, v := range defaultStatusMap {
//			envStatuses = append(envStatuses, v)
//		}
//	}
//
//	updateArgs := &commonmodels.Service{
//		ProductName: args.ProductName,
//		ServiceName: args.ServiceName,
//		Revision:    args.Revision,
//		Type:        args.Type,
//		CreateBy:    args.Username,
//		EnvConfigs:  args.EnvConfigs,
//		EnvStatuses: envStatuses,
//	}
//	return commonrepo.NewServiceColl().Update(updateArgs)
//}

func containersChanged(oldContainers []*commonmodels.Container, newContainers []*commonmodels.Container) bool {
	if len(oldContainers) != len(newContainers) {
		return true
	}
	oldSet := sets.NewString()
	for _, container := range oldContainers {
		oldSet.Insert(fmt.Sprintf("%s-%s", container.Name, container.Image))
	}
	newSet := sets.NewString()
	for _, container := range newContainers {
		newSet.Insert(fmt.Sprintf("%s-%s", container.Name, container.Image))
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

	currentService.VariableYaml = args.VariableYaml
	currentService.ServiceVariableKVs = args.ServiceVariableKVs

	currentService.RenderedYaml, err = commonutil.RenderK8sSvcYamlStrict(currentService.Yaml, args.ProductName, args.ServiceName, currentService.VariableYaml)
	if err != nil {
		return fmt.Errorf("failed to render yaml, err: %s", err)
	}

	err = commonrepo.NewServiceColl().UpdateServiceVariables(currentService)
	if err != nil {
		return e.ErrUpdateService.AddErr(err)
	}

	currentService.RenderedYaml = util.ReplaceWrapLine(currentService.RenderedYaml)
	currentService.KubeYamls = util.SplitYaml(currentService.RenderedYaml)

	// reparse service, check if container changes
	oldContainers := currentService.Containers
	if err := commonutil.SetCurrentContainerImages(currentService); err != nil {
		log.Errorf("failed to ser set container images, err: %s", err)
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
		Revision:    args.Revision,
		Type:        args.Type,
		CreateBy:    args.Username,
		EnvConfigs:  changeEnvConfigs,
		EnvStatuses: changeEnvStatus,
	}
	return commonrepo.NewServiceColl().UpdateServiceHealthCheckStatus(updateArgs)
}

func getRenderedYaml(args *YamlValidatorReq) string {
	extractVariableYaml, err := yamlutil.ExtractVariableYaml(args.Yaml)
	if err != nil {
		log.Errorf("failed to extract variable yaml, err: %w", err)
		extractVariableYaml = ""
	}
	extractVariableKVs, err := commontypes.YamlToServiceVariableKV(extractVariableYaml, nil)
	if err != nil {
		log.Errorf("failed to convert extract variable yaml to kv, err: %w", err)
		extractVariableKVs = nil
	}
	argVariableKVs, err := commontypes.YamlToServiceVariableKV(args.VariableYaml, nil)
	if err != nil {
		log.Errorf("failed to convert arg variable yaml to kv, err: %w", err)
		argVariableKVs = nil
	}
	variableYaml, _, err := commontypes.MergeServiceVariableKVs(extractVariableKVs, argVariableKVs)
	if err != nil {
		log.Errorf("failed to merge extractVariableKVs and argVariableKVs variable kv, err: %w", err)
		return args.Yaml
	}

	// yaml with go template grammar, yaml should be rendered with variable yaml
	tmpl, err := gotemplate.New(fmt.Sprintf("%v", time.Now().Unix())).Parse(args.Yaml)
	if err != nil {
		log.Errorf("failed to parse as go template, err: %s", err)
		return args.Yaml
	}

	variableMap := make(map[string]interface{})
	err = yaml.Unmarshal([]byte(variableYaml), &variableMap)
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

	KubeYamls := util.SplitYaml(yamlContent)
	for _, data := range KubeYamls {
		yamlDataArray := util.SplitYaml(data)
		for _, yamlData := range yamlDataArray {
			//验证格式
			resKind := new(types.KubeResourceKind)
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
					log.Errorf("failed to parse as template, err: %s", errTemplate)
				}
				errorDetails = append(errorDetails, fmt.Sprintf("Invalid yaml format. The content must be a series of valid Kubernetes resources. err: %s", err))
			}
		}
	}
	return errorDetails
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

	products, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
		Name:       projectName,
		Production: util.GetBoolPointer(false),
	})
	if err != nil {
		return fmt.Errorf("failed to list envs for product: %s, err: %s", projectName, err)
	}

	// check if namings rule changes for services deployed in envs
	if serviceTemplate.GetReleaseNaming() == args.NamingRule {
		products, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
			Name:       projectName,
			Production: util.GetBoolPointer(false),
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

	// check if the release name already exists
	for _, product := range products {
		releaseName := util.GeneReleaseName(args.NamingRule, product.ProductName, product.Namespace, product.EnvName, args.ServiceName)
		releaseNameMap, err := commonutil.GetReleaseNameToChartNameMap(product)
		if err != nil {
			return fmt.Errorf("failed to get release name to chart name map, err: %s", err)
		}
		if chartOrSvcName, ok := releaseNameMap[releaseName]; ok && chartOrSvcName != args.ServiceName {
			return fmt.Errorf("release name %s already exists for chart or service %s in environment: %s", releaseName, chartOrSvcName, product.EnvName)
		}
	}

	serviceTemplate.ReleaseNaming = args.NamingRule
	rev, err := getNextServiceRevision(projectName, args.ServiceName, false)
	if err != nil {
		return fmt.Errorf("failed to get service next revision, service %s, err: %s", args.ServiceName, err)
	}

	basePath := config.LocalTestServicePath(serviceTemplate.ProductName, serviceTemplate.ServiceName)
	if err = commonutil.PreLoadServiceManifests(basePath, serviceTemplate, false); err != nil {
		return fmt.Errorf("failed to load chart info for service %s, err: %s", serviceTemplate.ServiceName, err)
	}

	fsTree := os.DirFS(config.LocalTestServicePath(projectName, serviceTemplate.ServiceName))
	s3Base := config.ObjectStorageTestServicePath(projectName, serviceTemplate.ServiceName)
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
			notify.SendErrorMessage(userName, title, requestID, err, log)
		}
	}()

	return nil
}

func UpdateProductionServiceReleaseNamingRule(userName, requestID, projectName string, args *ReleaseNamingRule, log *zap.SugaredLogger) error {
	serviceTemplate, err := commonrepo.NewProductionServiceColl().Find(&commonrepo.ServiceFindOption{
		ServiceName:   args.ServiceName,
		Revision:      0,
		Type:          setting.HelmDeployType,
		ProductName:   projectName,
		ExcludeStatus: setting.ProductStatusDeleting,
	})
	if err != nil {
		return err
	}

	products, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
		Name:       projectName,
		Production: util.GetBoolPointer(true),
	})
	if err != nil {
		return fmt.Errorf("failed to list envs for product: %s, err: %s", projectName, err)
	}

	// check if namings rule changes for services deployed in envs
	if serviceTemplate.GetReleaseNaming() == args.NamingRule {
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

	// check if the release name already exists
	for _, product := range products {
		releaseName := util.GeneReleaseName(args.NamingRule, product.ProductName, product.Namespace, product.EnvName, args.ServiceName)
		releaseNameMap, err := commonutil.GetReleaseNameToChartNameMap(product)
		if err != nil {
			return fmt.Errorf("failed to get release name to chart name map, err: %s", err)
		}
		if chartOrSvcName, ok := releaseNameMap[releaseName]; ok && chartOrSvcName != args.ServiceName {
			return fmt.Errorf("release name %s already exists for chart or service %s in environment: %s", releaseName, chartOrSvcName, product.EnvName)
		}
	}

	serviceTemplate.ReleaseNaming = args.NamingRule
	rev, err := getNextServiceRevision(projectName, args.ServiceName, true)
	if err != nil {
		return fmt.Errorf("failed to get service next revision, service %s, err: %s", args.ServiceName, err)
	}

	basePath := config.LocalProductionServicePath(serviceTemplate.ProductName, serviceTemplate.ServiceName)
	if err = commonutil.PreLoadProductionServiceManifests(basePath, serviceTemplate); err != nil {
		return fmt.Errorf("failed to load chart info for service %s, err: %s", serviceTemplate.ServiceName, err)
	}

	fsTree := os.DirFS(config.LocalProductionServicePath(projectName, serviceTemplate.ServiceName))
	s3Base := config.ObjectStorageProductionServicePath(projectName, serviceTemplate.ServiceName)
	err = fs.ArchiveAndUploadFilesToS3(fsTree, []string{fmt.Sprintf("%s-%d", serviceTemplate.ServiceName, rev)}, s3Base, log)
	if err != nil {
		return fmt.Errorf("failed to upload chart info for service %s, err: %s", serviceTemplate.ServiceName, err)
	}

	serviceTemplate.Revision = rev
	err = commonrepo.NewProductionServiceColl().Create(serviceTemplate)
	if err != nil {
		return fmt.Errorf("failed to update relase naming for service: %s, err: %s", args.ServiceName, err)
	}

	go func() {
		// reinstall services in envs
		err = service.ReInstallHelmProductionSvcInAllEnvs(projectName, serviceTemplate)
		if err != nil {
			title := fmt.Sprintf("服务 [%s] 重建失败", args.ServiceName)
			notify.SendErrorMessage(userName, title, requestID, err, log)
		}
	}()

	return nil
}

func DeleteServiceTemplate(serviceName, serviceType, productName, isEnvTemplate, visibility string, log *zap.SugaredLogger) error {

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
		// 把该服务相关的s3的数据从仓库删除
		if err = fs.DeleteArchivedFileFromS3([]string{serviceName}, configbase.ObjectStorageServicePath(productName, serviceName), log); err != nil {
			log.Warnf("Failed to delete file %s, err: %s", serviceName, err)
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
	commonservice.DeleteServiceWebhookByName(serviceName, productName, log)
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
	yamls := util.SplitYaml(yamlNew)
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
		args.RenderedYaml, err = commonutil.RenderK8sSvcYaml(args.RenderedYaml, args.ProductName, args.ServiceName, args.VariableYaml)
		if err != nil {
			return fmt.Errorf("failed to render yaml, err: %s", err)
		}

		args.Yaml = util.ReplaceWrapLine(args.Yaml)
		args.RenderedYaml = util.ReplaceWrapLine(args.RenderedYaml)
		args.KubeYamls = util.SplitYaml(args.RenderedYaml)

		// since service may contain go-template grammar, errors may occur when parsing as k8s workloads
		// errors will only be logged here
		if err := commonutil.SetCurrentContainerImages(args); err != nil {
			log.Errorf("failed to ser set container images, err: %s", err)
			//return err
		}
		log.Infof("find %d containers in service %s", len(args.Containers), args.ServiceName)
	}

	// get next service revision
	rev, err := commonutil.GenerateServiceNextRevision(args.Production, args.ServiceName, args.ProductName)
	if err != nil {
		return fmt.Errorf("get next service template revision error: %v", err)
	}

	args.Revision = rev
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

func ListServiceTemplateOpenAPI(projectKey string, logger *zap.SugaredLogger) ([]*OpenAPIServiceBrief, error) {
	services, err := commonservice.ListServiceTemplate(projectKey, logger)
	if err != nil {
		log.Errorf("failed to list service from db, projectKey: %s, err:%v", projectKey, err)
		return nil, fmt.Errorf("failed to list service from db, projectKey: %s, error:%v", projectKey, err)
	}

	resp := make([]*OpenAPIServiceBrief, 0)
	for _, s := range services.Data {
		serv := &OpenAPIServiceBrief{
			ServiceName: s.Service,
			Source:      s.Source,
			Type:        s.Type,
		}
		container := make([]*ContainerBrief, 0)
		for _, c := range s.Containers {
			container = append(container, &ContainerBrief{
				Name:      c.Name,
				ImageName: c.ImageName,
				Image:     c.Image,
			})
		}
		serv.Containers = container
		resp = append(resp, serv)
	}

	return resp, nil
}

func ListProductionServiceTemplateOpenAPI(projectKey string, logger *zap.SugaredLogger) ([]*OpenAPIServiceBrief, error) {
	productionServices, err := ListProductionServices(projectKey, logger)
	if err != nil {
		log.Errorf("failed to list service from db, projectKey: %s, err:%v", projectKey, err)
		return nil, fmt.Errorf("failed to list service from db, projectKey: %s, error:%v", projectKey, err)
	}

	resp := make([]*OpenAPIServiceBrief, 0)
	for _, s := range productionServices.Data {
		serv := &OpenAPIServiceBrief{
			ServiceName: s.Service,
			Source:      s.Source,
			Type:        s.Type,
		}
		container := make([]*ContainerBrief, 0)
		for _, c := range s.Containers {
			container = append(container, &ContainerBrief{
				Name:      c.Name,
				ImageName: c.ImageName,
				Image:     c.Image,
			})
		}
		serv.Containers = container
		resp = append(resp, serv)
	}

	return resp, nil
}
