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

	"github.com/27149chen/afero"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/wait"

	configbase "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/fs"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/templatestore/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/templatestore/repository/mongodb"
	fsutil "github.com/koderover/zadig/pkg/util/fs"
)

func GetChartTemplate(name string, logger *zap.SugaredLogger) (*Chart, error) {
	chart, err := mongodb.NewChartColl().Get(name)
	if err != nil {
		logger.Errorf("Failed to get chart template %s, err: %s", name, err)
		return nil, err
	}

	localBase := configbase.LocalChartTemplatePath(name)
	s3Base := configbase.ObjectStorageChartTemplatePath(name)
	if err = fs.PreloadFiles(name, localBase, s3Base, logger); err != nil {
		return nil, err
	}

	base := filepath.Base(chart.Path)
	localPath := filepath.Join(localBase, base)
	fis, err := fs.GetFileInfos(os.DirFS(localPath))
	if err != nil {
		logger.Errorf("Failed to get local chart template %s from path %s, err: %s", name, localPath, err)
		return nil, err
	}

	return &Chart{
		Name:       name,
		CodehostID: chart.CodeHostID,
		Owner:      chart.Owner,
		Repo:       chart.Repo,
		Path:       chart.Path,
		Branch:     chart.Branch,
		Files:      fis,
	}, nil
}

func ListChartTemplates(logger *zap.SugaredLogger) ([]*Chart, error) {
	cs, err := mongodb.NewChartColl().List()
	if err != nil {
		logger.Errorf("Failed to list chart templates, err: %s", err)
		return nil, err
	}

	res := make([]*Chart, 0, len(cs))
	for _, c := range cs {
		res = append(res, &Chart{
			Name:       c.Name,
			CodehostID: c.CodeHostID,
			Owner:      c.Owner,
			Repo:       c.Repo,
			Path:       c.Path,
			Branch:     c.Branch,
		})
	}

	return res, nil
}

func GetFileContent(name, filePath, fileName string, logger *zap.SugaredLogger) ([]byte, error) {
	chart, err := mongodb.NewChartColl().Get(name)
	if err != nil {
		logger.Errorf("Failed to get chart template %s, err: %s", name, err)
		return nil, err
	}

	localBase := configbase.LocalChartTemplatePath(name)
	s3Base := configbase.ObjectStorageChartTemplatePath(name)
	if err = fs.PreloadFiles(name, localBase, s3Base, logger); err != nil {
		return nil, err
	}

	base := filepath.Base(chart.Path)
	file := filepath.Join(localBase, base, filePath, fileName)
	fileContent, err := os.ReadFile(file)
	if err != nil {
		logger.Errorf("Failed to read file %s, err: %s", file, err)
		return nil, err
	}

	return fileContent, nil
}

func AddChartTemplate(name string, args *fs.DownloadFromSourceArgs, logger *zap.SugaredLogger) error {
	if mongodb.NewChartColl().Exist(name) {
		return fmt.Errorf("a chart template with name %s is already existing", name)
	}

	sha1, err := processChartFromSource(name, args, logger)
	if err != nil {
		logger.Errorf("Failed to create chart %s, err: %s", name, err)
		return err
	}

	return mongodb.NewChartColl().Create(&models.Chart{
		Name:       name,
		Owner:      args.Owner,
		Repo:       args.Repo,
		Path:       args.Path,
		Branch:     args.Branch,
		CodeHostID: args.CodehostID,
		Sha1:       sha1,
	})
}

func UpdateChartTemplate(name string, args *fs.DownloadFromSourceArgs, logger *zap.SugaredLogger) error {
	chart, err := mongodb.NewChartColl().Get(name)
	if err != nil {
		logger.Errorf("Failed to get chart template %s, err: %s", name, err)
		return err
	}

	sha1, err := processChartFromSource(name, args, logger)
	if err != nil {
		logger.Errorf("Failed to update chart %s, err: %s", name, err)
		return err
	}

	if chart.Sha1 == sha1 {
		logger.Debug("Chart %s has no changes, skip updating.", name)
		return nil
	}

	return mongodb.NewChartColl().Update(&models.Chart{
		Name:       name,
		Owner:      args.Owner,
		Repo:       args.Repo,
		Path:       args.Path,
		Branch:     args.Branch,
		CodeHostID: args.CodehostID,
		Sha1:       sha1,
	})
}

func RemoveChartTemplate(name string, logger *zap.SugaredLogger) error {
	err := mongodb.NewChartColl().Delete(name)
	if err != nil {
		logger.Errorf("Failed to delete chart template %s, err: %s", name, err)
		return err
	}

	if err = fs.DeleteArchivedFileFromS3(name, configbase.ObjectStorageChartTemplatePath(name), logger); err != nil {
		logger.Warnf("Failed to delete file %s, err: %s", name, err)
	}

	return nil
}

func processChartFromSource(name string, args *fs.DownloadFromSourceArgs, logger *zap.SugaredLogger) (string, error) {
	tree, err := fs.DownloadFilesFromSource(args, func(a afero.Fs) (string, error) {
		return "", nil
	})

	if err != nil {
		logger.Errorf("Failed to download files with option %+v, err: %s", args, err)
		return "", err
	}

	var sha1 string
	if name == "" {
		name = filepath.Base(args.Path)
	}

	var wg wait.Group
	wg.Start(func() {
		logger.Debug("Start to save and upload chart")
		localBase := configbase.LocalChartTemplatePath(name)
		s3Base := configbase.ObjectStorageChartTemplatePath(name)

		err1 := fs.SaveAndUploadFiles(tree, name, localBase, s3Base, logger)
		if err1 != nil {
			logger.Errorf("Failed to save files to disk, err: %s", err1)
			err = err1
			return
		}

		logger.Debug("Finish to save and upload chart")
	})

	wg.Start(func() {
		logger.Debug("Start to calculate sha1 for chart")
		tmpDir, err1 := os.MkdirTemp("", "")
		if err1 != nil {
			logger.Errorf("Failed to create temp dir, err: %s", err1)
			err = err1
			return
		}
		defer func() {
			_ = os.RemoveAll(tmpDir)
		}()

		fileName := fmt.Sprintf("%s.tar.gz", filepath.Base(args.Path))
		tarball := filepath.Join(tmpDir, fileName)
		if err1 = fsutil.Tar(tree, tarball); err1 != nil {
			logger.Errorf("Failed to archive files to %s, err: %s", tarball, err1)
			err = err1
			return
		}

		sha1, err1 = fsutil.Sha1(os.DirFS(tmpDir), fileName)
		if err1 != nil {
			logger.Errorf("Failed to calculate sha1 for file %s, err: %s", tarball, err1)
			err = err1
			return
		}

		logger.Debug("Finish to calculate sha1 for chart")
	})

	wg.Wait()

	if err != nil {
		return "", err
	}

	return sha1, nil
}
