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
	"strconv"
	"strings"
	"sync"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/s3"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/crypto"
	"github.com/koderover/zadig/pkg/tool/errors"
	s3tool "github.com/koderover/zadig/pkg/tool/s3"
)

func UpdateS3Storage(updateBy, id string, storage *commonmodels.S3Storage, logger *zap.SugaredLogger) error {
	s3Storage := &s3.S3{S3Storage: storage}
	forcedPathStyle := true
	if s3Storage.Provider == setting.ProviderSourceAli {
		forcedPathStyle = false
	}
	client, err := s3tool.NewClient(s3Storage.Endpoint, s3Storage.Ak, s3Storage.Sk, s3Storage.Region, s3Storage.Insecure, forcedPathStyle)
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
	forcedPathStyle := true
	if s3Storage.Provider == setting.ProviderSourceAli {
		forcedPathStyle = false
	}
	client, err := s3tool.NewClient(s3Storage.Endpoint, s3Storage.Ak, s3Storage.Sk, s3Storage.Region, s3Storage.Insecure, forcedPathStyle)
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

func ListS3Storage(encryptedKey string, logger *zap.SugaredLogger) ([]*commonmodels.S3Storage, error) {
	stores, err := commonrepo.NewS3StorageColl().FindAll()
	if err == nil && len(stores) == 0 {
		stores = make([]*commonmodels.S3Storage, 0)
	}
	aesKey, err := service.GetAesKeyFromEncryptedKey(encryptedKey, logger)
	if err != nil {
		logger.Errorf("ListS3Storage GetAesKeyFromEncryptedKey err:%s", err)
		return nil, err
	}
	for _, store := range stores {
		store.Sk, err = crypto.AesEncryptByKey(store.Sk, aesKey.PlainText)
		if err != nil {
			logger.Errorf("ListS3Storage AesEncryptByKey err:%s", err)
			return nil, err
		}
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

func ListTars(id, kind string, serviceNames []string, logger *zap.SugaredLogger) ([]*commonmodels.TarInfo, error) {
	var (
		wg         wait.Group
		mutex      sync.RWMutex
		tarInfos   = make([]*commonmodels.TarInfo, 0)
		store      *commonmodels.S3Storage
		defaultS3  s3.S3
		defaultURL string
		err        error
	)

	store, err = commonrepo.NewS3StorageColl().Find(id)
	if err != nil {
		logger.Errorf("can't find store by id:%s err:%s", id, err)
		return nil, err
	}
	defaultS3 = s3.S3{
		S3Storage: store,
	}
	defaultURL, err = defaultS3.GetEncryptedURL()
	if err != nil {
		logger.Errorf("defaultS3 GetEncryptedURL err:%s", err)
		return nil, err
	}

	for _, serviceName := range serviceNames {
		// Change the service name to underscore splicing
		newServiceName := serviceName
		wg.Start(func() {
			deliveryArtifactArgs := &commonrepo.DeliveryArtifactArgs{
				Name:              newServiceName,
				Type:              kind,
				Source:            string(config.WorkflowType),
				PackageStorageURI: store.Endpoint + "/" + store.Bucket,
			}
			deliveryArtifacts, err := commonrepo.NewDeliveryArtifactColl().ListTars(deliveryArtifactArgs)
			if err != nil {
				logger.Errorf("ListTars err:%s", err)
				return
			}
			deliveryArtifactArgs.Name = newServiceName + "_" + newServiceName
			newDeliveryArtifacts, err := commonrepo.NewDeliveryArtifactColl().ListTars(deliveryArtifactArgs)
			if err != nil {
				logger.Errorf("ListTars err:%s", err)
				return
			}
			deliveryArtifacts = append(deliveryArtifacts, newDeliveryArtifacts...)
			for _, deliveryArtifact := range deliveryArtifacts {
				activities, _, err := commonrepo.NewDeliveryActivityColl().List(&commonrepo.DeliveryActivityArgs{ArtifactID: deliveryArtifact.ID.Hex()})
				if err != nil {
					logger.Errorf("deliveryActivity.list err:%s", err)
					return
				}
				urlArr := strings.Split(activities[0].URL, "/")
				workflowName := urlArr[len(urlArr)-2]
				taskIDStr := urlArr[len(urlArr)-1]
				taskID, err := strconv.Atoi(taskIDStr)
				if err != nil {
					logger.Errorf("string convert to int err:%s", err)
					return
				}

				mutex.Lock()
				tarInfos = append(tarInfos, &commonmodels.TarInfo{
					URL:          defaultURL,
					Name:         newServiceName,
					FileName:     deliveryArtifact.Image,
					WorkflowName: workflowName,
					TaskID:       int64(taskID),
				})
				mutex.Unlock()
			}
		})
	}
	wg.Wait()
	return tarInfos, nil
}
