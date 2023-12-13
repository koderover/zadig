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

package migrate

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/upgradepath"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/repository"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/util"
)

func init() {
	allSvcs = make(map[string]*models.Service)
	upgradepath.RegisterHandler("2.0.0", "2.1.0", V200ToV210)
	upgradepath.RegisterHandler("2.1.0", "2.0.0", V210ToV200)
}

func V200ToV210() error {
	err := addResourceTOK8sProductServices()
	if err != nil {
		return err
	}

	err = addReleaseNameToHelmProductServices()
	if err != nil {
		return err
	}
	return nil
}

func V210ToV200() error {
	return nil
}

var allSvcs map[string]*models.Service

func findSvcTemplate(productName, serviceName string, revision int64, production bool) (*models.Service, error) {
	svcKey := fmt.Sprintf("%s/%s/%v/%v", productName, serviceName, revision, production)
	if svc, ok := allSvcs[svcKey]; ok {
		return svc, nil
	}

	svcInfo, err := repository.QueryTemplateService(&mongodb.ServiceFindOption{
		ServiceName: serviceName,
		Revision:    revision,
		ProductName: productName,
	}, production)
	if err != nil {
		return nil, err
	}

	allSvcs[svcKey] = svcInfo
	return svcInfo, nil
}

func addResourceTOK8sProductServices() error {
	allProjects, err := template.NewProductColl().ListWithOption(&template.ProductListOpt{
		DeployType:    setting.K8SDeployType,
		BasicFacility: setting.BasicFacilityK8S,
	})
	if err != nil {
		return errors.Wrapf(err, "list k8s projects")
	}

	for _, project := range allProjects {
		if project.IsHostProduct() {
			continue
		}
		envs, err := mongodb.NewProductColl().List(&mongodb.ProductListOptions{
			Name: project.ProductName,
		})
		if err != nil {
			return errors.Wrapf(err, "get envs, project name: %s", project.ProductName)
		}
		for _, env := range envs {
			err = handleK8sProductService(env)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func addReleaseNameToHelmProductServices() error {
	allProjects, err := template.NewProductColl().ListWithOption(&template.ProductListOpt{
		DeployType: setting.HelmDeployType,
	})
	if err != nil {
		return errors.Wrapf(err, "list helm projects")
	}

	for _, project := range allProjects {
		envs, err := mongodb.NewProductColl().List(&mongodb.ProductListOptions{
			Name: project.ProductName,
		})
		if err != nil {
			return errors.Wrapf(err, "get envs, project name: %s", project.ProductName)
		}
		for _, env := range envs {
			err = handleHelmProductService(env)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func handleK8sProductService(product *models.Product) error {
	for _, svc := range product.GetSvcList() {
		if len(svc.Resources) > 0 {
			continue
		}

		prodSvcTemplate, err := findSvcTemplate(product.ProductName, svc.ServiceName, svc.Revision, product.Production)
		if err != nil {
			log.Errorf("failed to get svc template: %s/%s/%v, err: %s", product.ProductName, svc.ServiceName, svc.Revision, err.Error())
			continue
		}

		fullRenderedYaml, err := kube.RenderServiceYaml(prodSvcTemplate.Yaml, product.ProductName, svc.ServiceName, svc.GetServiceRender())
		if err != nil {
			log.Errorf("failed to render service yaml: %s/%s/%s/%v, err: %s", product.ProductName, product.EnvName, svc.ServiceName, svc.Revision, err.Error())
			continue
		}
		fullRenderedYaml = kube.ParseSysKeys(product.EnvName, product.EnvName, product.ProductName, svc.ServiceName, fullRenderedYaml)

		svc.Resources, err = kube.ManifestToResource(fullRenderedYaml)
		if err != nil {
			log.Errorf("failed to convert service yaml to resource: %s/%s/%s/%v, err: %s", product.ProductName, product.EnvName, svc.ServiceName, svc.Revision, err.Error())
			continue
		}
		log.Infof("update service resource: %s/%s/%v", product.ProductName, product.EnvName, svc.ServiceName)
	}
	return mongodb.NewProductColl().Update(product)
}

func handleHelmProductService(product *models.Product) error {
	for _, svc := range product.GetSvcList() {
		if !svc.FromZadig() {
			continue
		}
		prodSvcTemplate, err := findSvcTemplate(product.ProductName, svc.ServiceName, svc.Revision, product.Production)
		if err != nil {
			log.Errorf("failed to get svc template: %s/%s/%v, err: %s", product.ProductName, svc.ServiceName, svc.Revision, err.Error())
			continue
		}
		svc.ReleaseName = util.GeneReleaseName(prodSvcTemplate.GetReleaseNaming(), product.ProductName, product.Namespace, product.EnvName, svc.ServiceName)
		log.Infof("update service release: %s/%s/%v", product.ProductName, product.EnvName, svc.ServiceName)
	}
	return mongodb.NewProductColl().Update(product)
}
