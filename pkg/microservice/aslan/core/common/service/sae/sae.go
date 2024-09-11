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

package sae

import (
	"fmt"

	sae20190506 "github.com/alibabacloud-go/sae-20190506/client"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/v2/pkg/tool/crypto"
)

func ListSAE(encryptedKey string, log *zap.SugaredLogger) ([]*commonmodels.SAE, error) {
	aesKey, err := service.GetAesKeyFromEncryptedKey(encryptedKey, log)
	if err != nil {
		log.Errorf("ListSAE GetAesKeyFromEncryptedKey err:%v", err)
		return nil, err
	}
	resp, err := commonrepo.NewSAEColl().List()
	if err != nil {
		return []*commonmodels.SAE{}, nil
	}
	for _, sae := range resp {
		sae.AccessKeySecret, err = crypto.AesEncryptByKey(sae.AccessKeySecret, aesKey.PlainText)
		if err != nil {
			log.Errorf("ListSAE AesEncryptByKey err:%v", err)
			return nil, err
		}
	}
	return resp, nil
}

func ListSAEInfo(log *zap.SugaredLogger) ([]*commonmodels.SAE, error) {
	resp, err := commonrepo.NewSAEColl().List()
	if err != nil {
		return nil, err
	}
	for _, sae := range resp {
		sae.AccessKeySecret = ""
	}
	return resp, nil
}

func CreateSAE(args *commonmodels.SAE, log *zap.SugaredLogger) error {
	if args == nil {
		return errors.New("nil sae")
	}

	if err := commonrepo.NewSAEColl().Create(args); err != nil {
		log.Errorf("CreateSAE err:%v", err)
		return err
	}
	return nil
}

func FindSAE(id, name string) (*commonmodels.SAE, error) {
	return commonrepo.NewSAEColl().Find(&commonrepo.SAECollFindOption{Id: id, Name: name})
}

func UpdateSAE(id string, args *commonmodels.SAE, log *zap.SugaredLogger) error {
	return commonrepo.NewSAEColl().Update(id, args)
}

func DeleteSAE(id string) error {
	return commonrepo.NewSAEColl().Delete(id)
}

func ValidateSAE(args *commonmodels.SAE) error {
	if args == nil {
		return errors.New("nil SAE")
	}
	return validateSAE(args)
}

func validateSAE(args *commonmodels.SAE) error {
	client, err := newClient(args, "cn-hangzhou")
	if err != nil {
		return fmt.Errorf("new SAE client err:%v", err)
	}

	describeNamespacesListRequest := &sae20190506.DescribeNamespaceListRequest{}
	_, err = client.DescribeNamespaceList(describeNamespacesListRequest)
	if err != nil {
		return fmt.Errorf("DescribeNamespaceList err: %v", err)
	}

	return nil
}
