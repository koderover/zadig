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
	"io/fs"
	"path"

	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	fsservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/fs"
	"github.com/koderover/zadig/v2/pkg/tool/crypto"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

func ListHelmRepos(encryptedKey string, log *zap.SugaredLogger) ([]*commonmodels.HelmRepo, error) {
	aesKey, err := GetAesKeyFromEncryptedKey(encryptedKey, log)
	if err != nil {
		log.Errorf("ListHelmRepos GetAesKeyFromEncryptedKey err:%v", err)
		return nil, err
	}
	helmRepos, err := commonrepo.NewHelmRepoColl().List()
	if err != nil {
		log.Errorf("ListHelmRepos err:%v", err)
		return []*commonmodels.HelmRepo{}, nil
	}
	for _, helmRepo := range helmRepos {
		helmRepo.Password, err = crypto.AesEncryptByKey(helmRepo.Password, aesKey.PlainText)
		if err != nil {
			log.Errorf("ListHelmRepos AesEncryptByKey err:%v", err)
			return nil, err
		}
	}
	return helmRepos, nil
}

func ListHelmReposByProject(projectName string, log *zap.SugaredLogger) ([]*commonmodels.HelmRepo, error) {
	helmRepos, err := commonrepo.NewHelmRepoColl().ListByProject(projectName)
	if err != nil {
		log.Errorf("ListHelmRepos err:%v", err)
		return []*commonmodels.HelmRepo{}, nil
	}
	for _, helmRepo := range helmRepos {
		helmRepo.Password = ""
		helmRepo.Projects = nil
	}
	return helmRepos, nil
}

func ListHelmReposPublic() ([]*commonmodels.HelmRepo, error) {
	return commonrepo.NewHelmRepoColl().List()
}

func SaveAndUploadService(projectName, serviceName string, copies []string, fileTree fs.FS, isProduction bool) error {
	var localBase, s3Base string
	if !isProduction {
		localBase = config.LocalTestServicePath(projectName, serviceName)
		s3Base = config.ObjectStorageTestServicePath(projectName, serviceName)
	} else {
		localBase = config.LocalProductionServicePath(projectName, serviceName)
		s3Base = config.ObjectStorageProductionServicePath(projectName, serviceName)
	}
	names := append([]string{serviceName}, copies...)
	return fsservice.SaveAndUploadFiles(fileTree, names, localBase, s3Base, log.SugaredLogger())
}

func CopyAndUploadService(projectName, serviceName, currentChartPath string, copies []string, isProduction bool) error {
	var localBase, s3Base string
	if !isProduction {
		localBase = config.LocalTestServicePath(projectName, serviceName)
		s3Base = config.ObjectStorageTestServicePath(projectName, serviceName)
	} else {
		localBase = config.LocalProductionServicePath(projectName, serviceName)
		s3Base = config.ObjectStorageProductionServicePath(projectName, serviceName)
	}
	names := append([]string{serviceName}, copies...)

	return fsservice.CopyAndUploadFiles(names, path.Join(localBase, serviceName), s3Base, localBase, currentChartPath, log.SugaredLogger())
}
