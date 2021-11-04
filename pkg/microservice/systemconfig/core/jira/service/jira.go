package service

import (
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/repository/models"
	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/repository/mongo"
)

func GeJira(_ *zap.SugaredLogger) (*models.Jira, error) {
	return mongo.NewJiraColl().GetJira()
}

func CreateJira(jira *models.Jira, _ *zap.SugaredLogger) (*models.Jira, error) {
	return mongo.NewJiraColl().AddJira(jira)
}

func UpdateJira(jira *models.Jira, _ *zap.SugaredLogger) (*models.Jira, error) {
	return mongo.NewJiraColl().UpdateJira(jira)
}

func DeleteJira(_ *zap.SugaredLogger) error {
	return mongo.NewJiraColl().DeleteJira()
}
