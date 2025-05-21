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
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	"gopkg.in/gomail.v2"

	"github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/email/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/email/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/shared/client/aslan"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	"github.com/koderover/zadig/v2/pkg/tool/crypto"
)

func GetEmailHost(log *zap.SugaredLogger) (*models.EmailHost, error) {
	host, err := mongodb.NewEmailHostColl().Find()
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		log.Errorf("failed to find email host, error: %s", err)
		return nil, err
	}

	host.Password = "***"

	return host, nil
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
		if err != mongo.ErrNoDocuments {
			log.Errorf("GetEncryptedEmailHost find email host error:%s", err)
			return nil, err
		}
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

func ValidateEmailHost(ctx *internalhandler.Context, emailHost *models.EmailHost) error {
	d := gomail.NewDialer(emailHost.Name, emailHost.Port, emailHost.Username, emailHost.Password)
	if _, err := d.Dial(); err != nil {
		return fmt.Errorf("验证失败，请检查邮箱配置, 错误: %s", err)
	}
	return nil
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
