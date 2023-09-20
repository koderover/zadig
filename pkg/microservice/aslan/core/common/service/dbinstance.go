/*
Copyright 2023 The KodeRover Authors.

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
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/tool/crypto"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

func ListDBInstances(encryptedKey string, log *zap.SugaredLogger) ([]*commonmodels.DBInstance, error) {
	aesKey, err := GetAesKeyFromEncryptedKey(encryptedKey, log)
	if err != nil {
		log.Errorf("ListDBInstances GetAesKeyFromEncryptedKey err:%v", err)
		return nil, err
	}
	helmRepos, err := commonrepo.NewDBInstanceColl().List()
	if err != nil {
		return []*commonmodels.DBInstance{}, nil
	}
	for _, helmRepo := range helmRepos {
		helmRepo.Password, err = crypto.AesEncryptByKey(helmRepo.Password, aesKey.PlainText)
		if err != nil {
			log.Errorf("ListDBInstances AesEncryptByKey err:%v", err)
			return nil, err
		}
	}
	return helmRepos, nil
}

func CreateDBInstance(args *commonmodels.DBInstance, log *zap.SugaredLogger) error {
	if args == nil {
		return errors.New("nil DBInstance")
	}

	if err := commonrepo.NewDBInstanceColl().Create(args); err != nil {
		log.Errorf("CreateDBInstance err:%v", err)
		return err
	}
	return nil
}

func FindDBInstance(id, name string) (*commonmodels.DBInstance, error) {
	return commonrepo.NewDBInstanceColl().Find(&commonrepo.DBInstanceCollFindOption{Id: id, Name: name})
}

func UpdateDBInstance(args *commonmodels.DBInstance, log *zap.SugaredLogger) error {
	return commonrepo.NewDBInstanceColl().Update(args.ID, args)
}

func DeleteDBInstance(id string) error {
	return commonrepo.NewDBInstanceColl().Delete(id)
}
