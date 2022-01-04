/*
Copyright 2021 The KodeRover Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package service

import (
	"time"

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
	emailHost.CreatedAt = time.Now().Unix()
	emailHost.UpdatedAt = time.Now().Unix()
	return mongodb.NewEmailHostColl().Add(emailHost)
}

func UpdateEmailHost(emailHost *models.EmailHost, _ *zap.SugaredLogger) (*models.EmailHost, error) {
	emailHost.UpdatedAt = time.Now().Unix()
	return mongodb.NewEmailHostColl().Update(emailHost)
}

func DeleteEmailHost(_ *zap.SugaredLogger) error {
	return mongodb.NewEmailHostColl().Delete()
}

func GetEmailService(_ *zap.SugaredLogger) (*models.EmailService, error) {
	return mongodb.NewEmailServiceColl().GetEmailService()
}

func CreateEmailService(emailService *models.EmailService, _ *zap.SugaredLogger) (*models.EmailService, error) {
	emailService.CreatedAt = time.Now().Unix()
	emailService.UpdatedAt = time.Now().Unix()
	return mongodb.NewEmailServiceColl().AddEmailService(emailService)
}

func UpdateEmailService(emailService *models.EmailService, _ *zap.SugaredLogger) (*models.EmailService, error) {
	emailService.UpdatedAt = time.Now().Unix()
	return mongodb.NewEmailServiceColl().UpdateEmailService(emailService)
}

func DeleteEmailService(_ *zap.SugaredLogger) error {
	return mongodb.NewEmailServiceColl().DeleteEmailService()
}
