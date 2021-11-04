package service

import (
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/email/repository/models"
	mongo2 "github.com/koderover/zadig/pkg/microservice/systemconfig/core/email/repository/mongo"
)

func GetEmailHost(_ *zap.SugaredLogger) (*models.EmailHost, error) {
	return mongo2.NewEmailHostColl().Find()
}

func CreateEmailHost(emailHost *models.EmailHost, _ *zap.SugaredLogger) (*models.EmailHost, error) {
	return mongo2.NewEmailHostColl().Add(emailHost)
}

func UpdateEmailHost(host *models.EmailHost, _ *zap.SugaredLogger) (*models.EmailHost, error) {
	return mongo2.NewEmailHostColl().Update(host)
}

func DeleteEmailHost(_ *zap.SugaredLogger) error {
	return mongo2.NewEmailHostColl().Delete()
}

func GetEmailService(_ *zap.SugaredLogger) (*models.EmailService, error) {
	return mongo2.NewEmailServiceColl().GetEmailService()
}

func CreateEmailService(service *models.EmailService, _ *zap.SugaredLogger) (*models.EmailService, error) {
	return mongo2.NewEmailServiceColl().AddEmailService(service)
}

func UpdateEmailService(service *models.EmailService, _ *zap.SugaredLogger) (*models.EmailService, error) {
	return mongo2.NewEmailServiceColl().UpdateEmailService(service)
}

func DeleteEmailService(_ *zap.SugaredLogger) error {
	return mongo2.NewEmailServiceColl().DeleteEmailService()
}
