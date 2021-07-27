package service

import (
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	templatemodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/pkg/setting"
)

func DataMigrate() error {
	allServices, err := mongodb.NewServiceColl().ListMaxRevisions(nil)
	if err != nil {
		return err
	}
	allProjects, err := templaterepo.NewProductColl().List()
	if err != nil {
		return err
	}
	allEnvs, err := mongodb.NewProductColl().List(&mongodb.ProductListOptions{ExcludeStatus: setting.ProductStatusDeleting})
	if err != nil {
		return err
	}

	var updatedProjects []*templatemodels.Product
	var updatedEnvs []*models.Product

	// service name is unique before current version
	serviceMap := make(map[string]*models.Service)
	for _, s := range allServices {
		serviceMap[s.ServiceName] = s
	}

	// update field `SharedServices` for all projects
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

	// update field `ProductName` in field `Services` for all envs
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

	err = templaterepo.NewProductColl().UpdateAll(updatedProjects)
	if err != nil {
		return err
	}

	return mongodb.NewProductColl().UpdateAll(updatedEnvs)
}
