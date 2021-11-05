package service

import (
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/email/repository/models"
	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/email/repository/mongodb"
)

func GetEmailHost(_ *zap.SugaredLogger) (*models.EmailHost, error) {
	host, err := mongodb.NewEmailHostColl().Find()
	if host != nil {
		host.Password = "***"
	}
	return host, err
}

func InternalGetEmailHost(_ *zap.SugaredLogger) (*models.EmailHost, error) {
	return mongodb.NewEmailHostColl().Find()
}

func CreateEmailHost(emailHost *models.EmailHost, _ *zap.SugaredLogger) (*models.EmailHost, error) {
	return mongodb.NewEmailHostColl().Add(emailHost)
}

func UpdateEmailHost(host *models.EmailHost, _ *zap.SugaredLogger) (*models.EmailHost, error) {
	return mongodb.NewEmailHostColl().Update(host)
}

func DeleteEmailHost(_ *zap.SugaredLogger) error {
	return mongodb.NewEmailHostColl().Delete()
}

func GetEmailService(_ *zap.SugaredLogger) (*models.EmailService, error) {
	return mongodb.NewEmailServiceColl().GetEmailService()
}

func CreateEmailService(service *models.EmailService, _ *zap.SugaredLogger) (*models.EmailService, error) {
	return mongodb.NewEmailServiceColl().AddEmailService(service)
}

func UpdateEmailService(service *models.EmailService, _ *zap.SugaredLogger) (*models.EmailService, error) {
	return mongodb.NewEmailServiceColl().UpdateEmailService(service)
}

func DeleteEmailService(_ *zap.SugaredLogger) error {
	return mongodb.NewEmailServiceColl().DeleteEmailService()
}
