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

package fs

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"github.com/otiai10/copy"
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/command"
	s3service "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/s3"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/client/systemconfig"
	s3tool "github.com/koderover/zadig/pkg/tool/s3"
	fsutil "github.com/koderover/zadig/pkg/util/fs"
)

func ArchiveAndUploadFilesToSpecifiedS3(fileTree fs.FS, names []string, s3Base, s3Id string, logger *zap.SugaredLogger) error {
	s3Storage, err := s3service.FindS3ById(s3Id)
	if err != nil {
		logger.Errorf("Failed to find default s3, err:%v", err)
		return err
	}
	return archiveAndUploadFiles(fileTree, names, s3Base, s3Storage, logger)
}

func ArchiveAndUploadFilesToS3(fileTree fs.FS, names []string, s3Base string, logger *zap.SugaredLogger) error {
	s3Storage, err := s3service.FindDefaultS3()
	if err != nil {
		logger.Errorf("Failed to find default s3, err:%v", err)
		return err
	}
	return archiveAndUploadFiles(fileTree, names, s3Base, s3Storage, logger)
}

// archiveAndUploadFiles archive local files and upload to default s3 storage
// if multiple names appointed, s3storage.copy will be used to handle extra names
func archiveAndUploadFiles(fileTree fs.FS, names []string, s3Base string, s3Storage *s3service.S3, logger *zap.SugaredLogger) error {
	if len(names) == 0 {
		return fmt.Errorf("names not appointed")
	}

	tmpDir, err := os.MkdirTemp("", "")
	if err != nil {
		logger.Errorf("Failed to create temp dir, err: %s", err)
		return err
	}
	defer os.RemoveAll(tmpDir)

	name := names[0]
	copies := names[1:]

	tarball := fmt.Sprintf("%s.tar.gz", name)
	localPath := filepath.Join(tmpDir, tarball)
	if err = fsutil.Tar(fileTree, localPath); err != nil {
		logger.Errorf("Failed to archive tarball %s, err: %s", localPath, err)
		return err
	}

	forcedPathStyle := true
	if s3Storage.Provider == setting.ProviderSourceAli {
		forcedPathStyle = false
	}
	client, err := s3tool.NewClient(s3Storage.Endpoint, s3Storage.Ak, s3Storage.Sk, s3Storage.Insecure, forcedPathStyle)
	if err != nil {
		logger.Errorf("Failed to get s3 client, err: %s", err)
		return err
	}
	s3Path := filepath.Join(s3Storage.Subfolder, s3Base, tarball)
	if err = client.Upload(s3Storage.Bucket, localPath, s3Path); err != nil {
		logger.Errorf("Failed to upload file %s to s3, err: %s", localPath, err)
		return err
	}

	// copy file to avoid duplicated file transfer
	for _, copyName := range copies {
		targetPath := filepath.Join(s3Storage.Subfolder, s3Base, fmt.Sprintf("%s.tar.gz", copyName))
		err = client.CopyObject(s3Storage.Bucket, s3Path, targetPath)
		if err != nil {
			logger.Errorf("Failed to copy object from %s to %s", s3Path, targetPath)
			return err
		}
	}

	return nil
}

func DownloadAndExtractFilesFromS3(name, localBase, s3Base string, logger *zap.SugaredLogger) error {
	tmpDir, err := os.MkdirTemp("", "")
	if err != nil {
		logger.Errorf("Failed to create temp dir, err: %s", err)
	}
	defer os.RemoveAll(tmpDir)
	s3, err := s3service.FindDefaultS3()
	if err != nil {
		logger.Errorf("Failed to find default s3, err: %s", err)
		return err
	}
	tarball := fmt.Sprintf("%s.tar.gz", name)
	localPath := filepath.Join(tmpDir, tarball)
	s3Path := filepath.Join(s3.Subfolder, s3Base, tarball)

	forcedPathStyle := true
	if s3.Provider == setting.ProviderSourceAli {
		forcedPathStyle = false
	}

	client, err := s3tool.NewClient(s3.Endpoint, s3.Ak, s3.Sk, s3.Insecure, forcedPathStyle)
	if err != nil {
		logger.Errorf("Failed to create s3 client, err: %s", err)
		return err
	}

	if err = client.Download(s3.Bucket, s3Path, localPath); err != nil {
		logger.Errorf("Failed to download file from s3, err: %s", err)
		return err
	}

	if err = fsutil.Untar(localPath, localBase); err != nil {
		logger.Errorf("Failed to extract tarball %s, err: %s", localPath, err)
		return err
	}
	return nil
}

func DeleteArchivedFileFromS3(names []string, s3Base string, logger *zap.SugaredLogger) error {
	s3, err := s3service.FindDefaultS3()
	if err != nil {
		logger.Errorf("Failed to find default s3, err: %s", err)
		return err
	}

	s3PathList := make([]string, 0, len(names))
	for _, name := range s3PathList {
		tarball := fmt.Sprintf("%s.tar.gz", name)
		s3Path := filepath.Join(s3.Subfolder, s3Base, tarball)
		s3PathList = append(s3PathList, s3Path)
	}

	forcedPathStyle := true
	if s3.Provider == setting.ProviderSourceAli {
		forcedPathStyle = false
	}
	client, err := s3tool.NewClient(s3.Endpoint, s3.Ak, s3.Sk, s3.Insecure, forcedPathStyle)
	if err != nil {
		logger.Errorf("Failed to create s3 client, err: %s", err)
		return err
	}

	return client.DeleteObjects(s3.Bucket, s3PathList)
}

func DownloadAndCopyFilesFromGerrit(name, localBase string, logger *zap.SugaredLogger) error {
	chartTemplate, err := mongodb.NewChartColl().Get(name)
	base := path.Join(config.S3StoragePath(), chartTemplate.Repo)
	if err := os.RemoveAll(base); err != nil {
		logger.Warnf("Failed to remove dir, err:%s", err)
	}
	detail, err := systemconfig.New().GetCodeHost(chartTemplate.CodeHostID)
	if err != nil {
		logger.Errorf("Failed to GetCodehostDetail, err:%s", err)
		return err
	}
	err = command.RunGitCmds(detail, "default", "default", chartTemplate.Repo, chartTemplate.Branch, "origin")
	if err != nil {
		logger.Errorf("Failed to runGitCmds, err:%s", err)
		return err
	}

	if err := copy.Copy(path.Join(base, chartTemplate.Path), path.Join(localBase, path.Base(chartTemplate.Path))); err != nil {
		logger.Errorf("Failed to copy files for helm chart template %s, error: %s", name, err)
		return err
	}

	return nil
}

func DownloadAndCopyFilesFromGitee(name, localBase string, logger *zap.SugaredLogger) error {
	chartTemplate, err := mongodb.NewChartColl().Get(name)
	if err != nil {
		logger.Errorf("Failed to get chart template, err:%s", err)
		return err
	}
	base := path.Join(config.S3StoragePath(), chartTemplate.Repo)
	if err := os.RemoveAll(base); err != nil {
		logger.Warnf("Failed to remove dir, err:%s", err)
	}
	detail, err := systemconfig.New().GetCodeHost(chartTemplate.CodeHostID)
	if err != nil {
		logger.Errorf("Failed to get codehost detail, err:%s", err)
		return err
	}
	err = command.RunGitCmds(detail, chartTemplate.Owner, chartTemplate.GetNamespace(), chartTemplate.Repo, chartTemplate.Branch, "origin")
	if err != nil {
		logger.Errorf("Failed to runGitCmds, err:%s", err)
		return err
	}

	if err := copy.Copy(path.Join(base, chartTemplate.Path), path.Join(localBase, path.Base(chartTemplate.Path))); err != nil {
		logger.Errorf("Failed to copy files for helm chart template %s, error: %s", name, err)
		return err
	}
	return nil
}
