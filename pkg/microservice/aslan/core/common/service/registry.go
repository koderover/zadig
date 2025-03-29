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

	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/kube"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/tool/crypto"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/util"
)

func FindRegistryById(registryId string, getRealCredential bool, log *zap.SugaredLogger) (reg *models.RegistryNamespace, err error) {
	return findRegisty(&mongodb.FindRegOps{ID: registryId}, getRealCredential, log)
}

func findRegisty(regOps *mongodb.FindRegOps, getRealCredential bool, log *zap.SugaredLogger) (reg *models.RegistryNamespace, err error) {
	// TODO: 多租户适配
	resp, err := mongodb.NewRegistryNamespaceColl().Find(regOps)

	if err != nil {
		return nil, err
	}

	if !getRealCredential {
		return resp, nil
	}
	resp, err = commonutil.DecodeRegistry(resp)
	if err != nil {
		log.Errorf("DecodeRegistry error: %s", err)
		return nil, err
	}

	return resp, nil
}

func FindDefaultRegistry(getRealCredential bool, log *zap.SugaredLogger) (reg *models.RegistryNamespace, err error) {
	return findRegisty(&mongodb.FindRegOps{IsDefault: true}, getRealCredential, log)
}

func ListRegistryByProject(projectName string, log *zap.SugaredLogger) ([]*models.RegistryNamespace, error) {
	resp, err := mongodb.NewRegistryNamespaceColl().FindByProject(projectName)
	if err != nil {
		log.Errorf("RegistryNamespace.List error: %s", err)
		return resp, fmt.Errorf("RegistryNamespace.List error: %s", err)
	}

	return resp, nil
}

func ListRegistryNamespaces(encryptedKey string, getRealCredential bool, log *zap.SugaredLogger) ([]*models.RegistryNamespace, error) {
	resp, err := mongodb.NewRegistryNamespaceColl().FindAll(&mongodb.FindRegOps{})
	if err != nil {
		log.Errorf("RegistryNamespace.List error: %s", err)
		return resp, fmt.Errorf("RegistryNamespace.List error: %s", err)
	}
	var aesKey *commonutil.GetAesKeyFromEncryptedKeyResp
	if len(encryptedKey) > 0 {
		aesKey, err = commonutil.GetAesKeyFromEncryptedKey(encryptedKey, log)
		if err != nil {
			err = fmt.Errorf("RegistryNamespace.List GetAesKeyFromEncryptedKey error: %w", err)
			log.Error(err)
			return nil, err
		}
	}
	if !getRealCredential {
		if len(encryptedKey) > 0 {
			for _, reg := range resp {
				reg.SecretKey, err = crypto.AesEncryptByKey(reg.SecretKey, aesKey.PlainText)
				if err != nil {
					err = fmt.Errorf("RegistryNamespace.List AesEncryptByKey error: %w", err)
					log.Error(err)
					return nil, err
				}
			}
		}
		return resp, nil
	}

	for _, reg := range resp {
		switch reg.RegProvider {
		case config.RegistryTypeSWR:
			reg.SecretKey = util.ComputeHmacSha256(reg.AccessKey, reg.SecretKey)
			reg.AccessKey = fmt.Sprintf("%s@%s", reg.Region, reg.AccessKey)
		case config.RegistryTypeAWS:
			realAK, realSK, err := commonutil.GetAWSRegistryCredential(reg.ID.Hex(), reg.AccessKey, reg.SecretKey, reg.Region)
			if err != nil {
				err = fmt.Errorf("Failed to get keypair from aws registry credential, error: %w", err)
				log.Error(err)
				return nil, err
			}
			reg.AccessKey = realAK
			reg.SecretKey = realSK
		}
		if len(encryptedKey) == 0 {
			continue
		}
		reg.SecretKey, err = crypto.AesEncryptByKey(reg.SecretKey, aesKey.PlainText)
		if err != nil {
			err = fmt.Errorf("RegistryNamespace.List AesEncryptByKey error: %w", err)
			log.Error(err)
			return nil, err
		}
	}
	return resp, nil
}

func EnsureDefaultRegistrySecret(namespace string, registryId string, kubeClient client.Client, log *zap.SugaredLogger) error {
	var reg *models.RegistryNamespace
	var err error
	if len(registryId) > 0 {
		reg, err = FindRegistryById(registryId, true, log)
		if err != nil {
			log.Errorf(
				"service.EnsureRegistrySecret: failed to find registry: %s error msg:%s",
				registryId, err,
			)
			return err
		}
	} else {
		reg, err = FindDefaultRegistry(true, log)
		if err != nil {
			log.Errorf(
				"service.EnsureRegistrySecret: failed to find default candidate registry: %s %s",
				namespace, err,
			)
			return err
		}
	}

	err = kube.CreateOrUpdateDefaultRegistrySecret(namespace, reg, kubeClient)
	if err != nil {
		log.Errorf("[%s] CreateDockerSecret error: %s", namespace, err)
		return e.ErrUpdateSecret.AddDesc(e.CreateDefaultRegistryErrMsg)
	}

	return nil
}
