package util

import (
	"fmt"

	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/repository"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/util"
)

func GenerateEnvServiceNextRevision(projectName, envName, serviceName string, isHelmChart bool, session mongo.Session) (int64, error) {
	counterName := fmt.Sprintf(setting.EnvServiceVersionCounterName, projectName, envName, serviceName, isHelmChart)
	return commonrepo.NewCounterCollWithSession(session).GetNextSeq(counterName)
}

func CreateEnvServiceVersion(env *models.Product, prodSvc *models.ProductService, createBy string, operation config.EnvOperation, detail string, session mongo.Session, log *zap.SugaredLogger) error {
	name := prodSvc.ServiceName
	isHelmChart := !prodSvc.FromZadig()
	if isHelmChart {
		name = prodSvc.ReleaseName
	} else if prodSvc.Type == setting.HelmDeployType {
		tmplSvc, err := repository.QueryTemplateService(&commonrepo.ServiceFindOption{
			ServiceName: prodSvc.ServiceName,
			Revision:    prodSvc.Revision,
			ProductName: prodSvc.ProductName,
		}, env.Production)
		if err != nil {
			return fmt.Errorf("failed to get template service %s/%s/%s revision %d, isProduction %v, error: %v", env.ProductName, env.EnvName, prodSvc.ServiceName, prodSvc.Revision, env.Production, err)
		}

		releaseName := util.GeneReleaseName(tmplSvc.GetReleaseNaming(), env.ProductName, env.Namespace, env.EnvName, tmplSvc.ServiceName)
		prodSvc.ReleaseName = releaseName
	}

	revision, err := GenerateEnvServiceNextRevision(env.ProductName, env.EnvName, name, isHelmChart, session)
	if err != nil {
		return fmt.Errorf("failed to generate service %s/%s/%s revision, error: %v", env.ProductName, env.EnvName, name, err)
	}

	svcVersionColl := mongodb.NewEnvServiceVersionCollWithSession(session)
	version := &models.EnvServiceVersion{
		ProductName:     env.ProductName,
		EnvName:         env.EnvName,
		Namespace:       env.Namespace,
		Production:      env.Production,
		Revision:        revision,
		Service:         prodSvc,
		Operation:       operation,
		Detail:          detail,
		GlobalVariables: env.GlobalVariables,
		DefaultValues:   env.DefaultValues,
		YamlData:        env.YamlData,
		CreateBy:        createBy,
	}
	err = svcVersionColl.Create(version)
	if err != nil {
		return fmt.Errorf("failed to create service %s/%s/%s version %d, error: %v", env.ProductName, env.EnvName, name, revision, err)
	}

	log.Infof("Create environment service version for %s/%s/%s revision %d, isHelmChart %v", env.ProductName, env.EnvName, name, revision, isHelmChart)

	if revision > 20 {
		// delete old version
		err = svcVersionColl.DeleteRevisions(env.ProductName, env.EnvName, name, isHelmChart, env.Production, revision-20)
		if err != nil {
			log.Errorf("failed to delete service %s/%s/%s version less equal than %d, error: %v", env.ProductName, env.EnvName, name, revision-20, err)
		}
	}

	return nil
}
