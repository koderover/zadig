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
	"os"
	"path/filepath"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	s3service "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/s3"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
	s3tool "github.com/koderover/zadig/pkg/tool/s3"
	"github.com/koderover/zadig/pkg/util/fs"
)

func ListHelmRepos(log *zap.SugaredLogger) ([]*commonmodels.HelmRepo, error) {
	helmRepos, err := commonrepo.NewHelmRepoColl().List()
	if err != nil {
		log.Errorf("ListHelmRepos err:%v", err)
		return []*commonmodels.HelmRepo{}, nil
	}

	return helmRepos, nil
}

func PreLoadServiceManifests(base, productName, serviceName string) error {
	ok, err := fs.DirExists(base)
	if err != nil {
		log.Errorf("Failed to check if dir %s is exiting, err: %s", base, err)
		return err
	}
	if !ok {
		if err = DownloadServiceManifests(base, productName, serviceName); err != nil {
			log.Errorf("Failed to download service from s3, err: %s", err)
			return err
		}
	}

	return nil
}

func DownloadServiceManifests(base, projectName, serviceName string) error {
	s3Storage, err := s3service.FindDefaultS3()
	if err != nil {
		log.Errorf("Failed to find default s3, err: %s", err)
		return e.ErrListTemplate.AddDesc(err.Error())
	}

	tarball := fmt.Sprintf("%s.tar.gz", serviceName)
	tarFilePath := filepath.Join(base, tarball)
	s3Storage.Subfolder = filepath.Join(s3Storage.Subfolder, config.ObjectStorageServicePath(projectName, serviceName))

	forcedPathStyle := true
	if s3Storage.Provider == setting.ProviderSourceAli {
		forcedPathStyle = false
	}
	client, err := s3tool.NewClient(s3Storage.Endpoint, s3Storage.Ak, s3Storage.Sk, s3Storage.Insecure, forcedPathStyle)
	if err != nil {
		log.Errorf("Failed to create s3 client, err: %s", err)
		return err
	}
	if err = client.Download(s3Storage.Bucket, s3Storage.GetObjectPath(tarball), tarFilePath); err != nil {
		log.Errorf("Failed to download file from s3, err: %s", err)
		return err
	}
	if err = fs.Untar(tarFilePath, base); err != nil {
		log.Errorf("Untar err: %s", err)
		return err
	}
	if err = os.Remove(tarFilePath); err != nil {
		log.Errorf("Failed to remove file %s, err: %s", tarFilePath, err)
	}

	return nil
}
