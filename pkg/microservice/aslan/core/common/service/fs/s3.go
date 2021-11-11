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
	"path/filepath"

	"go.uber.org/zap"

	s3service "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/s3"
	"github.com/koderover/zadig/pkg/setting"
	s3tool "github.com/koderover/zadig/pkg/tool/s3"
	fsutil "github.com/koderover/zadig/pkg/util/fs"
)

func ArchiveAndUploadFilesToS3(fileTree fs.FS, name, s3Base string, logger *zap.SugaredLogger) error {
	tmpDir, err := os.MkdirTemp("", "")
	if err != nil {
		logger.Errorf("Failed to create temp dir, err: %s", err)
		return err
	}
	defer os.RemoveAll(tmpDir)

	tarball := fmt.Sprintf("%s.tar.gz", name)
	localPath := filepath.Join(tmpDir, tarball)
	if err = fsutil.Tar(fileTree, localPath); err != nil {
		logger.Errorf("Failed to archive tarball %s, err: %s", localPath, err)
		return err
	}
	s3Storage, err := s3service.FindDefaultS3()
	if err != nil {
		logger.Errorf("Failed to find default s3, err:%v", err)
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

func DeleteArchivedFileFromS3(name, s3Base string, logger *zap.SugaredLogger) error {
	s3, err := s3service.FindDefaultS3()
	if err != nil {
		logger.Errorf("Failed to find default s3, err: %s", err)
		return err
	}

	tarball := fmt.Sprintf("%s.tar.gz", name)
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

	return client.DeleteObjects(s3.Bucket, []string{s3Path})
}
