package service

import (
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/jira/repository/models"
	mongo2 "github.com/koderover/zadig/pkg/microservice/systemconfig/core/jira/repository/mongo"
)

func GeJira(_ *zap.SugaredLogger) (*models.Jira, error) {
	return mongo2.NewJiraColl().GetJira()
}

func CreateJira(jira *models.Jira, _ *zap.SugaredLogger) (*models.Jira, error) {
	return mongo2.NewJiraColl().AddJira(jira)
}

func UpdateJira(jira *models.Jira, _ *zap.SugaredLogger) (*models.Jira, error) {
	return mongo2.NewJiraColl().UpdateJira(jira)
}

func DeleteJira(_ *zap.SugaredLogger) error {
	return mongo2.NewJiraColl().DeleteJira()
}
