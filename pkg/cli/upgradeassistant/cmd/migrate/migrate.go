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

package migrate

import (
	"fmt"

	"github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/upgradepath"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	templatemodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
)

const oldServiceTemplateCounterName = "service:%s&type:%s"

func init() {
	upgradepath.AddHandler(upgradepath.V130, upgradepath.V131, V130ToV131)
	// upgradepath.AddHandler(upgradepath.V131, upgradepath.V130, V131ToV130)
}

// V130ToV131 migrates data from v1.3.0 to v1.3.1 with the following tasks:
// 1. Add field `SharedServices` for all projects
// 2. Add field `ProductName` in field `Services` for all envs
// 3. Change the ServiceTemplateCounterName format
func V130ToV131() error {
	log.Info("Migrating data from 1.3.0 to 1.3.1")

	allServices, err := mongodb.NewServiceColl().ListMaxRevisions(nil)
	if err != nil {
		log.Errorf("Failed to get services, err: %s", err)
		return err
	}

	if skipMigration(allServices, setting.ServiceTemplateCounterName) {
		log.Info("Migration skipped")
		return nil
	}

	allProjects, err := templaterepo.NewProductColl().List()
	if err != nil {
		log.Errorf("Failed to get projects, err: %s", err)
		return err
	}
	allEnvs, err := mongodb.NewProductColl().List(&mongodb.ProductListOptions{ExcludeStatus: setting.ProductStatusDeleting})
	if err != nil {
		log.Errorf("Failed to get envs, err: %s", err)
		return err
	}

	var updatedProjects []*templatemodels.Product
	var updatedEnvs []*models.Product

	// service name is unique before current version
	serviceMap := make(map[string]*models.Service)
	for _, s := range allServices {
		serviceMap[s.ServiceName] = s
	}

	// if items in serviceMap is less than allServices (which means that there are more than one services with same name),
	// we should stop here since the logic below may cause unexpected effects.
	if len(serviceMap) < len(allServices) {
		log.Info("Migration skipped")
		return nil
	}

	// add field `SharedServices` for all projects
	for _, project := range allProjects {
		var sharedServices []*templatemodels.ServiceInfo
		services := project.AllServiceInfoMap()
		for name := range services {
			service := serviceMap[name]
			if service == nil {
				continue
			}
			if service.ProductName != project.ProductName {
				sharedServices = append(sharedServices, &templatemodels.ServiceInfo{Name: name, Owner: service.ProductName})
			}
		}

		if len(sharedServices) > 0 {
			project.SharedServices = sharedServices
			updatedProjects = append(updatedProjects, project)
		}

	}

	// add field `ProductName` in field `Services` for all envs
	for _, env := range allEnvs {
		for _, group := range env.Services {
			for _, s := range group {
				service := serviceMap[s.ServiceName]
				if service == nil {
					continue
				}
				s.ProductName = service.ProductName
			}
		}

		updatedEnvs = append(updatedEnvs, env)
	}

	if err = templaterepo.NewProductColl().UpdateAll(updatedProjects); err != nil {
		log.Errorf("Failed to upgrade projects, err: %s", err)
		return err
	}

	if err = mongodb.NewProductColl().UpdateAll(updatedEnvs); err != nil {
		log.Errorf("Failed to upgrade envs, err: %s", err)
		return err
	}

	if err = UpdateServiceCounter(allServices); err != nil {
		log.Errorf("Failed to upgrade counters, err: %s", err)
		return err
	}

	return nil
}

// V131ToV130 rollbacks the changes from v1.3.1 to v1.3.0 with the following tasks:
// 1. Remove field `SharedServices` for all projects
// 2. Remove field `ProductName` in field `Services` for all envs
// 3. Revert the ServiceTemplateCounterName format
func V131ToV130() error {
	log.Info("Rollback data from 1.3.1 to 1.3.0")

	allServices, err := mongodb.NewServiceColl().ListMaxRevisions(nil)
	if err != nil {
		log.Errorf("Failed to get services, err: %s", err)
		return err
	}

	if skipMigration(allServices, oldServiceTemplateCounterName) {
		log.Info("Migration skipped")
		return nil
	}

	allProjects, err := templaterepo.NewProductColl().List()
	if err != nil {
		log.Errorf("Failed to get projects, err: %s", err)
		return err
	}
	allEnvs, err := mongodb.NewProductColl().List(&mongodb.ProductListOptions{ExcludeStatus: setting.ProductStatusDeleting})
	if err != nil {
		log.Errorf("Failed to get envs, err: %s", err)
		return err
	}

	var updatedProjects []*templatemodels.Product
	var updatedEnvs []*models.Product

	// clear field `SharedServices` for all projects
	for _, project := range allProjects {
		if len(project.SharedServices) > 0 {
			project.SharedServices = []*templatemodels.ServiceInfo{}
			updatedProjects = append(updatedProjects, project)
		}
	}

	// clear field `ProductName` in field `Services` for all envs
	for _, env := range allEnvs {
		for _, group := range env.Services {
			for _, s := range group {
				s.ProductName = ""
			}
		}

		updatedEnvs = append(updatedEnvs, env)
	}

	if err = templaterepo.NewProductColl().UpdateAll(updatedProjects); err != nil {
		log.Errorf("Failed to rollback projects, err: %s", err)
		return err
	}

	if err = mongodb.NewProductColl().UpdateAll(updatedEnvs); err != nil {
		log.Errorf("Failed to rollback envs, err: %s", err)
		return err
	}

	if err = RevertServiceCounter(allServices); err != nil {
		log.Errorf("Failed to rollback counters, err: %s", err)
		return err
	}

	return nil
}

func UpdateServiceCounter(allServices []*models.Service) error {
	return updateServiceCounter(allServices, oldServiceTemplateCounterName, setting.ServiceTemplateCounterName)
}

func RevertServiceCounter(allServices []*models.Service) error {
	return updateServiceCounter(allServices, setting.ServiceTemplateCounterName, oldServiceTemplateCounterName)
}

func updateServiceCounter(allServices []*models.Service, oldTemplate, newTemplate string) error {
	coll := mongodb.NewCounterColl()
	for _, s := range allServices {
		oldName := fmt.Sprintf(oldTemplate, s.ServiceName, s.Type)
		newName := fmt.Sprintf(newTemplate, s.ServiceName, s.ProductName)
		err := coll.Rename(oldName, newName)
		if err != nil {
			return err
		}
	}

	return nil
}

func skipMigration(allServices []*models.Service, newTemplate string) bool {
	if len(allServices) == 0 {
		return true
	}

	newName := fmt.Sprintf(newTemplate, allServices[0].ServiceName, allServices[0].ProductName)
	// skip if any new names exist (means upgrade is already done).
	if counter, _ := mongodb.NewCounterColl().Find(newName); counter != nil {
		return true
	}

	return false
}
