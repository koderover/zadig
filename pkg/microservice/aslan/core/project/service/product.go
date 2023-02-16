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
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/yaml"

	configbase "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/collaboration/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/collaboration/service"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/collaboration"
	environmentservice "github.com/koderover/zadig/pkg/microservice/aslan/core/environment/service"
	service2 "github.com/koderover/zadig/pkg/microservice/aslan/core/label/service"
	workflowservice "github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/service/workflow"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/client/policy"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/types"
	"github.com/koderover/zadig/pkg/util"
)

type CustomParseDataArgs struct {
	Rules []*ImageParseData `json:"rules"`
}

type ImageParseData struct {
	Repo     string `json:"repo,omitempty"`
	Image    string `json:"image,omitempty"`
	Tag      string `json:"tag,omitempty"`
	InUse    bool   `json:"inUse,omitempty"`
	PresetId int    `json:"presetId,omitempty"`
}

func GetProductTemplateServices(productName string, envType types.EnvType, isBaseEnv bool, baseEnvName string, log *zap.SugaredLogger) (*template.Product, error) {
	resp, err := templaterepo.NewProductColl().Find(productName)
	if err != nil {
		log.Errorf("GetProductTemplate error: %v", err)
		return nil, e.ErrGetProduct.AddDesc(err.Error())
	}

	err = FillProductTemplateVars([]*template.Product{resp}, log)
	if err != nil {
		return nil, fmt.Errorf("FillProductTemplateVars err : %v", err)
	}

	if resp.Services == nil {
		resp.Services = make([][]string, 0)
	}

	if envType == types.ShareEnv && !isBaseEnv {
		// At this point the request is from the environment share.
		resp.Services, err = environmentservice.GetEnvServiceList(context.TODO(), productName, baseEnvName)
		if err != nil {
			return nil, fmt.Errorf("failed to get service list from env %s of product %s: %s", baseEnvName, productName, err)
		}
	}

	return resp, nil
}

func ListOpenSourceProduct(log *zap.SugaredLogger) ([]*template.Product, error) {
	opt := &templaterepo.ProductListOpt{
		IsOpensource: "true",
	}

	tmpls, err := templaterepo.NewProductColl().ListWithOption(opt)
	if err != nil {
		log.Errorf("ProductTmpl.ListWithOpt error: %v", err)
		return nil, e.ErrListProducts.AddDesc(err.Error())
	}

	return tmpls, nil
}

// CreateProductTemplate 创建产品模板
func CreateProductTemplate(args *template.Product, log *zap.SugaredLogger) (err error) {
	kvs := args.Vars
	// do not save vars
	args.Vars = nil

	err = commonservice.ValidateKVs(kvs, args.AllServiceInfos(), log)
	if err != nil {
		return e.ErrCreateProduct.AddDesc(err.Error())
	}

	if err := ensureProductTmpl(args); err != nil {
		return e.ErrCreateProduct.AddDesc(err.Error())
	}

	err = commonrepo.NewProjectClusterRelationColl().Delete(&commonrepo.ProjectClusterRelationOption{ProjectName: args.ProductName})
	if err != nil {
		log.Errorf("Failed to delete projectClusterRelation, err:%s", err)
	}
	for _, clusterID := range args.ClusterIDs {
		err = commonrepo.NewProjectClusterRelationColl().Create(&commonmodels.ProjectClusterRelation{
			ProjectName: args.ProductName,
			ClusterID:   clusterID,
			CreatedBy:   args.UpdateBy,
		})
		if err != nil {
			log.Errorf("Failed to create projectClusterRelation, err:%s", err)
		}
	}

	err = templaterepo.NewProductColl().Create(args)
	if err != nil {
		log.Errorf("ProductTmpl.Create error: %v", err)
		return e.ErrCreateProduct.AddDesc(err.Error())
	}

	// 创建一个默认的渲染集
	err = commonservice.CreateRenderSet(&commonmodels.RenderSet{
		Name:        args.ProductName,
		ProductTmpl: args.ProductName,
		UpdateBy:    args.UpdateBy,
		IsDefault:   true,
		//KVs:         kvs,
	}, log)

	if err != nil {
		log.Errorf("ProductTmpl.Create error: %v", err)
		// 创建渲染集失败，删除产品模板
		return e.ErrCreateProduct.AddDesc(err.Error())
	}

	return
}

func UpdateServiceOrchestration(name string, services [][]string, updateBy string, log *zap.SugaredLogger) (err error) {
	templateProductInfo, err := templaterepo.NewProductColl().Find(name)
	if err != nil {
		log.Errorf("failed to query productInfo, projectName: %s, err: %s", name, err)
		return fmt.Errorf("failed to query productInfo, projectName: %s", name)
	}

	//validate services
	validServices := sets.NewString()
	usedServiceSet := sets.NewString()
	for _, serviceList := range templateProductInfo.Services {
		validServices.Insert(serviceList...)
	}

	for _, serviceSeq := range services {
		for _, service := range serviceSeq {
			if usedServiceSet.Has(service) {
				return fmt.Errorf("duplicated service:%s", service)
			}
			if !validServices.Has(service) {
				return fmt.Errorf("service:%s not in valid service list", service)
			}
			usedServiceSet.Insert(service)
			validServices.Delete(service)
		}
	}

	if validServices.Len() > 0 {
		return fmt.Errorf("service: [%s] not found in params", strings.Join(validServices.List(), ","))
	}

	if err = templaterepo.NewProductColl().UpdateServiceOrchestration(name, services, updateBy); err != nil {
		log.Errorf("UpdateChoreographyService error: %v", err)
		return e.ErrUpdateProduct.AddErr(err)
	}
	return nil
}

func UpdateProductionServiceOrchestration(name string, services [][]string, updateBy string, log *zap.SugaredLogger) (err error) {
	templateProductInfo, err := templaterepo.NewProductColl().Find(name)
	if err != nil {
		log.Errorf("failed to query productInfo, projectName: %s, err: %s", name, err)
		return fmt.Errorf("failed to query productInfo, projectName: %s", name)
	}

	//validate services
	validServices := sets.NewString()
	usedServiceSet := sets.NewString()
	for _, serviceList := range templateProductInfo.Services {
		validServices.Insert(serviceList...)
	}

	for _, serviceSeq := range services {
		for _, service := range serviceSeq {
			if usedServiceSet.Has(service) {
				return fmt.Errorf("duplicated service:%s", service)
			}
			if !validServices.Has(service) {
				return fmt.Errorf("service:%s not in valid service list", service)
			}
			usedServiceSet.Insert(service)
			validServices.Delete(service)
		}
	}

	if validServices.Len() > 0 {
		return fmt.Errorf("service: [%s] not found in params", strings.Join(validServices.List(), ","))
	}

	if err = templaterepo.NewProductColl().UpdateProductionServiceOrchestration(name, services, updateBy); err != nil {
		log.Errorf("UpdateChoreographyService error: %v", err)
		return e.ErrUpdateProduct.AddErr(err)
	}
	return nil
}

// UpdateProductTemplate 更新产品模板
func UpdateProductTemplate(name string, args *template.Product, log *zap.SugaredLogger) (err error) {
	kvs := args.Vars
	args.Vars = nil

	if err = commonservice.ValidateKVs(kvs, args.AllServiceInfos(), log); err != nil {
		log.Warnf("ProductTmpl.Update ValidateKVs error: %v", err)
	}

	if err := ensureProductTmpl(args); err != nil {
		return e.ErrUpdateProduct.AddDesc(err.Error())
	}

	if err = templaterepo.NewProductColl().Update(name, args); err != nil {
		log.Errorf("ProductTmpl.Update error: %v", err)
		return e.ErrUpdateProduct
	}
	// 如果是helm的项目，不需要新创建renderset
	if args.ProductFeature != nil && args.ProductFeature.DeployType == setting.HelmDeployType {
		return
	}
	// 更新默认的渲染集
	if err = commonservice.CreateRenderSet(&commonmodels.RenderSet{
		Name:        args.ProductName,
		ProductTmpl: args.ProductName,
		UpdateBy:    args.UpdateBy,
		IsDefault:   true,
		//KVs:         kvs,
	}, log); err != nil {
		log.Warnf("ProductTmpl.Update CreateRenderSet error: %v", err)
	}

	for _, envVars := range args.EnvVars {
		//创建环境变量
		if err = commonservice.CreateRenderSet(&commonmodels.RenderSet{
			EnvName:     envVars.EnvName,
			Name:        args.ProductName,
			ProductTmpl: args.ProductName,
			UpdateBy:    args.UpdateBy,
			IsDefault:   false,
			//KVs:         envVars.Vars,
		}, log); err != nil {
			log.Warnf("ProductTmpl.Update CreateRenderSet error: %v", err)
		}
	}

	//// 更新子环境渲染集
	//if err = commonservice.UpdateSubRenderSet(args.ProductName, kvs, log); err != nil {
	//	log.Warnf("ProductTmpl.Update UpdateSubRenderSet error: %v", err)
	//}

	return nil
}

// UpdateProductTmplStatus 更新项目onboarding状态
func UpdateProductTmplStatus(productName, onboardingStatus string, log *zap.SugaredLogger) (err error) {
	status, err := strconv.Atoi(onboardingStatus)
	if err != nil {
		log.Errorf("convert onboardingStatus to int failed, error: %v", err)
		return e.ErrUpdateProduct.AddErr(err)
	}

	if err = templaterepo.NewProductColl().UpdateOnboardingStatus(productName, status); err != nil {
		log.Errorf("ProductTmpl.UpdateOnboardingStatus failed, productName:%s, status:%d, error: %v", productName, status, err)
		return e.ErrUpdateProduct.AddErr(err)
	}

	return nil
}

func TransferHostProject(user, projectName string, log *zap.SugaredLogger) (err error) {
	projectInfo, err := templaterepo.NewProductColl().Find(projectName)
	if err != nil {
		return e.ErrUpdateProduct.AddDesc(err.Error())
	}

	if !projectInfo.IsHostProduct() {
		return e.ErrUpdateProduct.AddDesc("invalid project type")
	}

	services, err := transferServices(user, projectInfo, log)
	if err != nil {
		return err
	}

	// transfer host project type to k8s yaml project
	products, err := transferProducts(user, projectInfo, services, log)
	if err != nil {
		return e.ErrUpdateProduct.AddErr(err)
	}

	projectInfo.ProductFeature.CreateEnvType = "system"

	if err = saveServices(projectName, user, services); err != nil {
		return err
	}
	if err = saveProducts(products); err != nil {
		return err
	}
	if err = saveProject(projectInfo); err != nil {
		return err
	}
	return nil
}

// transferServices transfer service from external to zadig-host(spock)
func transferServices(user string, projectInfo *template.Product, logger *zap.SugaredLogger) ([]*commonmodels.Service, error) {
	templateServices, err := commonrepo.NewServiceColl().ListMaxRevisionsAllSvcByProduct(projectInfo.ProductName)
	if err != nil {
		return nil, err
	}

	for _, svc := range templateServices {
		log.Infof("transfer service %s/%s ", projectInfo.ProductName, svc.ServiceName)
		svc.Source = setting.SourceFromZadig
		svc.CreateBy = user
		svc.EnvName = ""
		svc.WorkloadType = ""
	}
	return templateServices, nil
}

func saveServices(projectName, username string, services []*commonmodels.Service) error {
	for _, svc := range services {
		serviceTemplateCounter := fmt.Sprintf(setting.ServiceTemplateCounterName, svc.ServiceName, svc.ProductName)
		err := commonrepo.NewCounterColl().UpsertCounter(serviceTemplateCounter, svc.Revision)
		if err != nil {
			log.Errorf("failed to set service counter: %s, err: %s", serviceTemplateCounter, err)
		}
	}

	return commonrepo.NewServiceColl().TransferServiceSource(projectName, setting.SourceFromExternal, setting.SourceFromZadig, username)
}

func saveProducts(products []*commonmodels.Product) error {
	for _, product := range products {

		err := commonrepo.NewServicesInExternalEnvColl().Delete(&commonrepo.ServicesInExternalEnvArgs{
			ProductName: product.ProductName,
			EnvName:     product.EnvName,
		})
		if err != nil && err != mongo.ErrNoDocuments {
			return err
		}

		err = commonrepo.NewProductColl().Update(product)
		if err != nil {
			return err
		}
		saveWorkloadStats(product.ClusterID, product.Namespace, product.ProductName, product.EnvName)
	}
	return nil
}

func saveProject(projectInfo *template.Product) error {
	return templaterepo.NewProductColl().UpdateProductFeature(projectInfo.ProductName, projectInfo.ProductFeature, projectInfo.UpdateBy)
}

// build service and env data
func transferProducts(user string, projectInfo *template.Product, templateServices []*commonmodels.Service, logger *zap.SugaredLogger) ([]*commonmodels.Product, error) {
	templateSvcMap := make(map[string]*commonmodels.Service)
	for _, svc := range templateServices {
		templateSvcMap[svc.ServiceName] = svc
	}

	products, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
		Name: projectInfo.ProductName,
	})
	if err != nil {
		return nil, err
	}

	// build rendersets and services, set necessary attributes
	for _, product := range products {
		rendersetInfo := &commonmodels.RenderSet{
			Name:        product.Namespace,
			EnvName:     product.EnvName,
			ProductTmpl: product.ProductName,
			UpdateBy:    user,
			IsDefault:   false,
		}
		err = commonservice.ForceCreateReaderSet(rendersetInfo, logger)
		if err != nil {
			return nil, err
		}

		product.Render = &commonmodels.RenderInfo{
			Name:        rendersetInfo.Name,
			Revision:    rendersetInfo.Revision,
			ProductTmpl: rendersetInfo.ProductTmpl,
			Description: rendersetInfo.Description,
		}

		currentWorkloads, err := commonservice.ListWorkloadTemplate(projectInfo.ProductName, product.EnvName, logger)
		if err != nil {
			return nil, err
		}

		// for transferred services, only support one group
		productServices := make([]*commonmodels.ProductService, 0)
		for _, workload := range currentWorkloads.Data {
			svcTemplate, ok := templateSvcMap[workload.Service]
			if !ok {
				log.Errorf("failed to find service: %s in template", workload.Service)
			}
			productServices = append(productServices, &commonmodels.ProductService{
				ServiceName: workload.Service,
				ProductName: product.ProductName,
				Type:        svcTemplate.Type,
				Revision:    svcTemplate.Revision,
				Containers:  svcTemplate.Containers,
			})
		}
		product.Services = [][]*commonmodels.ProductService{productServices}

		// update workload stat
		workloadStat, err := commonrepo.NewWorkLoadsStatColl().Find(product.ClusterID, product.Namespace)
		if err != nil {
			log.Errorf("workflowStat not found error:%s", err)
		}
		if workloadStat != nil {
			workloadStat.Workloads = commonservice.FilterWorkloadsByEnv(workloadStat.Workloads, product.ProductName, product.EnvName)
			if err := commonrepo.NewWorkLoadsStatColl().UpdateWorkloads(workloadStat); err != nil {
				log.Errorf("update workloads fail error:%s", err)
			}
		}

		// mark service as only import
		if product.ServiceDeployStrategy == nil {
			product.ServiceDeployStrategy = make(map[string]string)
		}
		for _, svc := range product.GetServiceMap() {
			product.ServiceDeployStrategy[svc.ServiceName] = setting.ServiceDeployStrategyImport
		}

		product.Source = setting.SourceFromZadig
		product.UpdateBy = user
		product.Revision = 1
		log.Infof("transfer project %s/%s ", projectInfo.ProductName, product.EnvName)
	}

	return products, nil
}

func saveWorkloadStats(clusterID, namespace, productName, envName string) {
	workloadStat, err := commonrepo.NewWorkLoadsStatColl().Find(clusterID, namespace)
	if err != nil {
		log.Errorf("failed to get workload stat data, err: %s", err)
		return
	}

	workloadStat.Workloads = commonservice.FilterWorkloadsByEnv(workloadStat.Workloads, productName, envName)
	if err := commonrepo.NewWorkLoadsStatColl().UpdateWorkloads(workloadStat); err != nil {
		log.Errorf("update workloads fail error:%s", err)
	}
}

// UpdateProject 更新项目
func UpdateProject(name string, args *template.Product, log *zap.SugaredLogger) (err error) {
	err = validateRule(args.CustomImageRule, args.CustomTarRule)
	if err != nil {
		return e.ErrInvalidParam.AddDesc(err.Error())
	}

	err = templaterepo.NewProductColl().Update(name, args)
	if err != nil {
		log.Errorf("Project.Update error: %v", err)
		return e.ErrUpdateProduct.AddDesc(err.Error())
	}
	return nil
}

func validateRule(customImageRule *template.CustomRule, customTarRule *template.CustomRule) error {
	var (
		customImageRuleMap map[string]string
		customTarRuleMap   map[string]string
	)
	body, err := json.Marshal(&customImageRule)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(body, &customImageRuleMap); err != nil {
		return err
	}

	for field, ruleValue := range customImageRuleMap {
		if err := validateCommonRule(ruleValue, field, config.ImageResourceType); err != nil {
			return err
		}
	}

	body, err = json.Marshal(&customTarRule)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(body, &customTarRuleMap); err != nil {
		return err
	}
	for field, ruleValue := range customTarRuleMap {
		if err := validateCommonRule(ruleValue, field, config.TarResourceType); err != nil {
			return err
		}
	}

	return nil
}

func validateCommonRule(currentRule, ruleType, deliveryType string) error {
	var (
		imageRegexString = "^[a-z0-9][a-zA-Z0-9-_:.]+$"
		tarRegexString   = "^[a-z0-9][a-zA-Z0-9-_.]+$"
		tagRegexString   = "^[a-z0-9A-Z_][a-zA-Z0-9-_.]+$"
		errMessage       = "contains invalid characters, please check"
	)

	if currentRule == "" {
		return fmt.Errorf("%s can not be empty", ruleType)
	}

	if deliveryType == config.ImageResourceType && !strings.Contains(currentRule, ":") {
		return fmt.Errorf("%s is invalid, must contain a colon", ruleType)
	}

	currentRule = commonservice.ReplaceRuleVariable(currentRule, &commonservice.Variable{
		"ss", "ss", "ss", "ss", "ss", "ss", "ss", "ss", "ss", "ss",
	})
	switch deliveryType {
	case config.ImageResourceType:
		if !regexp.MustCompile(imageRegexString).MatchString(currentRule) {
			return fmt.Errorf("image %s %s", ruleType, errMessage)
		}
		// validate tag
		tag := strings.Split(currentRule, ":")[1]
		if !regexp.MustCompile(tagRegexString).MatchString(tag) {
			return fmt.Errorf("image %s %s", ruleType, errMessage)
		}
	case config.TarResourceType:
		if !regexp.MustCompile(tarRegexString).MatchString(currentRule) {
			return fmt.Errorf("tar %s %s", ruleType, errMessage)
		}
	}
	return nil
}

// DeleteProductTemplate 删除产品模板
func DeleteProductTemplate(userName, productName, requestID string, isDelete bool, log *zap.SugaredLogger) (err error) {
	publicServices, err := commonrepo.NewServiceColl().ListMaxRevisions(&commonrepo.ServiceListOption{ProductName: productName, Visibility: setting.PublicService})
	if err != nil {
		log.Errorf("pre delete check failed, err: %s", err)
		return e.ErrDeleteProduct.AddDesc(err.Error())
	}

	serviceToProject, err := commonservice.GetServiceInvolvedProjects(publicServices, productName)
	if err != nil {
		log.Errorf("pre delete check failed, err: %s", err)
		return e.ErrDeleteProduct.AddDesc(err.Error())
	}
	for k, v := range serviceToProject {
		if len(v) > 0 {
			return e.ErrDeleteProduct.AddDesc(fmt.Sprintf("共享服务[%s]在项目%v中被引用，请解除引用后删除", k, v))
		}
	}

	envs, _ := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{Name: productName})
	for _, env := range envs {
		if err = commonrepo.NewProductColl().UpdateStatus(env.EnvName, productName, setting.ProductStatusDeleting); err != nil {
			log.Errorf("DeleteProductTemplate Update product Status error: %s", err)
			return e.ErrDeleteProduct
		}
	}

	if err = commonservice.DeleteRenderSet(productName, log); err != nil {
		log.Errorf("DeleteProductTemplate DeleteRenderSet err: %s", err)
		return err
	}

	if err = DeleteTestModules(productName, requestID, log); err != nil {
		log.Errorf("DeleteProductTemplate Delete productName %s test err: %s", productName, err)
		return err
	}

	// delete collaboration_mode and collaboration_instance
	if err := DeleteCollabrationMode(productName, userName, log); err != nil {
		log.Errorf("DeleteCollabrationMode err:%s", err)
		return err
	}

	if err = DeletePolicy(productName, log); err != nil {
		log.Errorf("DeletePolicy  productName %s  err: %s", productName, err)
		return err
	}

	if err = DeleteLabels(productName, log); err != nil {
		log.Errorf("DeleteLabels  productName %s  err: %s", productName, err)
		return err
	}

	if err = commonservice.DeleteWorkflows(productName, requestID, log); err != nil {
		log.Errorf("DeleteProductTemplate Delete productName %s workflow err: %s", productName, err)
		return err
	}

	if err = commonservice.DeleteWorkflowV3s(productName, requestID, log); err != nil {
		log.Errorf("DeleteProductTemplate Delete productName %s workflowV3 err: %s", productName, err)
		return err
	}

	if err = commonservice.DeleteWorkflowV4sByProjectName(productName, log); err != nil {
		log.Errorf("DeleteProductTemplate Delete productName %s workflowV4 err: %s", productName, err)
		return err
	}

	if err = commonservice.DeletePipelines(productName, requestID, log); err != nil {
		log.Errorf("DeleteProductTemplate Delete productName %s pipeline err: %s", productName, err)
		return err
	}

	// delete projectClusterRelation
	if err = commonrepo.NewProjectClusterRelationColl().Delete(&commonrepo.ProjectClusterRelationOption{ProjectName: productName}); err != nil {
		log.Errorf("DeleteProductTemplate Delete productName %s ProjectClusterRelation err: %s", productName, err)
	}

	err = templaterepo.NewProductColl().Delete(productName)
	if err != nil {
		log.Errorf("ProductTmpl.Delete error: %s", err)
		return e.ErrDeleteProduct
	}

	err = commonrepo.NewCounterColl().Delete(fmt.Sprintf("product:%s", productName))
	if err != nil {
		log.Errorf("Counter.Delete error: %s", err)
		return err
	}

	services, _ := commonrepo.NewServiceColl().ListMaxRevisions(
		&commonrepo.ServiceListOption{ProductName: productName, Type: setting.K8SDeployType},
	)

	//删除交付中心
	//删除构建/删除测试/删除服务
	//删除workflow和历史task
	go func() {
		_ = commonrepo.NewBuildColl().Delete("", productName)
		_ = commonrepo.NewServiceColl().Delete("", "", productName, "", 0)
		_ = commonservice.DeleteDeliveryInfos(productName, log)
		_ = DeleteProductsAsync(userName, productName, requestID, isDelete, log)

		// delete service webhooks after services are deleted
		for _, s := range services {
			commonservice.ProcessServiceWebhook(nil, s, s.ServiceName, log)
		}
	}()

	// 删除workload
	if isDelete {
		go func() {
			workloads, _ := commonrepo.NewWorkLoadsStatColl().FindByProductName(productName)
			for _, v := range workloads {
				// update workloads
				tmp := []commonmodels.Workload{}
				for _, vv := range v.Workloads {
					if vv.ProductName != productName {
						tmp = append(tmp, vv)
					}
				}
				v.Workloads = tmp
				commonrepo.NewWorkLoadsStatColl().UpdateWorkloads(v)
			}
		}()
	}
	// delete servicesInExternalEnv data
	go func() {
		_ = commonrepo.NewServicesInExternalEnvColl().Delete(&commonrepo.ServicesInExternalEnvArgs{
			ProductName: productName,
		})
	}()

	// delete privateKey data
	go func() {
		if err = commonrepo.NewPrivateKeyColl().BulkDelete(productName); err != nil {
			log.Errorf("failed to bulk delete privateKey, error:%s", err)
		}
	}()

	return nil
}

// ForkProduct Deprecated
func ForkProduct(username, uid, requestID string, args *template.ForkProject, log *zap.SugaredLogger) error {

	prodTmpl, err := templaterepo.NewProductColl().Find(args.ProductName)
	if err != nil {
		errMsg := fmt.Sprintf("[ProductTmpl.Find] %s error: %v", args.ProductName, err)
		log.Error(errMsg)
		return e.ErrForkProduct.AddDesc(errMsg)
	}

	prodTmpl.ChartInfos = args.ValuesYamls
	// Load Service
	var svcs [][]*commonmodels.ProductService
	allServiceInfoMap := prodTmpl.AllServiceInfoMap()
	for _, names := range prodTmpl.Services {
		servicesResp := make([]*commonmodels.ProductService, 0)

		for _, serviceName := range names {
			opt := &commonrepo.ServiceFindOption{
				ServiceName:   serviceName,
				ProductName:   allServiceInfoMap[serviceName].Owner,
				ExcludeStatus: setting.ProductStatusDeleting,
			}

			serviceTmpl, err := commonrepo.NewServiceColl().Find(opt)
			if err != nil {
				errMsg := fmt.Sprintf("[ServiceTmpl.List] %s error: %v", opt.ServiceName, err)
				log.Error(errMsg)
				return e.ErrForkProduct.AddDesc(errMsg)
			}
			serviceResp := &commonmodels.ProductService{
				ServiceName: serviceTmpl.ServiceName,
				ProductName: serviceTmpl.ProductName,
				Type:        serviceTmpl.Type,
				Revision:    serviceTmpl.Revision,
			}
			if serviceTmpl.Type == setting.HelmDeployType {
				serviceResp.Containers = make([]*commonmodels.Container, 0)
				for _, c := range serviceTmpl.Containers {
					container := &commonmodels.Container{
						Name:      c.Name,
						Image:     c.Image,
						ImagePath: c.ImagePath,
						ImageName: util.GetImageNameFromContainerInfo(c.ImageName, c.Name),
					}
					serviceResp.Containers = append(serviceResp.Containers, container)
				}
			}
			servicesResp = append(servicesResp, serviceResp)
		}
		svcs = append(svcs, servicesResp)
	}

	prod := commonmodels.Product{
		ProductName:     prodTmpl.ProductName,
		Revision:        prodTmpl.Revision,
		IsPublic:        false,
		EnvName:         args.EnvName,
		Services:        svcs,
		Source:          setting.HelmDeployType,
		ServiceRenders:  prodTmpl.ChartInfos,
		IsForkedProduct: true,
	}

	err = environmentservice.CreateProduct(username, requestID, &prod, log)
	if err != nil {
		_, messageMap := e.ErrorMessage(err)
		if description, ok := messageMap["description"]; ok {
			return e.ErrForkProduct.AddDesc(description.(string))
		}
		errMsg := fmt.Sprintf("Failed to create env in order to fork product, the error is: %+v", err)
		log.Errorf(errMsg)
		return e.ErrForkProduct.AddDesc(errMsg)
	}

	workflowPreset, err := workflowservice.PreSetWorkflow(args.ProductName, log)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to get workflow preset info, the error is: %+v", err)
		log.Error(errMsg)
		return e.ErrForkProduct.AddDesc(errMsg)
	}

	buildModule := []*commonmodels.BuildModule{}
	artifactModule := []*commonmodels.ArtifactModule{}
	for _, i := range workflowPreset {
		buildModuleVer := "stable"
		if len(i.BuildModuleVers) != 0 {
			buildModuleVer = i.BuildModuleVers[0]
		}
		buildModule = append(buildModule, &commonmodels.BuildModule{
			BuildModuleVer: buildModuleVer,
			Target:         i.Target,
		})
		artifactModule = append(artifactModule, &commonmodels.ArtifactModule{Target: i.Target})
	}

	workflowArgs := &commonmodels.Workflow{
		ArtifactStage:   &commonmodels.ArtifactStage{Enabled: true, Modules: artifactModule},
		BuildStage:      &commonmodels.BuildStage{Enabled: false, Modules: buildModule},
		Name:            args.WorkflowName,
		ProductTmplName: args.ProductName,
		Enabled:         true,
		EnvName:         args.EnvName,
		TestStage:       &commonmodels.TestStage{Enabled: false, Tests: []*commonmodels.TestExecArgs{}},
		SecurityStage:   &commonmodels.SecurityStage{Enabled: false},
		DistributeStage: &commonmodels.DistributeStage{
			Enabled:     false,
			Distributes: []*commonmodels.ProductDistribute{},
			Releases:    []commonmodels.RepoImage{},
		},
		HookCtl:   &commonmodels.WorkflowHookCtrl{Enabled: false, Items: []*commonmodels.WorkflowHook{}},
		Schedules: &commonmodels.ScheduleCtrl{Enabled: false, Items: []*commonmodels.Schedule{}},
		CreateBy:  username,
		UpdateBy:  username,
	}
	err = policy.NewDefault().CreateOrUpdateRoleBinding(args.ProductName, &policy.RoleBinding{
		UID:    uid,
		Role:   string(setting.Contributor),
		Preset: true,
	})
	if err != nil {
		log.Errorf("Failed to create or update roleBinding, err: %s", err)
		return e.ErrForkProduct
	}

	return workflowservice.CreateWorkflow(workflowArgs, log)
}

func UnForkProduct(userID string, username, productName, workflowName, envName, requestID string, log *zap.SugaredLogger) error {
	if _, err := workflowservice.FindWorkflow(workflowName, log); err == nil {
		err = commonservice.DeleteWorkflow(workflowName, requestID, false, log)
		if err != nil {
			log.Errorf("Failed to delete forked workflow: %s, the error is: %+v", workflowName, err)
			return e.ErrUnForkProduct.AddDesc(err.Error())
		}
	}

	policyClient := policy.NewDefault()
	err := policyClient.DeleteRoleBinding(configbase.RoleBindingNameFromUIDAndRole(userID, setting.Contributor, ""), productName)
	if err != nil {
		log.Errorf("Failed to delete roleBinding, err: %s", err)
		return e.ErrForkProduct
	}
	if err := environmentservice.DeleteProduct(username, envName, productName, requestID, true, log); err != nil {
		_, messageMap := e.ErrorMessage(err)
		if description, ok := messageMap["description"]; ok {
			if description != "not found" {
				return e.ErrUnForkProduct.AddDesc(description.(string))
			}
		} else {
			errMsg := fmt.Sprintf("Failed to delete env %s in order to unfork product, the error is: %+v", envName, err)
			log.Errorf(errMsg)
			return e.ErrUnForkProduct.AddDesc(errMsg)
		}
	}
	return nil
}

func FillProductTemplateVars(productTemplates []*template.Product, log *zap.SugaredLogger) error {
	return commonservice.FillProductTemplateVars(productTemplates, log)
}

// ensureProductTmpl 检查产品模板参数
func ensureProductTmpl(args *template.Product) error {
	if args == nil {
		return errors.New("nil ProductTmpl")
	}

	if len(args.ProductName) == 0 {
		return errors.New("empty product name")
	}

	if !config.ServiceNameRegex.MatchString(args.ProductName) {
		return fmt.Errorf("product name must match %s", config.ServiceNameRegexString)
	}

	serviceNames := sets.NewString()
	for _, sg := range args.Services {
		for _, s := range sg {
			if serviceNames.Has(s) {
				return fmt.Errorf("duplicated service found: %s", s)
			}
			serviceNames.Insert(s)
		}
	}

	// Revision为0表示是新增项目，新增项目不需要进行共享服务的判断，只在编辑项目时进行判断
	if args.Revision != 0 {
		//获取该项目下的所有服务
		productTmpl, err := templaterepo.NewProductColl().Find(args.ProductName)
		if err != nil {
			log.Errorf("Can not find project %s, error: %s", args.ProductName, err)
			return fmt.Errorf("project not found: %s", err)
		}

		var newSharedServices []*template.ServiceInfo
		currentSharedServiceMap := productTmpl.SharedServiceInfoMap()
		for _, s := range args.SharedServices {
			if _, ok := currentSharedServiceMap[s.Name]; !ok {
				newSharedServices = append(newSharedServices, s)
			}
		}

		if len(newSharedServices) > 0 {
			services, err := commonrepo.NewServiceColl().ListMaxRevisions(&commonrepo.ServiceListOption{
				InServices: newSharedServices,
				Visibility: setting.PublicService,
			})
			if err != nil {
				log.Errorf("Failed to list services, err: %s", err)
				return err
			}

			if len(newSharedServices) != len(services) {
				return fmt.Errorf("新增的共享服务服务不存在或者已经不是共享服务")
			}
		}
	}

	// 设置新的版本号
	rev, err := commonrepo.NewCounterColl().GetNextSeq("product:" + args.ProductName)
	if err != nil {
		return fmt.Errorf("get next product template revision error: %v", err)
	}

	args.Revision = rev
	return nil
}

func DeleteProductsAsync(userName, productName, requestID string, isDelete bool, log *zap.SugaredLogger) error {
	envs, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{Name: productName})
	if err != nil {
		return e.ErrListProducts.AddDesc(err.Error())
	}
	errList := new(multierror.Error)
	for _, env := range envs {
		err = environmentservice.DeleteProduct(userName, env.EnvName, productName, requestID, isDelete, log)
		if err != nil {
			errList = multierror.Append(errList, err)
		}
	}
	if err := errList.ErrorOrNil(); err != nil {
		log.Errorf("DeleteProductsAsync err:%v", err)
		return err
	}
	return nil
}

type ProductInfo struct {
	Value       string         `bson:"value"              json:"value"`
	Label       string         `bson:"label"              json:"label"`
	ServiceInfo []*ServiceInfo `bson:"services"           json:"services"`
}

type ServiceInfo struct {
	Value         string           `bson:"value"              json:"value"`
	Label         string           `bson:"label"              json:"label"`
	ContainerInfo []*ContainerInfo `bson:"containers"         json:"containers"`
}

// ContainerInfo ...
type ContainerInfo struct {
	Value string `bson:"value"              json:"value"`
	Label string `bson:"label"              json:"label"`
}

func ListTemplatesHierachy(userName string, log *zap.SugaredLogger) ([]*ProductInfo, error) {
	var (
		err          error
		resp         = make([]*ProductInfo, 0)
		productTmpls = make([]*template.Product, 0)
	)

	productTmpls, err = templaterepo.NewProductColl().List()
	if err != nil {
		log.Errorf("[%s] ProductTmpl.List error: %v", userName, err)
		return nil, e.ErrListProducts.AddDesc(err.Error())
	}

	for _, productTmpl := range productTmpls {
		pInfo := &ProductInfo{Value: productTmpl.ProductName, Label: productTmpl.ProductName, ServiceInfo: []*ServiceInfo{}}
		services, err := commonrepo.NewServiceColl().ListMaxRevisionsForServices(productTmpl.AllServiceInfos(), "")
		if err != nil {
			log.Errorf("Failed to list service for project %s, error: %s", productTmpl.ProductName, err)
			return nil, e.ErrGetProduct.AddDesc(err.Error())
		}
		for _, svcTmpl := range services {
			sInfo := &ServiceInfo{Value: svcTmpl.ServiceName, Label: svcTmpl.ServiceName, ContainerInfo: make([]*ContainerInfo, 0)}

			for _, c := range svcTmpl.Containers {
				sInfo.ContainerInfo = append(sInfo.ContainerInfo, &ContainerInfo{Value: c.Name, Label: c.Name})
			}

			pInfo.ServiceInfo = append(pInfo.ServiceInfo, sInfo)
		}
		resp = append(resp, pInfo)
	}
	return resp, nil
}

func GetCustomMatchRules(productName string, log *zap.SugaredLogger) ([]*ImageParseData, error) {
	productInfo, err := templaterepo.NewProductColl().Find(productName)
	if err != nil {
		log.Errorf("query product:%s fail, err:%s", productName, err.Error())
		return nil, fmt.Errorf("failed to find product %s", productName)
	}

	rules := productInfo.ImageSearchingRules
	if len(rules) == 0 {
		rules = commonservice.GetPresetRules()
	}

	ret := make([]*ImageParseData, 0, len(rules))
	for _, singleData := range rules {
		ret = append(ret, &ImageParseData{
			Repo:     singleData.Repo,
			Image:    singleData.Image,
			Tag:      singleData.Tag,
			InUse:    singleData.InUse,
			PresetId: singleData.PresetId,
		})
	}
	return ret, nil
}

func UpdateCustomMatchRules(productName, userName, requestID string, matchRules []*ImageParseData) error {
	productInfo, err := templaterepo.NewProductColl().Find(productName)
	if err != nil {
		log.Errorf("query product:%s fail, err:%s", productName, err.Error())
		return fmt.Errorf("failed to find product %s", productName)
	}

	if len(matchRules) == 0 {
		return errors.New("match rules can't be empty")
	}
	haveInUse := false
	for _, rule := range matchRules {
		if rule.InUse {
			haveInUse = true
			break
		}
	}
	if !haveInUse {
		return errors.New("no rule is selected to be used")
	}

	imageRulesToSave := make([]*template.ImageSearchingRule, 0)
	for _, singleData := range matchRules {
		if singleData.Repo == "" && singleData.Image == "" && singleData.Tag == "" {
			continue
		}
		imageRulesToSave = append(imageRulesToSave, &template.ImageSearchingRule{
			Repo:     singleData.Repo,
			Image:    singleData.Image,
			Tag:      singleData.Tag,
			InUse:    singleData.InUse,
			PresetId: singleData.PresetId,
		})
	}

	productInfo.ImageSearchingRules = imageRulesToSave
	productInfo.UpdateBy = userName

	services, err := commonrepo.NewServiceColl().ListMaxRevisionsByProduct(productName)
	if err != nil {
		return err
	}
	err = reParseServices(userName, requestID, services, imageRulesToSave)
	if err != nil {
		return err
	}

	err = templaterepo.NewProductColl().Update(productName, productInfo)
	if err != nil {
		log.Errorf("failed to update product:%s, err:%s", productName, err.Error())
		return fmt.Errorf("failed to store match rules")
	}

	return nil
}

// reparse values.yaml for each service
func reParseServices(userName, requestID string, serviceList []*commonmodels.Service, matchRules []*template.ImageSearchingRule) error {
	updatedServiceTmpls := make([]*commonmodels.Service, 0)

	var err error
	var projectName string
	for _, serviceTmpl := range serviceList {
		if serviceTmpl.Type != setting.HelmDeployType || serviceTmpl.HelmChart == nil {
			continue
		}
		valuesYaml := serviceTmpl.HelmChart.ValuesYaml
		projectName = serviceTmpl.ProductName

		valuesMap := make(map[string]interface{})
		err = yaml.Unmarshal([]byte(valuesYaml), &valuesMap)
		if err != nil {
			err = errors.Wrapf(err, "failed to unmarshal values.yamf for service %s", serviceTmpl.ServiceName)
			break
		}

		serviceTmpl.Containers, err = commonservice.ParseImagesByRules(valuesMap, matchRules)
		if err != nil {
			break
		}

		if len(serviceTmpl.Containers) == 0 {
			log.Warnf("service:%s containers is empty after parse, valuesYaml %s", serviceTmpl.ServiceName, valuesYaml)
		}

		serviceTmpl.CreateBy = userName
		serviceTemplate := fmt.Sprintf(setting.ServiceTemplateCounterName, serviceTmpl.ServiceName, serviceTmpl.ProductName)
		rev, errRevision := commonrepo.NewCounterColl().GetNextSeq(serviceTemplate)
		if errRevision != nil {
			err = fmt.Errorf("get next helm service revision error: %v", errRevision)
			break
		}
		serviceTmpl.Revision = rev
		if err = commonrepo.NewServiceColl().Delete(serviceTmpl.ServiceName, setting.HelmDeployType, serviceTmpl.ProductName, setting.ProductStatusDeleting, serviceTmpl.Revision); err != nil {
			log.Errorf("helmService.update delete %s error: %v", serviceTmpl.ServiceName, err)
			break
		}

		if err = commonrepo.NewServiceColl().Create(serviceTmpl); err != nil {
			log.Errorf("helmService.update serviceName:%s error:%v", serviceTmpl.ServiceName, err)
			err = e.ErrUpdateTemplate.AddDesc(err.Error())
			break
		}

		updatedServiceTmpls = append(updatedServiceTmpls, serviceTmpl)
	}

	// roll back all template services if error occurs
	if err != nil {
		for _, serviceTmpl := range updatedServiceTmpls {
			if err = commonrepo.NewServiceColl().Delete(serviceTmpl.ServiceName, setting.HelmDeployType, serviceTmpl.ProductName, "", serviceTmpl.Revision); err != nil {
				log.Errorf("helmService.update delete %s error: %v", serviceTmpl.ServiceName, err)
				continue
			}
		}
		return err
	}

	return environmentservice.AutoDeployHelmServiceToEnvs(userName, requestID, projectName, updatedServiceTmpls, log.SugaredLogger())
}

func DeleteCollabrationMode(productName string, userName string, log *zap.SugaredLogger) error {
	// find all collaboration mode in this project
	res, err := collaboration.GetCollaborationModes([]string{productName}, log)
	if err != nil {
		log.Errorf("GetCollaborationModes err: %s", err)
		return err
	}
	//  delete all collaborationMode
	for _, mode := range res.Collaborations {
		if err := service.DeleteCollaborationMode(userName, productName, mode.Name, log); err != nil {
			log.Errorf("DeleteCollaborationMode err: %s", err)
			return err
		}
	}
	// delete all collaborationIns
	if err := mongodb.NewCollaborationInstanceColl().DeleteByProject(productName); err != nil {
		log.Errorf("fail to DeleteByProject err:%s", err)
		return err
	}
	return nil
}

func DeletePolicy(productName string, log *zap.SugaredLogger) error {
	policy.NewDefault()
	if err := policy.NewDefault().DeletePolicies(productName, policy.DeletePoliciesArgs{
		Names: []string{},
	}); err != nil {
		log.Errorf("DeletePolicies err :%s", err)
		return err
	}
	return nil
}

func DeleteLabels(productName string, log *zap.SugaredLogger) error {
	if err := service2.DeleteLabelsAndBindingsByProject(productName, log); err != nil {
		log.Errorf("delete labels and bindings by project fail , err :%s", err)
		return err
	}
	return nil
}
