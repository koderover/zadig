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
	"os"
	"path"

	"github.com/27149chen/afero"
	"go.uber.org/zap"
	"helm.sh/helm/v3/pkg/repo"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/command"
	fsservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/fs"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/client/systemconfig"
	"github.com/koderover/zadig/pkg/tool/crypto"
	"github.com/koderover/zadig/pkg/tool/log"
	fsutil "github.com/koderover/zadig/pkg/util/fs"
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

func ListHelmReposPublic() ([]*commonmodels.HelmRepo, error) {
	return commonrepo.NewHelmRepoColl().List()
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
	switch svc.Source {
	case setting.SourceFromGerrit:
		return preLoadServiceManifestsFromGerrit(svc)
	case setting.SourceFromGitee, setting.SourceFromGiteeEE:
		return preLoadServiceManifestsFromGitee(svc)
	default:
		return preLoadServiceManifestsFromSource(svc)
	}
}

func PreloadServiceManifestsByRevision(base string, svc *commonmodels.Service) error {
	ok, err := fsutil.DirExists(base)
	if err != nil {
		log.Errorf("Failed to check if dir %s is exiting, err: %s", base, err)
		return err
	}
	if ok {
		return nil
	}

	//download chart info by revision
	serviceNameWithRevision := config.ServiceNameWithRevision(svc.ServiceName, svc.Revision)
	s3Base := config.ObjectStorageServicePath(svc.ProductName, svc.ServiceName)
	return fsservice.DownloadAndExtractFilesFromS3(serviceNameWithRevision, base, s3Base, log.SugaredLogger())
}

func DownloadServiceManifests(base, projectName, serviceName string) error {
	s3Base := config.ObjectStorageServicePath(projectName, serviceName)

	return fsservice.DownloadAndExtractFilesFromS3(serviceName, base, s3Base, log.SugaredLogger())
}

func SaveAndUploadService(projectName, serviceName string, copies []string, fileTree fs.FS) error {
	localBase := config.LocalServicePath(projectName, serviceName)
	s3Base := config.ObjectStorageServicePath(projectName, serviceName)
	names := append([]string{serviceName}, copies...)
	return fsservice.SaveAndUploadFiles(fileTree, names, localBase, s3Base, log.SugaredLogger())
}

func CopyAndUploadService(projectName, serviceName, currentChartPath string, copies []string) error {
	localBase := config.LocalServicePath(projectName, serviceName)
	s3Base := config.ObjectStorageServicePath(projectName, serviceName)
	names := append([]string{serviceName}, copies...)

	return fsservice.CopyAndUploadFiles(names, path.Join(localBase, serviceName), s3Base, localBase, currentChartPath, log.SugaredLogger())
}

func preLoadServiceManifestsFromSource(svc *commonmodels.Service) error {
	tree, err := fsservice.DownloadFilesFromSource(
		&fsservice.DownloadFromSourceArgs{CodehostID: svc.CodehostID, Owner: svc.RepoOwner, Namespace: svc.RepoNamespace, Repo: svc.RepoName, Path: svc.LoadPath, Branch: svc.BranchName, RepoLink: svc.SrcPath},
		func(afero.Fs) (string, error) {
			return svc.ServiceName, nil
		})
	if err != nil {
		return err
	}

	// save files to disk and upload them to s3
	if err = SaveAndUploadService(svc.ProductName, svc.ServiceName, nil, tree); err != nil {
		log.Errorf("Failed to save or upload files for service %s in project %s, error: %s", svc.ServiceName, svc.ProductName, err)
		return err
	}

	return nil
}

func preLoadServiceManifestsFromGerrit(svc *commonmodels.Service) error {
	base := path.Join(config.S3StoragePath(), svc.GerritRepoName)
	if err := os.RemoveAll(base); err != nil {
		log.Warnf("Failed to remove dir, err:%s", err)
	}
	detail, err := systemconfig.New().GetCodeHost(svc.GerritCodeHostID)
	if err != nil {
		log.Errorf("Failed to GetCodehostDetail, err:%s", err)
		return err
	}
	err = command.RunGitCmds(detail, "default", "default", svc.GerritRepoName, svc.GerritBranchName, svc.GerritRemoteName)
	if err != nil {
		log.Errorf("Failed to runGitCmds, err:%s", err)
		return err
	}
	// copy files to disk and upload them to s3
	if err := CopyAndUploadService(svc.ProductName, svc.ServiceName, svc.GerritPath, nil); err != nil {
		log.Errorf("Failed to save or upload files for service %s in project %s, error: %s", svc.ServiceName, svc.ProductName, err)
		return err
	}
	return nil
}

func GeneHelmRepo(chartRepo *commonmodels.HelmRepo) *repo.Entry {
	return &repo.Entry{
		Name:     chartRepo.RepoName,
		URL:      chartRepo.URL,
		Username: chartRepo.Username,
		Password: chartRepo.Password,
	}
}

func preLoadServiceManifestsFromGitee(svc *commonmodels.Service) error {
	base := path.Join(config.S3StoragePath(), svc.RepoName)
	if err := os.RemoveAll(base); err != nil {
		log.Warnf("Failed to remove dir, err:%s", err)
	}
	detail, err := systemconfig.New().GetCodeHost(svc.CodehostID)
	if err != nil {
		log.Errorf("Failed to GetCodehostDetail, err:%s", err)
		return err
	}
	err = command.RunGitCmds(detail, svc.RepoOwner, svc.GetRepoNamespace(), svc.RepoName, svc.BranchName, "origin")
	if err != nil {
		log.Errorf("Failed to runGitCmds, err:%s", err)
		return err
	}
	// save files to disk and upload them to s3
	if err := CopyAndUploadService(svc.ProductName, svc.ServiceName, svc.GiteePath, nil); err != nil {
		log.Errorf("Failed to copy or upload files for service %s in project %s, error: %s", svc.ServiceName, svc.ProductName, err)
		return err
	}
	return nil
}
