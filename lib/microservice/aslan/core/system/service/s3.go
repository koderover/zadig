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
	commonmodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	commonrepo "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/service/s3"
	"github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/tool/xlog"
)

func UpdateS3Storage(updateBy, id string, storage *commonmodels.S3Storage, logger *xlog.Logger) (*commonmodels.S3Storage, error) {
	s3Storage := &s3.S3{S3Storage: storage}
	if err := s3.Validate(s3Storage); err != nil {
		logger.Warnf("failed to validate storage %s %v", storage.Endpoint, err)
		return nil, errors.ErrValidateS3Storage.AddErr(err)
	}

	storage.UpdatedBy = updateBy
	err := commonrepo.NewS3StorageColl().Update(id, storage)

	return storage, err
}

func CreateS3Storage(updateBy string, storage *commonmodels.S3Storage, logger *xlog.Logger) (*commonmodels.S3Storage, error) {
	s3Storage := &s3.S3{S3Storage: storage}
	if err := s3.Validate(s3Storage); err != nil {
		logger.Warnf("failed to validate storage %s %v", storage.Endpoint, err)
		return nil, errors.ErrValidateS3Storage.AddErr(err)
	}

	storage.UpdatedBy = updateBy
	err := commonrepo.NewS3StorageColl().Create(storage)
	return storage, err
}

func ListS3Storage(logger *xlog.Logger) ([]*commonmodels.S3Storage, error) {
	stores, err := commonrepo.NewS3StorageColl().FindAll()
	if err == nil && len(stores) == 0 {
		stores = make([]*commonmodels.S3Storage, 0)
	}

	return stores, err
}

func DeleteS3Storage(deleteBy string, id string, logger *xlog.Logger) error {
	err := commonrepo.NewS3StorageColl().Delete(id)
	if err != nil {
		return err
	}

	logger.Infof("s3 storage %s is deleted by %s", id, deleteBy)
	return nil
}

func GetS3Storage(id string, logger *xlog.Logger) (*commonmodels.S3Storage, error) {
	store, err := commonrepo.NewS3StorageColl().Find(id)
	if err != nil {
		logger.Errorf("can't find store by id %s", id)
		return nil, err
	}

	return store, nil
}
