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

	"github.com/27149chen/afero"
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	fsservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/fs"
	"github.com/koderover/zadig/pkg/tool/log"
	fsutil "github.com/koderover/zadig/pkg/util/fs"
)

func ListHelmRepos(log *zap.SugaredLogger) ([]*commonmodels.HelmRepo, error) {
	helmRepos, err := commonrepo.NewHelmRepoColl().List()
	if err != nil {
		log.Errorf("ListHelmRepos err:%v", err)
		return []*commonmodels.HelmRepo{}, nil
	}

	return helmRepos, nil
}

func PreLoadServiceManifests(base string, svc *commonmodels.Service) error {
	ok, err := fsutil.DirExists(base)
	if err != nil {
		log.Errorf("Failed to check if dir %s is exiting, err: %s", base, err)
		return err
	}
	if ok {
		return nil
	}

	if err = DownloadServiceManifests(base, svc.ProductName, svc.ServiceName); err == nil {
		return nil
	}

	log.Warnf("Failed to download service from s3, err: %s", err)

	return preLoadServiceManifestsFromSource(svc)
}

func DownloadServiceManifests(base, projectName, serviceName string) error {
	s3Base := config.ObjectStorageServicePath(projectName, serviceName)
	return fsservice.DownloadAndExtractFilesFromS3(serviceName, base, s3Base, log.SugaredLogger())
}

func SaveAndUploadService(projectName, serviceName string, fileTree fs.FS) error {
	localBase := config.LocalServicePath(projectName, serviceName)
	s3Base := config.ObjectStorageServicePath(projectName, serviceName)

	return fsservice.SaveAndUploadFiles(fileTree, serviceName, localBase, s3Base, log.SugaredLogger())
}

func preLoadServiceManifestsFromSource(svc *commonmodels.Service) error {
	tree, err := fsservice.DownloadFilesFromSource(
		&fsservice.DownloadFromSourceArgs{CodehostID: svc.CodehostID, Owner: svc.RepoOwner, Repo: svc.RepoName, Path: svc.LoadPath, Branch: svc.BranchName, RepoLink: svc.SrcPath},
		func(afero.Fs) (string, error) {
			return svc.ServiceName, nil
		})
	if err != nil {
		return err
	}

	// save files to disk and upload them to s3
	if err = SaveAndUploadService(svc.ProductName, svc.ServiceName, tree); err != nil {
		log.Errorf("Failed to save or upload files for service %s in project %s, error: %s", svc.ServiceName, svc.ProductName, err)
		return err
	}

	return nil
}
