package util

import (
	"fmt"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
	"go.uber.org/zap"
)

func GenerateEnvServiceNextRevision(projectName, envName, serviceName string) (int64, error) {
	counterName := fmt.Sprintf(setting.EnvServiceVersionCounterName, projectName, envName, serviceName)
	return commonrepo.NewCounterColl().GetNextSeq(counterName)
}

func CreateEnvServiceVersion(env *models.Product, prodSvc *models.ProductService, createBy string, log *zap.SugaredLogger) error {
	revision, err := GenerateEnvServiceNextRevision(env.ProductName, env.EnvName, prodSvc.ServiceName)
	if err != nil {
		return fmt.Errorf("failed to generate service %s/%s/%s revision, error: %v", env.ProductName, env.EnvName, prodSvc.ServiceName, err)
	}
	version := &models.EnvServiceVersion{
		ProductName:     env.ProductName,
		EnvName:         env.EnvName,
		Namespace:       env.Namespace,
		Production:      env.Production,
		Revision:        revision,
		Service:         prodSvc,
		GlobalVariables: env.GlobalVariables,
		DefaultValues:   env.DefaultValues,
		YamlData:        env.YamlData,
		CreateBy:        createBy,
	}
	err = mongodb.NewEnvServiceVersionColl().Create(version)
	if err != nil {
		return fmt.Errorf("failed to create service %s/%s/%s version %d, error: %v", env.ProductName, env.EnvName, prodSvc.ServiceName, revision, err)
	}

	log.Infof("Create environment service version for %s/%s/%s revision %d", env.ProductName, env.EnvName, prodSvc.ServiceName, revision)

	return nil
}
