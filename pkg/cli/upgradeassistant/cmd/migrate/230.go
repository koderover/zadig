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
	"time"

	"github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/upgradepath"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/sets"
)

func init() {
	upgradepath.RegisterHandler("2.2.0", "2.3.0", V220ToV230)
	upgradepath.RegisterHandler("2.3.0", "2.2.0", V230ToV220)
}

func V220ToV230() error {
	log.Infof("-------- start migrate host project data --------")
	err := migrateHostProjectData()
	time.Sleep(time.Second * 100)
	if err != nil {
		log.Errorf("migrateHostProjectData error: %s", err)
		return err
	}

	return nil
}

func V230ToV220() error {
	return nil
}

func migrateHostProjectData() error {

	allProjects, err := template.NewProductColl().ListWithOption(&template.ProductListOpt{
		DeployType:    setting.K8SDeployType,
		BasicFacility: setting.BasicFacilityK8S,
	})

	if err != nil {
		return errors.WithMessage(err, "failed to list all projects")
	}

	for _, project := range allProjects {
		if !project.IsHostProduct() {
			continue
		}
		templateServices, err := commonrepo.NewServiceColl().ListMaxRevisionsByProduct(project.ProductName)
		if err != nil {
			return errors.WithMessagef(err, "failed to list services for product %s", project.ProductName)
		}
		tempSvcMap := make(map[string]*models.Service)
		for _, svc := range templateServices {
			tempSvcMap[svc.ServiceName] = svc
		}

		//getSvcRevision := func(svcName string) int64 {
		//	if svc, ok := tempSvcMap[svcName]; ok {
		//		return svc.Revision
		//	}
		//	return 1
		//}

		products, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
			Name: project.ProductName,
		})
		if err != nil {
			return errors.WithMessagef(err, "failed to find product %s, err: %s", project.ProductName, err)
		}

		for _, product := range products {
			// product data has been handled
			if len(product.Services) > 0 {
				// continue
			}

			log.Infof("------- handling single data for product %s, env %s -------", product.ProductName, product.EnvName)

			productServices, err := commonrepo.NewServiceColl().ListExternalWorkloadsBy(project.ProductName, product.EnvName)
			if err != nil {
				log.Errorf("ListWorkloadDetails ListExternalServicesBy err:%s", err)
				return errors.Wrapf(err, "failed to list external services for product %s", project.ProductName)
			}

			log.Debugf("1")
			servicesInExternalEnv, _ := commonrepo.NewServicesInExternalEnvColl().List(&commonrepo.ServicesInExternalEnvArgs{
				ProductName: project.ProductName,
				EnvName:     product.EnvName,
			})
			log.Debugf("2")

			svcNameList := sets.NewString()
			for _, singleProductSvc := range productServices {
				svcNameList.Insert(singleProductSvc.ServiceName)
			}
			for _, singleSvc := range servicesInExternalEnv {
				svcNameList.Insert(singleSvc.ServiceName)
			}

			log.Debugf("3")
			// fetch workload from namespace and extract resource / container info
			// note the image data in container may not be correct
			productSvcs := make([]*models.ProductService, 0)

			for _, templateSvc := range tempSvcMap {
				if !svcNameList.Has(templateSvc.ServiceName) {
					continue
				}

				log.Debugf("4")
				resources, err := kube.ManifestToResource(templateSvc.Yaml)
				if err != nil {
					log.Errorf("ManifestToResource err:%s", err)
					continue
				}
				log.Debugf("5")

				containers := make([]*models.Container, 0)
				for _, c := range templateSvc.Containers {
					containers = append(containers, &models.Container{
						Name:      c.Name,
						Image:     c.Image,
						ImageName: c.ImageName,
					})
				}

				productSvc := &models.ProductService{
					ServiceName:    templateSvc.ServiceName,
					ProductName:    product.ProductName,
					Type:           templateSvc.WorkloadType,
					Revision:       templateSvc.Revision,
					Containers:     containers,
					Resources:      resources,
					DeployStrategy: setting.ServiceDeployStrategyDeploy,
				}

				productSvc.GetServiceRender()
				productSvcs = append(productSvcs, productSvc)
				log.Debugf("6")
			}

			log.Debugf("7")
			product.Services = make([][]*models.ProductService, 0)
			product.Services = append(product.Services, productSvcs)

			err = commonrepo.NewProductColl().Update(product)
			if err != nil {
				log.Errorf("Failed to update product %s, the error is: %s", product.ProductName, err)
				continue
			}
			log.Debugf("8")
		}
	}
	return nil
}
