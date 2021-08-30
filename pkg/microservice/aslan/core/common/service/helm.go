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
	"io/fs"
	"os"
	"path/filepath"

	"github.com/27149chen/afero"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	githubservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/github"
	gitlabservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/gitlab"
	s3service "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/s3"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/codehost"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
	s3tool "github.com/koderover/zadig/pkg/tool/s3"
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
	if err = fsutil.Untar(tarFilePath, base); err != nil {
		log.Errorf("Untar err: %s", err)
		return err
	}
	if err = os.Remove(tarFilePath); err != nil {
		log.Errorf("Failed to remove file %s, err: %s", tarFilePath, err)
	}

	return nil
}

type DownloadFromSourceParams struct {
	CodehostID                int
	Owner, Repo, Path, Branch string
}

func DownloadServiceManifestsFromSource(svc *DownloadFromSourceParams, serviceNameGetter func(afero.Fs) (string, error)) (fs.FS, error) {
	getter, err := getTreeGetter(svc.CodehostID)
	if err != nil {
		log.Errorf("Failed to get tree getter, err: %s", err)
		return nil, err
	}

	chartTree, err := getter.GetTreeContents(svc.Owner, svc.Repo, svc.Path, svc.Branch)
	if err != nil {
		log.Errorf("Failed to get tree contents for service %+v, err: %s", svc, err)
		return nil, err
	}

	serviceName, err := serviceNameGetter(chartTree)
	if err != nil {
		log.Errorf("Failed to get service name, err: %s", err)
		return nil, err
	}

	// rename the root path of the chart to the service name
	f, _ := fs.ReadDir(afero.NewIOFS(chartTree), "")
	if len(f) == 1 {
		if err = chartTree.Rename(f[0].Name(), serviceName); err != nil {
			log.Errorf("Failed to rename dir name from %s to %s, err: %s", f[0].Name(), serviceName, err)
			return nil, err
		}
	}

	return afero.NewIOFS(chartTree), nil
}

type treeGetter interface {
	GetTreeContents(owner, repo, path, branch string) (afero.Fs, error)
}

func getTreeGetter(codeHostID int) (treeGetter, error) {
	ch, err := codehost.GetCodeHostInfoByID(codeHostID)
	if err != nil {
		log.Errorf("Failed to get codeHost by id %d, err: %s", codeHostID, err)
		return nil, err
	}

	switch ch.Type {
	case setting.SourceFromGithub:
		return githubservice.NewClient(ch.AccessToken, config.ProxyHTTPSAddr()), nil
	case setting.SourceFromGitlab:
		return gitlabservice.NewClient(ch.Address, ch.AccessToken)
	default:
		// should not have happened here
		log.DPanicf("invalid source: %s", ch.Type)
		return nil, fmt.Errorf("invalid source: %s", ch.Type)
	}
}

func SaveAndUploadService(projectName, serviceName string, fileTree fs.FS) error {
	var wg wait.Group
	var err error

	wg.Start(func() {
		err1 := saveInMemoryFilesToDisk(projectName, serviceName, fileTree)
		if err1 != nil {
			err = err1
		}
	})
	wg.Start(func() {
		err2 := UploadFilesToS3(projectName, serviceName, fileTree)
		if err2 != nil {
			err = err2
		}
	})

	wg.Wait()

	return err
}

func preLoadServiceManifestsFromSource(svc *commonmodels.Service) error {
	tree, err := DownloadServiceManifestsFromSource(
		&DownloadFromSourceParams{CodehostID: svc.CodehostID, Owner: svc.RepoOwner, Repo: svc.RepoName, Path: svc.LoadPath, Branch: svc.BranchName},
		func(afero.Fs) (string, error) {
			return svc.ServiceName, nil
		})

	// save files to disk and upload them to s3
	if err = SaveAndUploadService(svc.ProductName, svc.ServiceName, tree); err != nil {
		log.Errorf("Failed to save or upload files for service %s in project %s, error: %s", svc.ServiceName, svc.ProductName, err)
		return err
	}

	return nil
}

func saveInMemoryFilesToDisk(projectName, serviceName string, fileTree fs.FS) error {
	root := config.LocalServicePath(projectName, serviceName)

	tmpRoot := root + ".bak"
	if err := os.Rename(root, tmpRoot); err != nil {
		return err
	}

	if err := fsutil.SaveToDisk(fileTree, root); err != nil {
		if err = os.RemoveAll(root); err != nil {
			log.Warnf("Failed to delete path %s, err: %s", root, err)
		}
		return os.Rename(tmpRoot, root)
	}

	if err := os.RemoveAll(tmpRoot); err != nil {
		log.Warnf("Failed to delete path %s, err: %s", tmpRoot, err)
	}

	return nil
}

func UploadFilesToS3(projectName, serviceName string, fileTree fs.FS) error {
	fileName := fmt.Sprintf("%s.tar.gz", serviceName)
	tmpDir := os.TempDir()
	tarball := filepath.Join(tmpDir, fileName)
	if err := fsutil.Tar(fileTree, tarball); err != nil {
		log.Errorf("Failed to archive tarball %s, err: %s", tarball, err)
		return err
	}
	s3Storage, err := s3service.FindDefaultS3()
	if err != nil {
		log.Errorf("Failed to find default s3, err:%v", err)
		return err
	}
	forcedPathStyle := true
	if s3Storage.Provider == setting.ProviderSourceAli {
		forcedPathStyle = false
	}
	client, err := s3tool.NewClient(s3Storage.Endpoint, s3Storage.Ak, s3Storage.Sk, s3Storage.Insecure, forcedPathStyle)
	if err != nil {
		log.Errorf("Failed to get s3 client, err: %s", err)
		return err
	}
	s3Storage.Subfolder = filepath.Join(s3Storage.Subfolder, config.ObjectStorageServicePath(projectName, serviceName))
	objectKey := s3Storage.GetObjectPath(fileName)
	if err = client.Upload(s3Storage.Bucket, tarball, objectKey); err != nil {
		log.Errorf("Failed to upload file %s to s3, err: %s", tarball, err)
		return err
	}
	if err = os.Remove(tarball); err != nil {
		log.Errorf("Failed to remove file %s, err: %s", tarball, err)
	}

	return nil
}
