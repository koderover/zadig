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

	"github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/email/repository/models"
	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/email/repository/mongodb"
	"github.com/koderover/zadig/pkg/shared/client/aslan"
	"github.com/koderover/zadig/pkg/tool/crypto"
)

func GetEmailHost(_ *zap.SugaredLogger) (*models.EmailHost, error) {
	host, err := mongodb.NewEmailHostColl().Find()
	if host != nil {
		host.Password = "***"
	}
	return host, err
}

func GetEmailHostInternal(_ *zap.SugaredLogger) (*models.EmailHost, error) {
	return mongodb.NewEmailHostColl().Find()
}

func GetEncryptedEmailHost(encryptedKey string, log *zap.SugaredLogger) (*models.EmailHost, error) {
	aesKey, err := aslan.New(config.AslanServiceAddress()).GetTextFromEncryptedKey(encryptedKey)
	if err != nil {
		log.Errorf("GetEncryptedEmailHost GetTextFromEncryptedKey error:%s", err)
		return nil, err
	}
	result, err := mongodb.NewEmailHostColl().Find()
	if err != nil {
		log.Errorf("GetEncryptedEmailHost find email host error:%s", err)
		return nil, err
	}
	if result == nil {
		return nil, nil
	}
	password, err := crypto.AesEncryptByKey(result.Password, aesKey.PlainText)
	if err != nil {
		log.Errorf("GetEncryptedEmailHost AesEncryptByKey error:%s", err)
		return nil, err
	}
	result.Password = password
	return result, nil
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
