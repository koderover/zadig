package service

import (
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/email/repository/mongo"
	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/jira/repository/models"
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
