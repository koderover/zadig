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
	"strings"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/pm"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/crypto"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/types"
)

func ListPrivateKeys(encryptedKey, projectName, keyword string, systemOnly bool, log *zap.SugaredLogger) ([]*commonmodels.PrivateKey, error) {
	var resp []*commonmodels.PrivateKey
	var err error
	privateKeys, err := commonrepo.NewPrivateKeyColl().List(&commonrepo.PrivateKeyArgs{ProjectName: projectName, SystemOnly: systemOnly})
	if err != nil {
		log.Errorf("PrivateKey.List error: %s", err)
		return resp, e.ErrListPrivateKeys
	}

	if keyword == "" {
		resp = privateKeys
	} else {
		for _, privateKey := range privateKeys {
			if strings.Contains(privateKey.Name, keyword) || strings.Contains(privateKey.IP, keyword) {
				resp = append(resp, privateKey)
			}
		}
	}

	aesKey, err := commonservice.GetAesKeyFromEncryptedKey(encryptedKey, log)
	if err != nil {
		return nil, err
	}
	for _, key := range resp {
		if key.Probe == nil {
			key.Probe = &types.Probe{ProbeScheme: setting.ProtocolTCP}
		}
		key.PrivateKey, err = crypto.AesEncryptByKey(key.PrivateKey, aesKey.PlainText)
		if err != nil {
			return nil, err
		}
	}
	return resp, nil
}

func ListPrivateKeysInternal(log *zap.SugaredLogger) ([]*commonmodels.PrivateKey, error) {
	resp, err := commonrepo.NewPrivateKeyColl().List(&commonrepo.PrivateKeyArgs{})
	if err != nil {
		if commonrepo.IsErrNoDocuments(err) {
			return []*commonmodels.PrivateKey{}, nil
		}
		log.Errorf("PrivateKey.List error: %v", err)
		return resp, e.ErrListPrivateKeys
	}
	return resp, nil
}

func GetPrivateKey(id string, log *zap.SugaredLogger) (*commonmodels.PrivateKey, error) {
	resp, err := commonrepo.NewPrivateKeyColl().Find(commonrepo.FindPrivateKeyOption{
		ID: id,
	})
	if err != nil {
		log.Errorf("PrivateKey.Find %s error: %s", id, err)
		return resp, e.ErrGetPrivateKey
	}
	return resp, nil
}

func CreatePrivateKey(args *commonmodels.PrivateKey, log *zap.SugaredLogger) error {
	if !config.CVMNameRegex.MatchString(args.Name) {
		return e.ErrCreatePrivateKey.AddDesc("主机名称仅支持字母，数字和下划线且首个字符不以数字开头")
	}

	privateKeyArgs := &commonrepo.PrivateKeyArgs{
		Name: args.Name,
	}

	if privateKeys, _ := commonrepo.NewPrivateKeyColl().List(privateKeyArgs); len(privateKeys) > 0 {
		return e.ErrCreatePrivateKey.AddDesc("Name already exists")
	}

	err := commonrepo.NewPrivateKeyColl().Create(args)
	if err != nil {
		log.Errorf("failed to create privateKey, error: %s", err)
		return e.ErrCreatePrivateKey
	}
	return nil
}

func UpdatePrivateKey(id string, args *commonmodels.PrivateKey, log *zap.SugaredLogger) error {
	_, err := commonrepo.NewPrivateKeyColl().Find(commonrepo.FindPrivateKeyOption{ID: id})
	if err != nil {
		log.Errorf("failed to find privateKey with id: %s, error: %s", id, err)
		return e.ErrUpdatePrivateKey.AddErr(fmt.Errorf("failed to find privateKey with id: %s, err: %s", id, err))
	}

	err = commonrepo.NewPrivateKeyColl().Update(id, args)
	if err != nil {
		log.Errorf("failed to update privateKey, error: %s", err)
		return e.ErrUpdatePrivateKey.AddErr(err)
	}
	return nil
}

func DeletePrivateKey(id, userName string, log *zap.SugaredLogger) error {
	// 检查该私钥是否被引用
	buildOpt := &commonrepo.BuildListOption{PrivateKeyID: id}
	builds, err := commonrepo.NewBuildColl().List(buildOpt)
	if err == nil && len(builds) != 0 {
		log.Errorf("PrivateKey has been used by build, private key id:%s, product name:%s, build name:%s", id, builds[0].ProductName, builds[0].Name)
		return e.ErrDeleteUsedPrivateKey
	}

	err = commonrepo.NewPrivateKeyColl().Delete(id)
	if err != nil {
		log.Errorf("PrivateKey.Delete %s error: %s", id, err)
		return e.ErrDeletePrivateKey
	}
	// update releated services , which contains the privateKey
	services, err := commonrepo.NewServiceColl().ListMaxRevisions(&commonrepo.ServiceListOption{Type: "pm"})
	if err != nil {
		return err
	}
	for _, service := range services {
		hostIDsSet := sets.NewString()
		for _, config := range service.EnvConfigs {
			hostIDsSet.Insert(config.HostIDs...)
		}
		if !hostIDsSet.Has(id) {
			continue
		}
		// has related hostID
		envConfigs := []*commonmodels.EnvConfig{}
		for _, config := range service.EnvConfigs {
			hostIdsSet := sets.NewString(config.HostIDs...)
			if hostIdsSet.Has(id) {
				hostIdsSet.Delete(id)
				config.HostIDs = hostIdsSet.List()
			}
			envConfigs = append(envConfigs, config)
		}

		envStatus, err := pm.GenerateEnvStatus(service.EnvConfigs, log)
		if err != nil {
			log.Errorf("GenerateEnvStatus err:%s", err)
			continue
		}
		args := &commonservice.ServiceTmplBuildObject{
			ServiceTmplObject: &commonservice.ServiceTmplObject{
				ProductName:  service.ProductName,
				ServiceName:  service.ServiceName,
				Visibility:   service.Visibility,
				Revision:     service.Revision,
				Type:         service.Type,
				Username:     userName,
				HealthChecks: service.HealthChecks,
				EnvConfigs:   envConfigs,
				EnvStatuses:  envStatus,
				From:         "deletePriveteKey",
			},
			Build: &commonmodels.Build{Name: service.BuildName},
		}
		if err := commonservice.UpdatePmServiceTemplate(userName, args, log); err != nil {
			log.Errorf("UpdatePmServiceTemplate err :%s", err)
			continue
		}
	}
	return nil
}

func ListLabels() ([]string, error) {
	return commonrepo.NewPrivateKeyColl().DistinctLabels()
}

// override: Full coverage (temporarily reserved)
// increment: Incremental coverage
// patch: Overwrite existing
func BatchCreatePrivateKey(args []*commonmodels.PrivateKey, option, username string, log *zap.SugaredLogger) error {
	switch option {
	case "increment":
		for _, currentPrivateKey := range args {
			if !config.CVMNameRegex.MatchString(currentPrivateKey.Name) {
				return e.ErrBulkCreatePrivateKey.AddDesc("主机名称仅支持字母，数字和下划线且首个字符不以数字开头")
			}

			if privateKeys, _ := commonrepo.NewPrivateKeyColl().List(&commonrepo.PrivateKeyArgs{Name: currentPrivateKey.Name}); len(privateKeys) > 0 {
				continue
			}

			currentPrivateKey.UpdateBy = username
			if err := commonrepo.NewPrivateKeyColl().Create(currentPrivateKey); err != nil {
				log.Errorf("PrivateKey.Create error: %s", err)
				return e.ErrBulkCreatePrivateKey.AddDesc("bulk add privateKey failed")
			}
		}

	case "patch":
		for _, currentPrivateKey := range args {
			if !config.CVMNameRegex.MatchString(currentPrivateKey.Name) {
				return e.ErrBulkCreatePrivateKey.AddDesc("主机名称仅支持字母，数字和下划线且首个字符不以数字开头")
			}
			currentPrivateKey.UpdateBy = username
			if privateKeys, _ := commonrepo.NewPrivateKeyColl().List(&commonrepo.PrivateKeyArgs{Name: currentPrivateKey.Name}); len(privateKeys) > 0 {
				if err := commonrepo.NewPrivateKeyColl().Update(privateKeys[0].ID.Hex(), currentPrivateKey); err != nil {
					log.Errorf("PrivateKey.update error: %s", err)
					return e.ErrBulkCreatePrivateKey.AddDesc("bulk update privateKey failed")
				}
				continue
			}
			if err := commonrepo.NewPrivateKeyColl().Create(currentPrivateKey); err != nil {
				log.Errorf("PrivateKey.Create error: %s", err)
				return e.ErrBulkCreatePrivateKey.AddDesc("bulk add privateKey failed")
			}
		}
	}

	return nil
}
