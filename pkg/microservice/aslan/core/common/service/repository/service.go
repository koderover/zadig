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

package repository

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

type IServiceColl interface {
	Find(option *mongodb.ServiceFindOption) (*models.Service, error)
	Delete(serviceName, serviceType, productName, status string, revision int64) error
	Create(args *models.Service) error
}

func ServiceCollWithSession(production bool, session mongo.Session) IServiceColl {
	var inner IServiceColl
	if !production {
		inner = mongodb.NewServiceCollWithSession(session)
	} else {
		inner = mongodb.NewProductionServiceCollWithSession(session)
	}
	return &serviceCollWithModuleSync{IServiceColl: inner, production: production}
}

// serviceCollWithModuleSync wraps an IServiceColl with side-effects that keep
// the service_module collection in sync. The wrapper is transparent for
// reads; Create / Delete are intercepted to mirror auto records into the new
// table (best-effort during Phase 3 — old field stays authoritative).
//
// Caveat: the wrapped writes here happen outside the mongo session if the
// underlying coll was constructed with one. A transaction rollback on the
// inner Create will leave the synced auto records in place. Acceptable while
// the new table is non-authoritative; revisit once the read switch lands.
type serviceCollWithModuleSync struct {
	IServiceColl
	production bool
}

func (s *serviceCollWithModuleSync) Create(args *models.Service) error {
	if err := s.IServiceColl.Create(args); err != nil {
		return err
	}
	syncAutoModulesBestEffort(args, s.production, "Create")
	return nil
}

func (s *serviceCollWithModuleSync) Delete(serviceName, serviceType, productName, status string, revision int64) error {
	if err := s.IServiceColl.Delete(serviceName, serviceType, productName, status, revision); err != nil {
		return err
	}
	if revision > 0 {
		if dErr := DeleteAutoServiceModulesForRevision(context.Background(), productName, serviceName, s.production, revision); dErr != nil {
			log.Warnf("service_module: Delete failed to drop auto records for %s/%s rev %d: %s", productName, serviceName, revision, dErr)
		}
	}
	return nil
}

func QueryTemplateService(option *mongodb.ServiceFindOption, production bool) (*models.Service, error) {
	if !production {
		return mongodb.NewServiceColl().Find(option)
	} else {
		return mongodb.NewProductionServiceColl().Find(option)
	}
}

func QueryTemplateServiceWithSession(option *mongodb.ServiceFindOption, production bool, session mongo.Session) (*models.Service, error) {
	if !production {
		return mongodb.NewServiceCollWithSession(session).Find(option)
	} else {
		return mongodb.NewProductionServiceCollWithSession(session).Find(option)
	}
}

func ListServicesWithSRevision(option *mongodb.SvcRevisionListOption, production bool) ([]*models.Service, error) {
	if !production {
		return mongodb.NewServiceColl().ListServicesWithSRevision(option)
	} else {
		return mongodb.NewProductionServiceColl().ListServicesWithSRevision(option)
	}
}

func ListMaxRevisions(opt *mongodb.ServiceListOption, production bool) ([]*models.Service, error) {
	if !production {
		return mongodb.NewServiceColl().ListMaxRevisions(opt)
	} else {
		return mongodb.NewProductionServiceColl().ListMaxRevisions(opt)
	}
}

func ListMaxRevisionsServices(productName string, production bool, removeApplicationLinked bool) ([]*models.Service, error) {
	if !production {
		return mongodb.NewServiceColl().ListMaxRevisionsByProductWithFilter(productName, removeApplicationLinked)
	} else {
		return mongodb.NewProductionServiceColl().ListMaxRevisionsByProductWithFilter(productName, removeApplicationLinked)
	}
}

func ListMaxRevisionsServicesWithSession(productName string, production bool, session mongo.Session) ([]*models.Service, error) {
	if !production {
		return mongodb.NewServiceCollWithSession(session).ListMaxRevisionsByProduct(productName)
	} else {
		return mongodb.NewProductionServiceCollWithSession(session).ListMaxRevisionsByProduct(productName)
	}
}

func GetMaxRevisionsServicesMap(productName string, production bool) (map[string]*models.Service, error) {
	svcMap := make(map[string]*models.Service)
	services, err := ListMaxRevisionsServices(productName, production, false)
	if err != nil {
		return nil, err
	}

	for _, svc := range services {
		svcMap[svc.ServiceName] = svc
	}

	return svcMap, nil
}

func UpdateServiceVariables(args *models.Service, production bool) error {
	if !production {
		return mongodb.NewServiceColl().UpdateServiceVariables(args)
	} else {
		return mongodb.NewProductionServiceColl().UpdateServiceVariables(args)
	}
}

func UpdateServiceContainers(args *models.Service, production bool) error {
	var err error
	if !production {
		err = mongodb.NewServiceColl().UpdateServiceContainers(args)
	} else {
		err = mongodb.NewProductionServiceColl().UpdateServiceContainers(args)
	}
	if err != nil {
		return err
	}
	syncAutoModulesBestEffort(args, production, "UpdateServiceContainers")
	return nil
}

func UpdateStatus(serviceName, productName, status string, production bool) error {
	if !production {
		return mongodb.NewServiceColl().UpdateStatus(serviceName, productName, status)
	} else {
		return mongodb.NewProductionServiceColl().UpdateStatus(serviceName, productName, status)
	}
}

func Update(service *models.Service, production bool) error {
	if !production {
		return mongodb.NewServiceColl().Update(service)
	} else {
		return mongodb.NewProductionServiceColl().Update(service)
	}
}

func UpdateWithSession(service *models.Service, production bool, session mongo.Session) error {
	if !production {
		return mongodb.NewServiceCollWithSession(session).Update(service)
	} else {
		return mongodb.NewProductionServiceCollWithSession(session).Update(service)
	}
}

func Create(service *models.Service, production bool) error {
	var err error
	if !production {
		err = mongodb.NewServiceColl().Create(service)
	} else {
		err = mongodb.NewProductionServiceColl().Create(service)
	}
	if err != nil {
		return err
	}
	syncAutoModulesBestEffort(service, production, "Create")
	return nil
}

func Delete(serviceName, serviceType, productName, status string, revision int64, production bool) error {
	var err error
	if !production {
		err = mongodb.NewServiceColl().Delete(serviceName, serviceType, productName, status, revision)
	} else {
		err = mongodb.NewProductionServiceColl().Delete(serviceName, serviceType, productName, status, revision)
	}
	if err != nil {
		return err
	}
	// Per-revision delete — drop the matching auto records, leave manual alone.
	// Revision 0 means "all revisions" in some callers; skip the new-table
	// cleanup in that case to avoid wiping cross-revision auto data, which
	// DeleteAllServiceModulesForService should handle instead.
	if revision > 0 {
		if dErr := DeleteAutoServiceModulesForRevision(context.Background(), productName, serviceName, production, revision); dErr != nil {
			log.Warnf("service_module: failed to delete auto records for %s/%s rev %d: %s", productName, serviceName, revision, dErr)
		}
	}
	return nil
}

// syncAutoModulesBestEffort writes the auto-discovered modules to the new
// service_module collection. Failure is logged but not propagated — during
// Phase 3 the legacy Service.Containers field remains authoritative; the new
// table is being populated for the read-path switch later.
func syncAutoModulesBestEffort(svc *models.Service, production bool, callSite string) {
	if err := SyncAutoServiceModules(context.Background(), svc, production); err != nil {
		log.Warnf("service_module: %s failed to sync auto modules for %s/%s rev %d: %s",
			callSite, svc.ProductName, svc.ServiceName, svc.Revision, err)
	}
}
