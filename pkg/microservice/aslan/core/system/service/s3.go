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
	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/s3"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/errors"
	s3tool "github.com/koderover/zadig/pkg/tool/s3"
)

func UpdateS3Storage(updateBy, id string, storage *commonmodels.S3Storage, logger *zap.SugaredLogger) error {
	s3Storage := &s3.S3{S3Storage: storage}
	forcedPathStyle := false
	if s3Storage.Provider == setting.ProviderSourceSystemDefault {
		forcedPathStyle = true
	}
	client, err := s3tool.NewClient(s3Storage.Endpoint, s3Storage.Ak, s3Storage.Sk, s3Storage.Insecure, forcedPathStyle)
	if err != nil {
		logger.Warnf("Failed to create s3 client, error is: %+v", err)
		return errors.ErrValidateS3Storage.AddErr(err)
	}
	if err := client.ValidateBucket(storage.Bucket); err != nil {
		logger.Warnf("failed to validate storage %s %v", storage.Endpoint, err)
		return errors.ErrValidateS3Storage.AddErr(err)
	}

	storage.UpdatedBy = updateBy
	return commonrepo.NewS3StorageColl().Update(id, storage)
}

func CreateS3Storage(updateBy string, storage *commonmodels.S3Storage, logger *zap.SugaredLogger) error {
	s3Storage := &s3.S3{S3Storage: storage}
	forcedPathStyle := false
	if s3Storage.Provider == setting.ProviderSourceSystemDefault {
		forcedPathStyle = true
	}
	client, err := s3tool.NewClient(s3Storage.Endpoint, s3Storage.Ak, s3Storage.Sk, s3Storage.Insecure, forcedPathStyle)
	if err != nil {
		logger.Warnf("Failed to create s3 client, error is: %+v", err)
		return errors.ErrValidateS3Storage.AddErr(err)
	}
	if err := client.ValidateBucket(s3Storage.Bucket); err != nil {
		logger.Warnf("failed to validate storage %s %v", storage.Endpoint, err)
		return errors.ErrValidateS3Storage.AddErr(err)
	}

	storage.UpdatedBy = updateBy
	return commonrepo.NewS3StorageColl().Create(storage)
}

func ListS3Storage(logger *zap.SugaredLogger) ([]*commonmodels.S3Storage, error) {
	stores, err := commonrepo.NewS3StorageColl().FindAll()
	if err == nil && len(stores) == 0 {
		stores = make([]*commonmodels.S3Storage, 0)
	}

	return stores, err
}

func DeleteS3Storage(deleteBy string, id string, logger *zap.SugaredLogger) error {
	err := commonrepo.NewS3StorageColl().Delete(id)
	if err != nil {
		return err
	}

	logger.Infof("s3 storage %s is deleted by %s", id, deleteBy)
	return nil
}

func GetS3Storage(id string, logger *zap.SugaredLogger) (*commonmodels.S3Storage, error) {
	store, err := commonrepo.NewS3StorageColl().Find(id)
	if err != nil {
		logger.Errorf("can't find store by id %s", id)
		return nil, err
	}

	return store, nil
}
