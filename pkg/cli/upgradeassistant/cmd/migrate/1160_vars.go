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

package migrate

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"gopkg.in/yaml.v3"

	uamodel "github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/repository/models"
	uamongo "github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/util"
)

type DataBulkUpdater struct {
	Coll           *mongo.Collection
	WriteModels    []mongo.WriteModel
	WriteThreshold int
}

func HandleK8sYamlVars() error {
	err := handleServiceTemplates()
	if err != nil {
		log.Errorf("failed to handleServiceTemplates, err: %s", err)
		return err
	}

	err = adjustProductRenderInfo()
	if err != nil {
		log.Errorf("failed to adjustProductRenderInfo, err: %s", err)
		return err
	}

	return nil
}

func (dbu *DataBulkUpdater) AddModel(wModel mongo.WriteModel) error {
	dbu.WriteModels = append(dbu.WriteModels, wModel)
	if len(dbu.WriteModels) >= dbu.WriteThreshold {
		return dbu.Write()
	}
	return nil
}

func (dbu *DataBulkUpdater) Write() error {
	if len(dbu.WriteModels) == 0 {
		return nil
	}
	if _, err := dbu.Coll.BulkWrite(context.TODO(), dbu.WriteModels); err != nil {
		return fmt.Errorf("bulk write data error: %s", err)
	}
	dbu.WriteModels = []mongo.WriteModel{}
	return nil
}

func handleServiceTemplates() error {
	temProducts, err := templaterepo.NewProductColl().ListWithOption(&templaterepo.ProductListOpt{
		DeployType: setting.K8SDeployType,
	})
	if err != nil {
		log.Errorf("handleServiceTemplates list projects error: %s", err)
		return fmt.Errorf("handleServiceTemplates list projects error: %s", err)
	}

	for _, project := range temProducts {
		err = setServiceVariables(project.ProjectName)
		if err != nil {
			return err
		}
	}

	return nil
}

// read current parsed variables and convert to service variable yaml
func setServiceVariables(projectName string) error {
	templateServices, err := mongodb.NewServiceColl().ListMaxRevisionsByProduct(projectName)
	if err != nil {
		log.Errorf("failed to find service templates, err: %s", err)
		return err
	}

	defaultRenderset, err := uamongo.NewRenderSetColl().Find(&uamongo.RenderSetFindOption{
		ProductTmpl: projectName,
		Name:        projectName,
		IsDefault:   true,
	})
	if err != nil {
		log.Errorf("failed to find default renderset: %s, err: %s", projectName, err)
		return nil
	}

	// no variable used for this project
	if len(defaultRenderset.KVs) == 0 {
		return nil
	}

	svcsNeedUpdate := make([]*models.Service, 0)
	for _, svc := range templateServices {
		if len(svc.VariableYaml) > 0 {
			continue
		}
		valuesMap := make(map[string]interface{})
		for _, kv := range defaultRenderset.KVs {
			if !util.InStringArray(svc.ServiceName, kv.Services) {
				continue
			}
			valuesMap[kv.Key] = kv.Value
		}
		// no variable is used on this service
		if len(valuesMap) == 0 {
			continue
		}
		variableYaml, _ := yaml.Marshal(valuesMap)
		svc.VariableYaml = string(variableYaml)
		svcsNeedUpdate = append(svcsNeedUpdate, svc)
	}

	updater := &DataBulkUpdater{
		Coll:           mongodb.NewServiceColl().Collection,
		WriteThreshold: 20,
	}
	for _, svc := range svcsNeedUpdate {
		err = updater.AddModel(mongo.NewUpdateOneModel().
			SetFilter(bson.D{{"product_name", svc.ProductName}, {"service_name", svc.ServiceName}, {"revision", svc.Revision}}).
			SetUpdate(bson.D{{"$set",
				bson.D{
					{"variable_yaml", svc.VariableYaml},
				}},
			}))
		if err != nil {
			return fmt.Errorf("failed to write template service variable: %s", err)
		}
	}
	err = updater.Write()
	if err != nil {
		return fmt.Errorf("failed to write template service variable: %s", err)
	}

	return nil
}

func adjustProductRenderInfo() error {
	temProducts, err := templaterepo.NewProductColl().ListWithOption(&templaterepo.ProductListOpt{
		DeployType: setting.K8SDeployType,
	})
	if err != nil {
		log.Errorf("adjustProductRenderInfo list projects error: %s", err)
		return fmt.Errorf("adjustProductRenderInfo list projects error: %s", err)
	}
	projectNames := make([]string, 0)
	for _, v := range temProducts {
		projectNames = append(projectNames, v.ProductName)
	}

	products, err := uamongo.NewProductColl().List(&uamongo.ProductListOptions{ExcludeStatus: setting.ProductStatusDeleting, InProjects: projectNames})
	if err != nil {
		log.Errorf("adjustProductRenderInfo list product error: %s", err)
		return fmt.Errorf("adjustProductRenderInfo list product error: %s", err)
	}

	for _, product := range products {
		err = adjustSingleProductRender(product)
		if err != nil {
			return err
		}
	}
	return nil
}

// used product.render instead of product.service[].render
// convert current kvs in render into global data
func adjustSingleProductRender(product *uamodel.Product) error {

	var maxVersionRender *uamodel.RenderInfo = nil
	for _, svc := range product.GetServiceMap() {
		if svc.Render != nil && (maxVersionRender == nil || svc.Render.Revision > maxVersionRender.Revision) {
			maxVersionRender = svc.Render
		}
	}
	if product.Render != nil && (maxVersionRender == nil || product.Render.Revision > maxVersionRender.Revision) {
		maxVersionRender = product.Render
	}

	if maxVersionRender == nil {
		return nil
	}

	renderSet, err := uamongo.NewRenderSetColl().Find(&uamongo.RenderSetFindOption{
		ProductTmpl: product.ProductName,
		EnvName:     product.EnvName,
		IsDefault:   false,
		Revision:    maxVersionRender.Revision,
		Name:        maxVersionRender.Name,
	})
	if err != nil {
		log.Errorf("failed to find renderset info: %v/%v", renderSet.Name, renderSet.Revision)
		return err
	}

	// the render set has been handled
	if len(renderSet.DefaultValues) > 0 {
		return nil
	}

	if len(renderSet.KVs) == 0 {
		return setProductRender(product, maxVersionRender)
	}

	// turn current kv info into global variable-yaml
	valuesMap := make(map[string]interface{})
	for _, kv := range renderSet.KVs {
		valuesMap[kv.Key] = kv.Value
	}
	valuesYaml, _ := yaml.Marshal(valuesMap)
	renderSet.DefaultValues = string(valuesYaml)

	err = uamongo.NewRenderSetColl().UpdateDefaultValues(renderSet)
	if err != nil {
		return err
	}

	return setProductRender(product, maxVersionRender)
}

func setProductRender(product *uamodel.Product, maxVersionRender *uamodel.RenderInfo) error {
	// revisions of product.render and product.service[].render are the same
	if product.Render != nil && product.Render.Revision == maxVersionRender.Revision {
		return nil
	}
	product.Render = maxVersionRender
	return uamongo.NewProductColl().UpdateProductRender(product)
}
