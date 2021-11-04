package service

import (
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/email/models"
	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/repository/mongo"
)

func GetEmailHost(_ *zap.SugaredLogger) (*models.EmailHost, error) {
	return mongo.NewEmailHostColl().Find()
}

func CreateEmailHost(emailHost *models.EmailHost, _ *zap.SugaredLogger) (*models.EmailHost, error) {
	return mongo.NewEmailHostColl().Add(emailHost)
}

func UpdateEmailHost(host *models.EmailHost, _ *zap.SugaredLogger) (*models.EmailHost, error) {
	return mongo.NewEmailHostColl().Update(host)
}

func DeleteEmailHost(_ *zap.SugaredLogger) error {
	return mongo.NewEmailHostColl().Delete()
}

func GetEmailService(_ *zap.SugaredLogger) (*models.EmailService, error) {
	return mongo.NewEmailServiceColl().GetEmailService()
}

func CreateEmailService(service *models.EmailService, _ *zap.SugaredLogger) (*models.EmailService, error) {
	return mongo.NewEmailServiceColl().AddEmailService(service)
}

func UpdateEmailService(service *models.EmailService, _ *zap.SugaredLogger) (*models.EmailService, error) {
	return mongo.NewEmailServiceColl().UpdateEmailService(service)
}

func DeleteEmailService(_ *zap.SugaredLogger) error {
	return mongo.NewEmailServiceColl().DeleteEmailService()
}
