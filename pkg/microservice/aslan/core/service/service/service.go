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

	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
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
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/notify"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/pm"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/repository"
	commontypes "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/types"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/environment/service"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/systemconfig"
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

func GetServiceTemplateOption(serviceName, productName string, revision int64, production bool, log *zap.SugaredLogger) (*ServiceOption, error) {
	service, err := commonservice.GetServiceTemplate(serviceName, setting.K8SDeployType, productName, setting.ProductStatusDeleting, revision, production, log)
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

func CreateK8sWorkLoads(ctx context.Context, requestID, userName string, args *K8sWorkloadsArgs, production bool, log *zap.SugaredLogger) error {
	kubeClient, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(args.ClusterID)
	if err != nil {
		log.Errorf("[%s] error: %v", args.Namespace, err)
		return err
	}

	clientset, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(args.ClusterID)
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
	templateSvcs := make(map[string]*commonmodels.Service)

	services, err := repository.ListMaxRevisions(&commonrepo.ServiceListOption{
		ProductName: args.ProductName,
	}, production)

	for _, v := range services {
		serviceString.Insert(v.ServiceName)
		templateSvcs[v.ServiceName] = v
	}

	session := mongotool.Session()
	defer session.EndSession(context.TODO())

	productCol := commonrepo.NewProductCollWithSession(session)

	err = mongotool.StartTransaction(session)
	if err != nil {
		return e.ErrCreateEnv.AddDesc("failed to start transaction")
	}

	productServices := make([]*commonmodels.ProductService, 0)

	g := new(errgroup.Group)
	for _, workload := range args.WorkLoads {
		tempWorkload := workload
		g.Go(func() error {

			productSvc := &commonmodels.ProductService{
				ServiceName: tempWorkload.Name,
				ProductName: args.ProductName,
				Type:        setting.K8SDeployType,
				Revision:    1,
			}
			productServices = append(productServices, productSvc)

			// If the service is already included in the database service template, add it to the new association table
			if serviceString.Has(tempWorkload.Name) {
				productSvc.Containers = templateSvcs[tempWorkload.Name].Containers
				productSvc.Resources, _ = kube.ManifestToResource(templateSvcs[tempWorkload.Name].Yaml)
				return nil
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

			templateSvc, err := CreateWorkloadTemplate(&commonmodels.Service{
				ServiceName:  tempWorkload.Name,
				Yaml:         string(bs),
				ProductName:  args.ProductName,
				CreateBy:     userName,
				Type:         setting.K8SDeployType,
				WorkloadType: tempWorkload.Type,
				Source:       setting.SourceFromExternal,
				EnvName:      args.EnvName,
				Revision:     1,
			}, production, session, log)
			if err != nil {
				return err
			}

			productSvc.Containers = templateSvc.Containers
			productSvc.Resources, _ = kube.ManifestToResource(templateSvc.Yaml)
			templateSvcs[templateSvc.ServiceName] = templateSvc
			serviceString.Insert(templateSvc.ServiceName)
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		mongotool.AbortTransaction(session)
		return err
	}

	// create environment if not exists
	if _, err = productCol.Find(&commonrepo.ProductFindOptions{
		Name:    args.ProductName,
		EnvName: args.EnvName,
	}); err != nil {
		if err := service.CreateProduct(userName, requestID, &service.ProductCreateArg{Product: &commonmodels.Product{
			ProductName: args.ProductName,
			Source:      setting.SourceFromExternal,
			ClusterID:   args.ClusterID,
			RegistryID:  args.RegistryID,
			EnvName:     args.EnvName,
			Namespace:   args.Namespace,
			UpdateBy:    userName,
			IsExisted:   true,
			Production:  production,
			Services:    [][]*commonmodels.ProductService{productServices},
		}, Session: session}, log); err != nil {
			mongotool.AbortTransaction(session)
			return e.ErrCreateProduct.AddDesc("create product Error for unknown reason")
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

func UpdateWorkloads(ctx context.Context, requestID, username, productName, envName string, args UpdateWorkloadsArgs, production bool, log *zap.SugaredLogger) error {
	kubeClient, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(args.ClusterID)
	if err != nil {
		log.Errorf("[%s] error: %s", args.Namespace, err)
		return err
	}
	clientset, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(args.ClusterID)
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

	err = mongotool.StartTransaction(session)
	if err != nil {
		return fmt.Errorf("failed to start transaction")
	}

	productInfo, err := commonrepo.NewProductCollWithSession(session).Find(&commonrepo.ProductFindOptions{
		Name:    productName,
		EnvName: envName,
	})
	if err != nil {
		return fmt.Errorf("failed to find product: %s/%s, err: %s", productName, envName, err)
	}

	add := map[string]*ServiceWorkloadsUpdateAction{}
	uploadSet := sets.NewString()
	removeSet := sets.NewString()

	for _, v := range args.WorkLoads {
		uploadSet.Insert(v.Name)
	}

	filteredSvcs := make([]*commonmodels.ProductService, 0)
	for _, svc := range productInfo.GetSvcList() {
		if uploadSet.Has(svc.ServiceName) {
			filteredSvcs = append(filteredSvcs, svc)
		} else {
			removeSet.Insert(svc.ServiceName)
		}
		uploadSet.Delete(svc.ServiceName)
	}

	for _, v := range args.WorkLoads {
		if uploadSet.Has(v.Name) {
			add[v.Name] = &ServiceWorkloadsUpdateAction{
				EnvName:     envName,
				Name:        v.Name,
				Type:        v.Type,
				ProductName: v.ProductName,
				//Operation:   "add",
			}
		}
	}

	for _, v := range removeSet.List() {
		serviceColl := repository.ServiceCollWithSession(production, session)
		err = serviceColl.Delete(v, setting.K8SDeployType, productName, "", 1)
		if err != nil {
			log.Errorf("failed to delete service %s, err: %s", v, err)
			mongotool.AbortTransaction(session)
			return fmt.Errorf("failed to delete service %s, err: %s", v, err)
		}
	}

	for _, v := range add {
		var bs []byte
		switch v.Type {
		case setting.Deployment:
			bs, _, err = getter.GetDeploymentYamlFormat(args.Namespace, v.Name, kubeClient)
		case setting.StatefulSet:
			bs, _, err = getter.GetStatefulSetYamlFormat(args.Namespace, v.Name, kubeClient)
		case setting.CronJob:
			bs, _, err = getter.GetCronJobYamlFormat(args.Namespace, v.Name, kubeClient, service.VersionLessThan121(versionInfo))
		}
		//svcNeedAdd.Insert(v.Name)
		if len(bs) == 0 || err != nil {
			log.Errorf("UpdateK8sWorkLoads not found yaml %s", err)
			delete(add, v.Name)
			continue
		}

		productSvc := &commonmodels.ProductService{
			ServiceName: v.Name,
			ProductName: productName,
			Type:        setting.K8SDeployType,
			Revision:    1,
		}

		templateSvc, err := CreateWorkloadTemplate(&commonmodels.Service{
			ServiceName:  v.Name,
			Yaml:         string(bs),
			ProductName:  productName,
			CreateBy:     username,
			Type:         setting.K8SDeployType,
			WorkloadType: v.Type,
			Source:       setting.SourceFromExternal,
			EnvName:      envName,
			Revision:     1,
		}, production, session, log)
		if err != nil {
			log.Errorf("create service template failed err:%v", err)
			delete(add, v.Name)
			continue
		}

		productSvc.Containers = templateSvc.Containers
		productSvc.Resources, _ = kube.ManifestToResource(templateSvc.Yaml)
		filteredSvcs = append(filteredSvcs, productSvc)
	}
	productInfo.Services = [][]*commonmodels.ProductService{filteredSvcs}
	err = commonrepo.NewProductCollWithSession(session).Update(productInfo)
	if err != nil {
		log.Errorf("failed to update product: %s error: %s", productName, err)
		mongotool.AbortTransaction(session)
		return err
	}

	return mongotool.CommitTransaction(session)
}

// CreateWorkloadTemplate only use for host projects
func CreateWorkloadTemplate(args *commonmodels.Service, production bool, session mongo.Session, log *zap.SugaredLogger) (*commonmodels.Service, error) {
	productTemp, err := templaterepo.NewProductColl().Find(args.ProductName)
	if err != nil {
		log.Errorf("Failed to find project %s, err: %s", args.ProductName, err)
		return nil, e.ErrInvalidParam.AddErr(err)
	}
	// 遍历args.KubeYamls，获取 Deployment 或者 StatefulSet 里面所有containers 镜像和名称
	args.KubeYamls = []string{args.Yaml}
	if err := commonutil.SetCurrentContainerImages(args); err != nil {
		log.Errorf("Failed tosetCurrentContainerImages %s, err: %s", args.ProductName, err)
		return nil, err
	}
	opt := &commonrepo.ServiceFindOption{
		ServiceName:   args.ServiceName,
		ProductName:   args.ProductName,
		ExcludeStatus: setting.ProductStatusDeleting,
	}

	serviceColl := repository.ServiceCollWithSession(production, session)
	_, notFoundErr := serviceColl.Find(opt)
	if notFoundErr != nil {
		if !production {
			if len(productTemp.Services) > 0 && !sets.NewString(productTemp.Services[0]...).Has(args.ServiceName) {
				productTemp.Services[0] = append(productTemp.Services[0], args.ServiceName)
			} else if len(productTemp.Services) == 0 {
				productTemp.Services = [][]string{{args.ServiceName}}
			}
		} else {
			if len(productTemp.ProductionServices) > 0 && !sets.NewString(productTemp.ProductionServices[0]...).Has(args.ServiceName) {
				productTemp.ProductionServices[0] = append(productTemp.ProductionServices[0], args.ServiceName)
			} else if len(productTemp.ProductionServices) == 0 {
				productTemp.ProductionServices = [][]string{{args.ServiceName}}
			}
		}

		err = templaterepo.NewProductCollWithSess(session).Update(args.ProductName, productTemp)
		if err != nil {
			log.Errorf("CreateServiceTemplate Update %s error: %s", args.ServiceName, err)
			return nil, e.ErrCreateTemplate.AddDesc(err.Error())
		}
		if err := serviceColl.Delete(args.ServiceName, args.Type, args.ProductName, setting.ProductStatusDeleting, 1); err != nil {
			log.Errorf("ServiceTmpl.delete %s error: %v", args.ServiceName, err)
		}

		if err := serviceColl.Create(args); err != nil {
			log.Errorf("ServiceTmpl.Create %s error: %v", args.ServiceName, err)
			return nil, e.ErrCreateTemplate.AddDesc(err.Error())
		}
	}
	return args, nil
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

func CreateServiceTemplate(userName string, args *commonmodels.Service, force bool, production bool, log *zap.SugaredLogger) (*ServiceOption, error) {
	opt := &commonrepo.ServiceFindOption{
		ServiceName:   args.ServiceName,
		Revision:      0,
		Type:          args.Type,
		ProductName:   args.ProductName,
		ExcludeStatus: setting.ProductStatusDeleting,
	}

	serviceTmpl, notFoundErr := repository.QueryTemplateService(opt, production)
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
	args.Production = production
	if err := ensureServiceTmpl(userName, args, log); err != nil {
		log.Errorf("ensureServiceTmpl error: %+v", err)
		return nil, e.ErrValidateTemplate.AddDesc(err.Error())
	}

	if err := repository.Delete(args.ServiceName, args.Type, args.ProductName, setting.ProductStatusDeleting, args.Revision, production); err != nil {
		log.Errorf("ServiceTmpl.delete %s error: %v", args.ServiceName, err)
	}

	// create a new revision of template service
	if err := repository.Create(args, production); err != nil {
		log.Errorf("ServiceTmpl.Create %s error: %v", args.ServiceName, err)
		return nil, e.ErrCreateTemplate.AddDesc(err.Error())
	}

	if notFoundErr != nil {
		if productTempl, err := commonservice.GetProductTemplate(args.ProductName, log); err == nil {
			//获取项目里面的所有服务
			if production {
				if len(productTempl.ProductionServices) > 0 && !sets.NewString(productTempl.ProductionServices[0]...).Has(args.ServiceName) {
					productTempl.ProductionServices[0] = append(productTempl.ProductionServices[0], args.ServiceName)
				} else {
					productTempl.ProductionServices = [][]string{{args.ServiceName}}
				}
			} else {
				if len(productTempl.Services) > 0 && !sets.NewString(productTempl.Services[0]...).Has(args.ServiceName) {
					productTempl.Services[0] = append(productTempl.Services[0], args.ServiceName)
				} else {
					productTempl.Services = [][]string{{args.ServiceName}}
				}
			}
			//更新项目模板
			err = templaterepo.NewProductColl().Update(args.ProductName, productTempl)
			if err != nil {
				log.Errorf("CreateServiceTemplate Update %s error: %s", args.ServiceName, err)
				return nil, e.ErrCreateTemplate.AddDesc(err.Error())
			}
		}
	}
	commonservice.ProcessServiceWebhook(args, serviceTmpl, args.ServiceName, production, log)

	err = service.AutoDeployYamlServiceToEnvs(userName, "", args, production, log)
	if err != nil {
		return nil, e.ErrCreateTemplate.AddErr(err)
	}

	return GetServiceOption(args, log)
}

func UpdateServiceEnvStatus(args *commonservice.ServiceTmplObject) error {
	currentService, err := commonrepo.NewServiceColl().Find(&commonrepo.ServiceFindOption{
		ProductName: args.ProductName,
		ServiceName: args.ServiceName,
		Revision:    args.Revision,
	})
	if err != nil {
		log.Errorf("Can not find service with option %+v. Error: %s", args, err)
		return err
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

		for k := range defaultStatusMap {
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
		oldSet.Insert(fmt.Sprintf("%s-%s", container.Name, container.Image))
	}
	newSet := sets.NewString()
	for _, container := range newContainers {
		newSet.Insert(fmt.Sprintf("%s-%s", container.Name, container.Image))
	}
	return !oldSet.Equal(newSet)
}

func UpdateServiceVariables(args *commonservice.ServiceTmplObject, production bool) error {
	currentService, err := repository.QueryTemplateService(&commonrepo.ServiceFindOption{
		ProductName: args.ProductName,
		ServiceName: args.ServiceName,
	}, production)
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

	err = repository.UpdateServiceVariables(currentService, production)
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
		err = repository.UpdateServiceContainers(currentService, production)
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

type UpdateEnvVMServiceHealthCheckRequest struct {
	ProjectName string `json:"project_name"`
	EnvName     string `json:"env_name"`
	ServiceName string `json:"service_name"`
	Username    string `json:"username"`
}

func UpdateEnvVMServiceHealthCheck(req *UpdateEnvVMServiceHealthCheckRequest) error {
	latestService, err := commonrepo.NewServiceColl().Find(&commonrepo.ServiceFindOption{
		ProductName: req.ProjectName,
		ServiceName: req.ServiceName,
	})
	if err != nil {
		log.Errorf("Can not find service with option %+v. Error: %s", req, err)
		return err
	}

	changeEnvStatus := make([]*commonmodels.EnvStatus, 0)
	for _, envConfigs := range latestService.EnvConfigs {
		if envConfigs.EnvName != req.EnvName {
			continue
		}

		for _, hostID := range envConfigs.HostIDs {
			for _, healthCheck := range latestService.HealthChecks {
				changeEnvStatus = append(changeEnvStatus, &commonmodels.EnvStatus{
					HostID:       hostID,
					EnvName:      req.EnvName,
					Address:      hostID,
					HealthChecks: healthCheck,
				})
			}
		}
	}

	for _, envStatus := range latestService.EnvStatuses {
		if envStatus.EnvName != req.EnvName {
			changeEnvStatus = append(changeEnvStatus, envStatus)
		}
	}

	// generate env status for this env
	updateArgs := &commonmodels.Service{
		ProductName: latestService.ProductName,
		ServiceName: req.ServiceName,
		Revision:    latestService.Revision,
		Type:        latestService.Type,
		CreateBy:    latestService.CreateBy,
		EnvConfigs:  latestService.EnvConfigs,
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

func UpdateReleaseNamingRule(userName, requestID, projectName string, args *ReleaseNamingRule, production bool, log *zap.SugaredLogger) error {
	serviceTemplate, err := repository.QueryTemplateService(&commonrepo.ServiceFindOption{
		ServiceName:   args.ServiceName,
		Revision:      0,
		Type:          setting.HelmDeployType,
		ProductName:   projectName,
		ExcludeStatus: setting.ProductStatusDeleting,
	}, production)
	if err != nil {
		return err
	}

	products, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
		Name:       projectName,
		Production: util.GetBoolPointer(production),
	})
	if err != nil {
		return fmt.Errorf("failed to list envs for product: %s, err: %s", projectName, err)
	}

	// check if namings rule changes for services deployed in envs
	if serviceTemplate.GetReleaseNaming() == args.NamingRule {
		products, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
			Name:       projectName,
			Production: util.GetBoolPointer(production),
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
	rev, err := getNextServiceRevision(projectName, args.ServiceName, production)
	if err != nil {
		return fmt.Errorf("failed to get service next revision, service %s, err: %s", args.ServiceName, err)
	}

	if production {
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
	} else {
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
	}

	serviceTemplate.Revision = rev
	err = repository.Create(serviceTemplate, production)
	if err != nil {
		return fmt.Errorf("failed to update relase naming for service: %s, err: %s", args.ServiceName, err)
	}

	go func() {
		// reinstall services in envs
		err = service.ReInstallHelmSvcInAllEnvs(projectName, serviceTemplate, production)
		if err != nil {
			title := fmt.Sprintf("服务 [%s] 重建失败", args.ServiceName)
			notify.SendErrorMessage(userName, title, requestID, err, log)
		}
	}()

	return nil
}

func DeleteServiceTemplate(serviceName, serviceType, productName string, production bool, log *zap.SugaredLogger) error {
	// 如果服务是PM类型，删除服务更新build的target信息
	if serviceType == setting.PMDeployType {
		// PM service type only support testing service
		if production {
			return e.ErrDeleteTemplate.AddDesc("PM service type only support testing service")
		}

		if serviceTmpl, err := commonservice.GetServiceTemplate(
			serviceName, setting.PMDeployType, productName, setting.ProductStatusDeleting, 0, false, log,
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

	err := repository.UpdateStatus(serviceName, productName, setting.ProductStatusDeleting, production)
	if err != nil {
		errMsg := fmt.Sprintf("[service.UpdateStatus] %s-%s error: %v", serviceName, serviceType, err)
		log.Error(errMsg)
		return e.ErrDeleteTemplate.AddDesc(errMsg)
	}

	if serviceType == setting.HelmDeployType {
		// 把该服务相关的s3的数据从仓库删除
		if production {
			if err = fs.DeleteArchivedFileFromS3([]string{serviceName}, configbase.ObjectStorageProductionServicePath(productName, serviceName), log); err != nil {
				log.Warnf("Failed to delete file %s, err: %s", serviceName, err)
			}
		} else {
			if err = fs.DeleteArchivedFileFromS3([]string{serviceName}, configbase.ObjectStorageServicePath(productName, serviceName), log); err != nil {
				log.Warnf("Failed to delete file %s, err: %s", serviceName, err)
			}
		}
	}

	//删除环境模板
	if productTempl, err := commonservice.GetProductTemplate(productName, log); err == nil {
		if production {
			newServices := make([][]string, len(productTempl.ProductionServices))
			for i, services := range productTempl.ProductionServices {
				for _, service := range services {
					if service != serviceName {
						newServices[i] = append(newServices[i], service)
					}
				}
			}
			productTempl.ProductionServices = newServices
		} else {
			newServices := make([][]string, len(productTempl.Services))
			for i, services := range productTempl.Services {
				for _, service := range services {
					if service != serviceName {
						newServices[i] = append(newServices[i], service)
					}
				}
			}
			productTempl.Services = newServices
		}
		err = templaterepo.NewProductColl().Update(productName, productTempl)
		if err != nil {
			log.Errorf("DeleteServiceTemplate Update %s error: %v", serviceName, err)
			return e.ErrDeleteTemplate.AddDesc(err.Error())
		}
	}
	commonservice.DeleteServiceWebhookByName(serviceName, productName, production, log)
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
	services, err := commonservice.ListServiceTemplate(projectKey, false, false, logger)
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
	productionServices, err := commonservice.ListServiceTemplate(projectKey, true, false, logger)
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
