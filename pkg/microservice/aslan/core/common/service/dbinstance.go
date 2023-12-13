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
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/tool/crypto"
)

func ListDBInstances(encryptedKey string, log *zap.SugaredLogger) ([]*commonmodels.DBInstance, error) {
	aesKey, err := GetAesKeyFromEncryptedKey(encryptedKey, log)
	if err != nil {
		log.Errorf("ListDBInstances GetAesKeyFromEncryptedKey err:%v", err)
		return nil, err
	}
	resp, err := commonrepo.NewDBInstanceColl().List()
	if err != nil {
		return []*commonmodels.DBInstance{}, nil
	}
	for _, db := range resp {
		db.Password, err = crypto.AesEncryptByKey(db.Password, aesKey.PlainText)
		if err != nil {
			log.Errorf("ListDBInstances AesEncryptByKey err:%v", err)
			return nil, err
		}
	}
	return resp, nil
}

func ListDBInstancesInfo(log *zap.SugaredLogger) ([]*commonmodels.DBInstance, error) {
	resp, err := commonrepo.NewDBInstanceColl().List()
	if err != nil {
		return nil, err
	}
	for _, db := range resp {
		db.Password = ""
	}
	return resp, nil
}

func ListDBInstancesInfoByProject(projectName string, log *zap.SugaredLogger) ([]*commonmodels.DBInstance, error) {
	resp, err := commonrepo.NewDBInstanceColl().ListByProject(projectName)
	if err != nil {
		return nil, err
	}
	for _, db := range resp {
		db.Password = ""
	}
	return resp, nil
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

func UpdateDBInstance(id string, args *commonmodels.DBInstance, log *zap.SugaredLogger) error {
	return commonrepo.NewDBInstanceColl().Update(id, args)
}

func DeleteDBInstance(id string) error {
	return commonrepo.NewDBInstanceColl().Delete(id)
}

func ValidateDBInstance(args *commonmodels.DBInstance) error {
	if args == nil {
		return errors.New("nil DBInstance")
	}
	switch args.Type {
	case config.DBInstanceTypeMySQL, config.DBInstanceTypeMariaDB:
		return validateMySQLInstance(args)
	default:
		return errors.Errorf("invalid db type %s", args.Type)
	}
}

func validateMySQLInstance(args *commonmodels.DBInstance) error {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/?charset=utf8&parseTime=True&loc=Local", args.Username, args.Password, args.Host, args.Port)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return errors.Errorf("connect mysql failed, err: %s", err)
	}
	defer db.Close()

	if err = db.Ping(); err != nil {
		return errors.Errorf("ping mysql failed, err: %s", err)
	}
	return nil
}
