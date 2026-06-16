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
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/crypto"
)

func ListDBInstances(encryptedKey string, log *zap.SugaredLogger) ([]*commonmodels.DBInstance, error) {
	resp, err := commonrepo.NewDBInstanceColl().List()
	if err != nil {
		return []*commonmodels.DBInstance{}, nil
	}
	if err := protectDBInstancePasswords(resp, encryptedKey, log); err != nil {
		return nil, err
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

func GetEncryptedDBInstance(id, encryptedKey string, log *zap.SugaredLogger) (*commonmodels.DBInstance, error) {
	resp, err := FindDBInstance(id, "")
	if err != nil {
		return nil, err
	}
	if err := protectDBInstancePasswords([]*commonmodels.DBInstance{resp}, encryptedKey, log); err != nil {
		return nil, err
	}
	return resp, nil
}

func protectDBInstancePasswords(instances []*commonmodels.DBInstance, encryptedKey string, log *zap.SugaredLogger) error {
	targets := make([]*commonmodels.DBInstance, 0)
	for _, instance := range instances {
		if instance == nil || instance.Password == "" {
			continue
		}
		targets = append(targets, instance)
	}

	if len(targets) == 0 {
		return nil
	}

	if encryptedKey == "" {
		for _, instance := range targets {
			instance.Password = setting.MaskValue
		}
		return nil
	}

	aesKey, err := commonutil.GetAesKeyFromEncryptedKey(encryptedKey, log)
	if err != nil {
		log.Errorf("DBInstance.GetAesKeyFromEncryptedKey err:%v", err)
		return err
	}

	encryptedPasswords := make([]string, len(targets))
	for i, instance := range targets {
		encryptedPasswords[i], err = crypto.AesEncryptByKey(instance.Password, aesKey.PlainText)
		if err != nil {
			log.Errorf("DBInstance.AesEncryptByKey err:%v", err)
			return err
		}
	}

	for i, instance := range targets {
		instance.Password = encryptedPasswords[i]
	}

	return nil
}

func restoreMaskedDBInstancePassword(id string, args *commonmodels.DBInstance) error {
	if args == nil || args.Password != setting.MaskValue {
		return nil
	}

	lookupID := id
	if lookupID == "" && !args.ID.IsZero() {
		lookupID = args.ID.Hex()
	}

	oldDB, err := FindDBInstance(lookupID, args.Name)
	if err != nil {
		return err
	}
	args.Password = oldDB.Password
	return nil
}

func UpdateDBInstance(id string, args *commonmodels.DBInstance, log *zap.SugaredLogger) error {
	if err := restoreMaskedDBInstancePassword(id, args); err != nil {
		log.Errorf("UpdateDBInstance restore masked password error:%v", err)
		return err
	}
	return commonrepo.NewDBInstanceColl().Update(id, args)
}

func DeleteDBInstance(id string) error {
	return commonrepo.NewDBInstanceColl().Delete(id)
}

func ValidateDBInstance(args *commonmodels.DBInstance) error {
	if args == nil {
		return errors.New("nil DBInstance")
	}
	if err := restoreMaskedDBInstancePassword("", args); err != nil {
		return err
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
