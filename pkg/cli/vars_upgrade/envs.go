/*
Copyright 2023 The KodeRover Authors.

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

package vars_upgrade

import (
	"fmt"
	"strings"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"

	"k8s.io/apimachinery/pkg/util/sets"

	"gopkg.in/yaml.v3"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"

	templaterepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
)

func handlerEnvVars() error {
	temProducts, err := templaterepo.NewProductColl().ListWithOption(&templaterepo.ProductListOpt{
		DeployType:    setting.K8SDeployType,
		BasicFacility: setting.BasicFacilityK8S,
	})
	if err != nil {
		log.Errorf("handleServiceTemplates list projects error: %s", err)
		return fmt.Errorf("handleServiceTemplates list projects error: %s", err)
	}

	projectSets := sets.NewString()
	if len(appointedProjects) > 0 {
		projectSets.Insert(strings.Split(appointedProjects, ",")...)
	}
	filter := func(templateName string) bool {
		if len(projectSets) == 0 || projectSets.Has(templateName) {
			return true
		}
		return false
	}

	projectsMap := make(map[string]*template.Product)

	for _, project := range temProducts {
		if project.IsHostProduct() {
			continue
		}
		if !filter(project.ProductName) {
			continue
		}

		projectsMap[project.ProductName] = project
	}

	log.Infof("************** found %d yaml projects ", len(projectsMap))

	for _, project := range projectsMap {
		if err := handleSingleProjectEnvVars(project); err != nil {
			log.Error(err)
		}
	}

	return nil
}

func handleSingleProjectEnvVars(project *template.Product) error {
	log.Infof("-------- start handle products in project %s --------", project.ProductName)

	envs, err := mongodb.NewProductColl().List(&mongodb.ProductListOptions{
		Name: project.ProductName,
	})
	if err != nil {
		return fmt.Errorf("!!!!!!!!! failed to find envs in project %s, err: %s", project.ProductName, err)
	}
	for _, product := range envs {
		log.Infof("\n+++++++++++ handling product %s/%s +++++++++++", project.ProductName, product.EnvName)
		err = handleSingleProduct(product)
		if err != nil {
			log.Error(err)
		}
	}
	log.Infof("\n-------- finished handle project %s -----------\n\n", project.ProductName)
	return nil
}

func handleSingleProduct(product *models.Product) error {
	product.EnsureRenderInfo()
	usedRenderSet, exist, err := mongodb.NewRenderSetColl().FindRenderSet(&mongodb.RenderSetFindOption{
		ProductTmpl: product.ProductName,
		EnvName:     product.EnvName,
		IsDefault:   false,
		Revision:    product.Render.Revision,
		Name:        product.Render.Name,
	})
	if err != nil {
		return fmt.Errorf("!!!!!!!!! failed to find renderset of product %s/%s, err: %s", product.ProductName, product.EnvName, err)
	}
	if !exist {
		return fmt.Errorf("!!!!!!!!! failed to find renderset of product %s/%s, not exist", product.ProductName, product.EnvName)
	}
	//defaultValues := make(map[string]interface{})
	//err = yaml.Unmarshal([]byte(usedRenderSet.DefaultValues), &defaultValues)
	//if err != nil {
	//	return fmt.Errorf("!!!!!!!!! failed to unmarshal defaultValues of product %s/%s, err: %s", project.ProductName, product.EnvName, err)
	//}
	defaultValues := make(map[string]interface{})
	if len(usedRenderSet.KVs) > 0 {
		for _, kv := range usedRenderSet.KVs {
			defaultValues[kv.Key] = kv.Value
		}
	} else {
		err = yaml.Unmarshal([]byte(usedRenderSet.DefaultValues), &defaultValues)
		if err != nil {
			return fmt.Errorf("!!!!!!!!! failed to unmarshal defaultValues of product %s/%s, err: %s", product.ProductName, product.EnvName, err)
		}
	}

	handledKey := sets.NewString()
	for k := range defaultValues {
		//k := kv.Key
		if handledKey.Has(k) {
			continue
		}
		switch k {
		case "TOLERATION_KEY", "TOLERATION_VALUE":
			defaultValues["TOLERATIONS"] = []map[string]interface{}{
				{"KEY": defaultValues["TOLERATION_KEY"], "VALUE": defaultValues["TOLERATION_VALUE"]},
			}
			handledKey.Insert("TOLERATION_KEY", "TOLERATION_VALUE")
		case "NODE_AFFINITY_KEY", "NODE_AFFINITY_VALUE":
			defaultValues["NODE_AFFINITY"] = []map[string]interface{}{
				{"KEY": defaultValues["NODE_AFFINITY_KEY"], "VALUES": []interface{}{defaultValues["NODE_AFFINITY_VALUE"]}},
			}
			handledKey.Insert("NODE_AFFINITY_KEY", "NODE_AFFINITY_VALUE")
		}
	}
	bs, err := yaml.Marshal(defaultValues)
	if err != nil {
		return fmt.Errorf("!!!!!!!!! failed to marshal defaultValues of product %s/%s, err: %s", product.ProductName, product.EnvName, err)
	}
	bsStr := string(bs)
	if len(defaultValues) == 0 {
		bsStr = ""
	}
	log.Infof("product: %s/%s, defaultValues: %v", product.ProductName, product.EnvName, bsStr)
	if !write {
		return nil
	}
	usedRenderSet.DefaultValues = bsStr
	err = mongodb.NewRenderSetColl().Update(usedRenderSet)
	if err != nil {
		return fmt.Errorf("!!!!!!!!! failed to update renderset of product %s/%s, renderset: %s/%d, err: %s", product.ProductName, product.EnvName, usedRenderSet.Name, usedRenderSet.Revision, err)
	}
	return nil
}
