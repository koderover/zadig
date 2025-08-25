/*
Copyright 2022 The KodeRover Authors.

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

	"github.com/hashicorp/go-multierror"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	templatemodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	commontypes "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/types"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/setting"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/util"
)

type YamlProductItem struct {
	OldName         string                           `json:"old_name"`
	NewName         string                           `json:"new_name"`
	BaseName        string                           `json:"base_name"`
	DefaultValues   string                           `json:"default_values"`
	GlobalVariables []*commontypes.GlobalVariableKV  `json:"global_variables"`
	Services        []*commonservice.K8sSvcRenderArg `json:"services"`
}

type CopyYamlProductArg struct {
	Items []YamlProductItem `json:"items"`
}

type HelmProductItem struct {
	OldName       string                            `json:"old_name"`
	NewName       string                            `json:"new_name"`
	BaseName      string                            `json:"base_name"`
	DefaultValues string                            `json:"default_values"`
	ChartValues   []*commonservice.HelmSvcRenderArg `json:"chart_values"`
	ValuesData    *commonservice.ValuesDataArgs     `json:"values_data"`
}

type CopyHelmProductArg struct {
	Items []HelmProductItem
}

func BulkCopyHelmProduct(projectName, user, requestID string, arg CopyHelmProductArg, log *zap.SugaredLogger) error {
	if len(arg.Items) == 0 {
		return nil
	}
	var envs []string
	for _, item := range arg.Items {
		envs = append(envs, item.OldName)
	}
	products, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
		Name:       projectName,
		InEnvs:     envs,
		Production: util.GetBoolPointer(false),
	})
	if err != nil {
		return err
	}
	productMap := make(map[string]*commonmodels.Product)
	for _, product := range products {
		productMap[product.EnvName] = product
	}
	var args []*CreateSingleProductArg
	for _, item := range arg.Items {
		if item.OldName == item.NewName {
			continue
		}
		if product, ok := productMap[item.OldName]; ok {
			chartValues := make([]*ProductHelmServiceCreationInfo, 0)
			for _, value := range item.ChartValues {
				chartValues = append(chartValues, &ProductHelmServiceCreationInfo{
					HelmSvcRenderArg: value,
					DeployStrategy:   setting.ServiceDeployStrategyDeploy,
				})
			}
			args = append(args, &CreateSingleProductArg{
				ProductName:   projectName,
				EnvName:       item.NewName,
				Namespace:     projectName + "-" + "env" + "-" + util.SanitizeName(item.NewName),
				ClusterID:     product.ClusterID,
				DefaultValues: item.DefaultValues,
				RegistryID:    product.RegistryID,
				BaseEnvName:   product.BaseName,
				BaseName:      item.BaseName,
				ChartValues:   chartValues,
				ValuesData:    item.ValuesData,
			})
		} else {
			return fmt.Errorf("product:%s not exist", item.OldName)
		}
	}
	return CopyHelmProduct(projectName, user, requestID, args, log)
}

func BulkCopyYamlProduct(projectName, user, requestID string, arg CopyYamlProductArg, log *zap.SugaredLogger) error {
	if len(arg.Items) == 0 {
		return nil
	}

	var envs []string
	for _, item := range arg.Items {
		envs = append(envs, item.OldName)
	}
	products, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
		Name:       projectName,
		InEnvs:     envs,
		Production: util.GetBoolPointer(false),
	})
	if err != nil {
		return err
	}

	productMap := make(map[string]*commonmodels.Product)
	for _, product := range products {
		productMap[product.EnvName] = product
	}

	for _, item := range arg.Items {
		if item.OldName == item.NewName {
			continue
		}
		if product, ok := productMap[item.OldName]; ok {
			newProduct := *product
			newProduct.EnvName = item.NewName
			//newProduct.Vars = item.Vars
			newProduct.Namespace = projectName + "-env-" + util.SanitizeName(newProduct.EnvName)
			util.Clear(&newProduct.ID)
			newProduct.BaseName = item.BaseName
			newProduct.GlobalVariables = item.GlobalVariables
			newProduct.DefaultValues = item.DefaultValues

			svcVariableKVMap := make(map[string][]*commontypes.RenderVariableKV)
			for _, sv := range item.Services {
				svcVariableKVMap[sv.ServiceName] = sv.VariableKVs
			}

			for _, svc := range newProduct.GetServiceMap() {
				if variableKVs, ok := svcVariableKVMap[svc.ServiceName]; ok {
					yamlContent, err := commontypes.RenderVariableKVToYaml(variableKVs, true)
					if err != nil {
						return fmt.Errorf("failed to convert variable kvs to yaml, err: %w", err)
					}
					svc.GetServiceRender().OverrideYaml = &templatemodels.CustomYaml{
						YamlContent:       yamlContent,
						RenderVariableKVs: variableKVs,
					}
				}
			}

			err = CreateProduct(user, requestID, &ProductCreateArg{&newProduct, nil}, log)
			if err != nil {
				return err
			}
		} else {
			return fmt.Errorf("product:%s not exist", item.OldName)
		}
	}
	return nil
}

// CopyYamlProduct copy product from source product
func CopyYamlProduct(user, requestID, projectName string, args []*CreateSingleProductArg, log *zap.SugaredLogger) (err error) {
	errList := new(multierror.Error)

	templateProduct, err := templaterepo.NewProductColl().Find(projectName)
	if err != nil || templateProduct == nil {
		if err != nil {
			log.Errorf("failed to query product %s, err %s ", projectName, err.Error())
		}
		return e.ErrCreateEnv.AddDesc(fmt.Sprintf("failed to query product %s ", projectName))
	}

	for _, arg := range args {
		baseProject, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
			Name:    arg.ProductName,
			EnvName: arg.BaseEnvName,
		})
		if err != nil {
			return e.ErrCreateEnv.AddErr(fmt.Errorf("failed to find base environment: %s, err: %s", arg.BaseEnvName, err))
		}

		// use service revision defined in base environment
		servicesInBaseNev := baseProject.GetServiceMap()

		for _, svcLists := range arg.Services {
			for _, svc := range svcLists {
				if baseSvc, ok := servicesInBaseNev[svc.ServiceName]; ok {
					svc.Revision = baseSvc.Revision
					svc.ProductName = baseSvc.ProductName
				}
			}
		}

		err = createSingleYamlProduct(templateProduct, requestID, user, arg, log)
		if err != nil {
			errList = multierror.Append(errList, err)
		}
	}
	return errList.ErrorOrNil()
}

// CopyHelmProduct copy product from source product, only works in test product
func CopyHelmProduct(productName, userName, requestID string, args []*CreateSingleProductArg, log *zap.SugaredLogger) error {
	errList := new(multierror.Error)
	templateProduct, err := templaterepo.NewProductColl().Find(productName)
	if err != nil || templateProduct == nil {
		if err != nil {
			log.Errorf("failed to query product %s, err %s ", productName, err.Error())
		}
		return e.ErrCreateEnv.AddDesc(fmt.Sprintf("failed to query product %s ", productName))
	}

	// fill all chart infos from product renderset
	err = commonservice.FillProductTemplateValuesYamls(templateProduct, false, log)
	if err != nil {
		return e.ErrCreateEnv.AddDesc(err.Error())
	}

	for _, arg := range args {
		baseProduct, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
			Name:    productName,
			EnvName: arg.BaseEnvName,
		})
		if err != nil {
			errList = multierror.Append(errList, fmt.Errorf("failed to query base product info name :%s,envname:%s", productName, arg.BaseName))
			continue
		}
		templateSvcs, err := commonutil.GetProductUsedTemplateSvcs(baseProduct)
		templateServiceMap := make(map[string]*commonmodels.Service)
		for _, svc := range templateSvcs {
			templateServiceMap[svc.ServiceName] = svc
		}

		//services deployed in base product may be different with services in template product
		//use services in base product when copying product instead of services in template product
		svcGroups := make([][]string, 0)
		for _, svcList := range baseProduct.Services {
			svcs := make([]string, 0)
			for _, svc := range svcList {
				svcs = append(svcs, svc.ServiceName)
			}
			svcGroups = append(svcGroups, svcs)
		}
		templateProduct.Services = svcGroups

		err = copySingleHelmProduct(templateProduct, baseProduct, requestID, userName, arg, templateServiceMap, log)
		if err != nil {
			errList = multierror.Append(errList, err)
		}
	}
	return errList.ErrorOrNil()
}

func copySingleHelmProduct(templateProduct *templatemodels.Product, productInfo *commonmodels.Product, requestID, userName string, arg *CreateSingleProductArg, serviceTmplMap map[string]*commonmodels.Service, log *zap.SugaredLogger) error {
	productInfo.ID = primitive.NilObjectID
	productInfo.Revision = 1
	productInfo.EnvName = arg.EnvName
	productInfo.UpdateBy = userName
	productInfo.ClusterID = arg.ClusterID
	productInfo.BaseName = arg.BaseName
	productInfo.Namespace = commonservice.GetProductEnvNamespace(arg.EnvName, arg.ProductName, arg.Namespace)
	productInfo.EnvConfigs = arg.EnvConfigs

	// merge chart infos, use chart info in product to override charts in template_project
	sourceChartMap := make(map[string]*templatemodels.ServiceRender)
	for _, singleChart := range productInfo.GetAllSvcRenders() {
		sourceChartMap[singleChart.ServiceName] = singleChart
	}
	templateCharts := templateProduct.ChartInfos
	templateProduct.ChartInfos = make([]*templatemodels.ServiceRender, 0)
	for _, chart := range templateCharts {
		if chartFromSource, ok := sourceChartMap[chart.ServiceName]; ok {
			templateProduct.ChartInfos = append(templateProduct.ChartInfos, chartFromSource)
		} else {
			templateProduct.ChartInfos = append(templateProduct.ChartInfos, chart)
		}
	}
	// fill services and chart infos of product
	err := prepareHelmProductCreation(templateProduct, productInfo, arg, serviceTmplMap, log)
	if err != nil {
		return err
	}

	return CreateProduct(userName, requestID, &ProductCreateArg{productInfo, nil}, log)
}
