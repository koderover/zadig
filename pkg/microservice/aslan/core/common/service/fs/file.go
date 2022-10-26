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
	"io/fs"
	"os"

	"github.com/otiai10/copy"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/koderover/zadig/pkg/setting"
	fsutil "github.com/koderover/zadig/pkg/util/fs"
)

// PreloadFiles downloads a tarball from object storage and extracts it to a local path for further usage.
// It happens only if files do not exist in local disk.
func PreloadFiles(name, localBase, s3Base, source string, logger *zap.SugaredLogger) error {
	ok, err := fsutil.DirExists(localBase)
	if err != nil {
		logger.Errorf("Failed to check if dir %s is exiting, err: %s", localBase, err)
		return err
	}
	if ok {
		return nil
	}

	switch source {
	case setting.SourceFromGerrit:
		if err = DownloadAndCopyFilesFromGerrit(name, localBase, logger); err != nil {
			logger.Errorf("Failed to download files from s3, err: %s", err)
			return err
		}
	case setting.SourceFromGitee, setting.SourceFromGiteeEE:
		if err = DownloadAndCopyFilesFromGitee(name, localBase, logger); err != nil {
			logger.Errorf("Failed to download files from s3, err: %s", err)
			return err
		}
	default:
		if err = DownloadAndExtractFilesFromS3(name, localBase, s3Base, logger); err != nil {
			logger.Errorf("Failed to download files from s3, err: %s", err)
			return err
		}
	}

	return nil
}

// SaveAndUploadFiles saves a tree of files to local disk, at the same time, archives them and uploads to object storage.
func SaveAndUploadFiles(fileTree fs.FS, names []string, localBase, s3Base string, logger *zap.SugaredLogger) error {
	var wg wait.Group
	var err error

	wg.Start(func() {
		err1 := saveInMemoryFilesToDisk(fileTree, localBase, logger)
		if err1 != nil {
			logger.Errorf("Failed to save files to disk, err: %s", err1)
			err = err1
		}
	})

	wg.Start(func() {
		err2 := ArchiveAndUploadFilesToS3(fileTree, names, s3Base, logger)
		if err2 != nil {
			logger.Errorf("Failed to upload files to s3, err: %s", err2)
			err = err2
		}
	})

	wg.Wait()

	return err
}

// CopyAndUploadFiles copy a tree of files to other dir, at the same time, archives them and uploads to object storage.
func CopyAndUploadFiles(names []string, localBase, s3Base, zipPath, currentChartPath string, logger *zap.SugaredLogger) error {
	err := copy.Copy(currentChartPath, localBase)
	if err != nil {
		logger.Errorf("failed to copy chart info, err %s", err)
		return err
	}

	if s3Base == "" {
		return nil
	}
	fileTree := os.DirFS(zipPath)
	err = ArchiveAndUploadFilesToS3(fileTree, names, s3Base, logger)
	if err != nil {
		logger.Errorf("Failed to upload files to s3, err: %s", err)
	}
	return err
}

func saveInMemoryFilesToDisk(fileTree fs.FS, root string, logger *zap.SugaredLogger) error {
	ok, err := fsutil.DirExists(root)
	if err != nil {
		return err
	}

	if !ok {
		return fsutil.SaveToDisk(fileTree, root)
	}

	tmpRoot := root + ".bak"
	if err = os.Rename(root, tmpRoot); err != nil {
		return err
	}

	if err = fsutil.SaveToDisk(fileTree, root); err != nil {
		logger.Errorf("Failed to save files, err: %s", err)
		if err1 := os.RemoveAll(root); err1 != nil {
			logger.Warnf("Failed to delete path %s, err: %s", root, err1)
		}
		if err1 := os.Rename(tmpRoot, root); err1 != nil {
			logger.Errorf("Failed to rename path from %s to %s, err: %s", tmpRoot, root, err1)
		}

		return err
	}

	if err := os.RemoveAll(tmpRoot); err != nil {
		logger.Warnf("Failed to delete path %s, err: %s", tmpRoot, err)
	}

	return nil
}
