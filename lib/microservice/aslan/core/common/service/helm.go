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
	"context"
	"fmt"
	"os"
	"path"

	commonmodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	commonrepo "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	s3service "github.com/koderover/zadig/lib/microservice/aslan/core/common/service/s3"
	"github.com/koderover/zadig/lib/microservice/aslan/internal/log"
	"github.com/koderover/zadig/lib/setting"
	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/tool/xlog"
	"github.com/koderover/zadig/lib/util"
)

func ListHelmRepos(log *xlog.Logger) ([]*commonmodels.HelmRepo, error) {
	helmRepos, err := commonrepo.NewHelmRepoColl().List()
	if err != nil {
		log.Errorf("ListHelmRepos err:%v", err)
		return []*commonmodels.HelmRepo{}, nil
	}

	return helmRepos, nil
}

func DownloadService(base, serviceName string) error {
	s3Storage, err := s3service.FindDefaultS3()
	if err != nil {
		log.Errorf("获取默认的s3配置失败 err:%v", err)
		return e.ErrListTemplate.AddDesc(err.Error())
	}
	subFolderName := serviceName + "-" + setting.HelmDeployType
	if s3Storage.Subfolder != "" {
		s3Storage.Subfolder = fmt.Sprintf("%s/%s/%s", s3Storage.Subfolder, subFolderName, "service")
	} else {
		s3Storage.Subfolder = fmt.Sprintf("%s/%s", subFolderName, "service")
	}
	filePath := fmt.Sprintf("%s.tar.gz", serviceName)
	tarFilePath := path.Join(base, filePath)
	if err = s3service.Download(context.Background(), s3Storage, filePath, tarFilePath); err != nil {
		log.Errorf("s3下载文件失败 err:%v", err)
		return err
	}
	if err = util.UnTar("/", tarFilePath); err != nil {
		log.Errorf("unTar err:%v", err)
		return err
	}
	if err = os.Remove(tarFilePath); err != nil {
		log.Errorf("remove file err:%v", err)
	}
	return nil
}
