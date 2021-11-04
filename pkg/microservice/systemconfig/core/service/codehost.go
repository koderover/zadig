package service

import (
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/repository/models"
	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/repository/mongo"
)

func CreateCodehost(codehost *models.CodeHost, _ *zap.SugaredLogger) (*models.CodeHost, error) {
	return mongo.NewCodehostColl().AddCodeHost(codehost)
}

func FindCodehost(_ *zap.SugaredLogger) ([]*models.CodeHost, error) {
	return mongo.NewCodehostColl().FindCodeHosts()
}
