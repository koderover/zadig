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

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func ListPrivateKeys(log *zap.SugaredLogger) ([]*commonmodels.PrivateKey, error) {
	resp, err := commonrepo.NewPrivateKeyColl().List(&commonrepo.PrivateKeyArgs{})
	if err != nil {
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
		log.Errorf("PrivateKey.Find %s error: %v", id, err)
		return resp, e.ErrGetPrivateKey
	}
	return resp, nil
}

func CreatePrivateKey(args *commonmodels.PrivateKey, log *zap.SugaredLogger) error {
	if !config.CVMNameRegex.MatchString(args.Name) {
		return e.ErrCreatePrivateKey.AddDesc("主机名称仅支持字母，数字和下划线且首个字符不以数字开头")
	}

	if privateKeys, _ := commonrepo.NewPrivateKeyColl().List(&commonrepo.PrivateKeyArgs{Name: args.Name}); len(privateKeys) > 0 {
		return e.ErrCreatePrivateKey.AddDesc("Name already exists")
	}

	err := commonrepo.NewPrivateKeyColl().Create(args)
	if err != nil {
		log.Errorf("PrivateKey.Create error: %v", err)
		return e.ErrCreatePrivateKey
	}
	return nil
}

func UpdatePrivateKey(id string, args *commonmodels.PrivateKey, log *zap.SugaredLogger) error {
	err := commonrepo.NewPrivateKeyColl().Update(id, args)
	if err != nil {
		log.Errorf("PrivateKey.Update %s error: %v", id, err)
		return e.ErrUpdatePrivateKey
	}
	return nil
}

func DeletePrivateKey(id string, log *zap.SugaredLogger) error {
	// 检查该私钥是否被引用
	buildOpt := &commonrepo.BuildListOption{PrivateKeyID: id}
	builds, err := commonrepo.NewBuildColl().List(buildOpt)
	if err == nil && len(builds) != 0 {
		log.Errorf("PrivateKey has been used by build, private key id:%s, product name:%s, build name:%s", id, builds[0].ProductName, builds[0].Name)
		return e.ErrDeleteUsedPrivateKey
	}

	err = commonrepo.NewPrivateKeyColl().Delete(id)
	if err != nil {
		log.Errorf("PrivateKey.Delete %s error: %v", id, err)
		return e.ErrDeletePrivateKey
	}
	return nil
}
