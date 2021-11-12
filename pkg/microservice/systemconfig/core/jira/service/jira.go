package service

import (
	"time"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/jira/repository/models"
	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/jira/repository/mongodb"
)

func GeJira(_ *zap.SugaredLogger) (*models.Jira, error) {
	return mongodb.NewJiraColl().GetJira()
}

func CreateJira(jira *models.Jira, _ *zap.SugaredLogger) (*models.Jira, error) {
	jira.CreatedAt = time.Now().Unix()
	jira.UpdatedAt = time.Now().Unix()
	return mongodb.NewJiraColl().AddJira(jira)
}

func UpdateJira(jira *models.Jira, _ *zap.SugaredLogger) (*models.Jira, error) {
	jira.UpdatedAt = time.Now().Unix()
	return mongodb.NewJiraColl().UpdateJira(jira)
}

func DeleteJira(_ *zap.SugaredLogger) error {
	return mongodb.NewJiraColl().DeleteJira()
}
