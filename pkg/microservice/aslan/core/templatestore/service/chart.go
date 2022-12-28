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
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/27149chen/afero"
	"github.com/otiai10/copy"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"

	configbase "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/command"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/fs"
	s3service "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/s3"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/template"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/service/service"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/client/systemconfig"
	"github.com/koderover/zadig/pkg/tool/log"
	s3tool "github.com/koderover/zadig/pkg/tool/s3"
	fsutil "github.com/koderover/zadig/pkg/util/fs"
)

var (
	variableExtractRegexp              = regexp.MustCompile("{{.(\\w*)}}")
	ChartTemplateDefaultSystemVariable = map[string]string{
		setting.TemplateVariableProduct: setting.TemplateVariableProductDescription,
		setting.TemplateVariableService: setting.TemplateVariableServiceDescription,
	}
)

type ChartTemplateListResp struct {
	SystemVariables []*commonmodels.ChartVariable `json:"systemVariables"`
	ChartTemplates  []*template.Chart             `json:"chartTemplates"`
}

func GetChartTemplate(name string, logger *zap.SugaredLogger) (*template.Chart, error) {
	chart, err := commonrepo.NewChartColl().Get(name)
	if err != nil {
		logger.Errorf("Failed to get chart template %s, err: %s", name, err)
		return nil, err
	}

	localBase := configbase.LocalChartTemplatePath(name)
	s3Base := configbase.ObjectStorageChartTemplatePath(name)
	if err = fs.PreloadFiles(name, localBase, s3Base, chart.Source, logger); err != nil {
		return nil, err
	}

	base := filepath.Base(chart.Path)
	localPath := filepath.Join(localBase, base)
	fis, err := fs.GetFileInfos(os.DirFS(localPath))
	if err != nil {
		logger.Errorf("Failed to get local chart template %s from path %s, err: %s", name, localPath, err)
		return nil, err
	}

	variables := make([]*commonmodels.ChartVariable, 0, len(chart.ChartVariables))
	for _, v := range chart.ChartVariables {
		variables = append(variables, &commonmodels.ChartVariable{
			Key:   v.Key,
			Value: v.Value,
		})
	}

	return &template.Chart{
		Name:       name,
		CodehostID: chart.CodeHostID,
		Owner:      chart.Owner,
		Repo:       chart.Repo,
		Path:       chart.Path,
		Branch:     chart.Branch,
		Namespace:  chart.GetNamespace(),
		Files:      fis,
		Variables:  variables,
	}, nil
}

func GetChartTemplateVariables(name string, logger *zap.SugaredLogger) ([]*commonmodels.ChartVariable, error) {
	chart, err := mongodb.NewChartColl().Get(name)
	if err != nil {
		logger.Errorf("Failed to get chart template %s, err: %s", name, err)
		return nil, err
	}

	variables := make([]*commonmodels.ChartVariable, 0)
	for _, v := range chart.ChartVariables {
		variables = append(variables, &commonmodels.ChartVariable{
			Key:   v.Key,
			Value: v.Value,
		})
	}
	return variables, nil
}

func getChartTemplateDefaultVariables() []*commonmodels.ChartVariable {
	resp := make([]*commonmodels.ChartVariable, 0)
	for key, description := range ChartTemplateDefaultSystemVariable {
		resp = append(resp, &commonmodels.ChartVariable{
			Key:         key,
			Description: description,
		})
	}
	return resp
}

func ListChartTemplates(logger *zap.SugaredLogger) (*ChartTemplateListResp, error) {
	cs, err := mongodb.NewChartColl().List()
	if err != nil {
		logger.Errorf("Failed to list chart templates, err: %s", err)
		return nil, err
	}

	res := make([]*template.Chart, 0, len(cs))
	for _, c := range cs {
		res = append(res, &template.Chart{
			Name:       c.Name,
			CodehostID: c.CodeHostID,
			Owner:      c.Owner,
			Namespace:  c.GetNamespace(),
			Repo:       c.Repo,
			Path:       c.Path,
			Branch:     c.Branch,
		})
	}

	ret := &ChartTemplateListResp{
		SystemVariables: getChartTemplateDefaultVariables(),
		ChartTemplates:  res,
	}

	return ret, nil
}

func GetFileContentForTemplate(name, filePath, fileName string, logger *zap.SugaredLogger) ([]byte, error) {
	chart, err := mongodb.NewChartColl().Get(name)
	if err != nil {
		logger.Errorf("Failed to get chart template %s, err: %s", name, err)
		return nil, err
	}

	return getFileContent(name, chart.Path, filePath, fileName, chart.Source, logger)
}

func getFileContent(name, path, filePath, fileName, source string, logger *zap.SugaredLogger) ([]byte, error) {
	localBase := configbase.LocalChartTemplatePath(name)
	s3Base := configbase.ObjectStorageChartTemplatePath(name)
	if err := fs.PreloadFiles(name, localBase, s3Base, source, logger); err != nil {
		return nil, err
	}

	base := filepath.Base(path)
	file := filepath.Join(localBase, base, filePath, fileName)
	fileContent, err := os.ReadFile(file)
	if err != nil {
		logger.Errorf("Failed to read file %s, err: %s", file, err)
		return nil, err
	}

	return fileContent, nil
}

func parseTemplateVariables(name, path, source string, logger *zap.SugaredLogger) ([]string, error) {
	valueYamlContent, err := getFileContent(name, path, "", setting.ValuesYaml, source, logger)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read values.yaml")
	}
	strSet := sets.NewString()
	allMatches := variableExtractRegexp.FindAllStringSubmatch(string(valueYamlContent), -1)
	for _, match := range allMatches {
		if len(match) != 2 {
			continue
		}
		strSet.Insert(match[1])
	}
	return strSet.List(), nil
}

func AddChartTemplate(name string, args *fs.DownloadFromSourceArgs, logger *zap.SugaredLogger) error {
	if mongodb.NewChartColl().Exist(name) {
		return fmt.Errorf("a chart template with name %s is already existing", name)
	}

	ch, err := systemconfig.New().GetCodeHost(args.CodehostID)
	if err != nil {
		log.Errorf("Failed to get codeHost by id %d, err: %s", args.CodehostID, err)
		return err
	}
	var (
		sha1    string
		loadErr error
	)
	switch ch.Type {
	case setting.SourceFromGerrit, setting.SourceFromGitee, setting.SourceFromGiteeEE, setting.SourceFromOther:
		sha1, loadErr = processChartFromGitRepo(name, args, logger)
	default:
		sha1, loadErr = processChartFromSource(name, args, logger)
	}
	if loadErr != nil {
		logger.Errorf("Failed to create chart %s, err: %s", name, loadErr)
		return loadErr
	}

	variablesNames, err := parseTemplateVariables(name, args.Path, ch.Type, logger)
	if err != nil {
		return errors.Wrapf(err, "failed to prase variables")
	}

	variables := make([]*commonmodels.ChartVariable, 0, len(variablesNames))
	for _, v := range variablesNames {
		variables = append(variables, &commonmodels.ChartVariable{
			Key: v,
		})
	}

	return mongodb.NewChartColl().Create(&models.Chart{
		Name:           name,
		Owner:          args.Owner,
		Namespace:      args.Namespace,
		Repo:           args.Repo,
		Path:           args.Path,
		Branch:         args.Branch,
		CodeHostID:     args.CodehostID,
		Sha1:           sha1,
		ChartVariables: variables,
		Source:         ch.Type,
	})
}

func UpdateChartTemplate(name string, args *fs.DownloadFromSourceArgs, logger *zap.SugaredLogger) error {
	chart, err := mongodb.NewChartColl().Get(name)
	if err != nil {
		logger.Errorf("Failed to get chart template %s, err: %s", name, err)
		return err
	}
	ch, err := systemconfig.New().GetCodeHost(args.CodehostID)
	if err != nil {
		log.Errorf("Failed to get codeHost by id %d, err: %s", args.CodehostID, err)
		return err
	}
	var (
		sha1    string
		loadErr error
	)

	switch ch.Type {
	case setting.SourceFromGerrit, setting.SourceFromGitee, setting.SourceFromGiteeEE, setting.SourceFromOther:
		sha1, loadErr = processChartFromGitRepo(name, args, logger)
	default:
		sha1, loadErr = processChartFromSource(name, args, logger)
	}

	if loadErr != nil {
		logger.Errorf("Failed to update chart %s, err: %s", name, loadErr)
		return loadErr
	}

	if chart.Sha1 == sha1 {
		logger.Debug("Chart %s has no changes, skip updating.", name)
		return nil
	}

	variablesNames, err := parseTemplateVariables(name, args.Path, ch.Type, logger)
	if err != nil {
		return errors.Wrapf(err, "failed to prase variables")
	}

	variables := make([]*commonmodels.ChartVariable, 0, len(variablesNames))
	curVariableMap := chart.GetVariableMap()

	for _, vName := range variablesNames {
		variable := &commonmodels.ChartVariable{
			Key: vName,
		}
		if v, ok := curVariableMap[vName]; ok {
			variable.Value = v.Value
		}
		variables = append(variables, variable)
	}

	err = mongodb.NewChartColl().Update(&models.Chart{
		Name:           name,
		Owner:          args.Owner,
		Namespace:      args.Namespace,
		Repo:           args.Repo,
		Path:           args.Path,
		Branch:         args.Branch,
		CodeHostID:     args.CodehostID,
		Sha1:           sha1,
		ChartVariables: variables,
		Source:         ch.Type,
	})

	return err
}

func GetChartTemplateReference(name string, logger *zap.SugaredLogger) ([]*template.ServiceReference, error) {
	ret := make([]*template.ServiceReference, 0)
	referenceList, err := commonrepo.NewServiceColl().GetChartTemplateReference(name)
	if err != nil {
		logger.Errorf("Failed to get chart reference for template name: %s, the error is: %s", name, err)
		return ret, err
	}
	for _, reference := range referenceList {
		ret = append(ret, &template.ServiceReference{
			ServiceName: reference.ServiceName,
			ProjectName: reference.ProductName,
		})
	}
	return ret, nil
}

func SyncHelmTemplateReference(userName, name string, logger *zap.SugaredLogger) error {
	return service.SyncServiceFromTemplate(userName, setting.SourceFromChartTemplate, "", name, logger)
}

func UpdateChartTemplateVariables(name string, args []*commonmodels.Variable, logger *zap.SugaredLogger) error {
	chart, err := mongodb.NewChartColl().Get(name)
	if err != nil {
		logger.Errorf("Failed to get chart template %s, err: %s", name, err)
		return err
	}

	requestKeys := sets.NewString()
	variables := make([]*commonmodels.ChartVariable, 0)
	for _, variable := range args {
		variables = append(variables, &commonmodels.ChartVariable{
			Key:   variable.Key,
			Value: variable.Value,
		})
		requestKeys.Insert(variable.Key)
	}

	curKeys := sets.NewString()
	for _, variable := range chart.ChartVariables {
		curKeys.Insert(variable.Key)
	}

	if !requestKeys.Equal(curKeys) {
		return fmt.Errorf("request variable keys are different from keys in file")
	}

	err = mongodb.NewChartColl().Update(&models.Chart{
		Name:           name,
		Owner:          chart.Owner,
		Namespace:      chart.Namespace,
		Repo:           chart.Repo,
		Path:           chart.Path,
		Branch:         chart.Branch,
		CodeHostID:     chart.CodeHostID,
		Sha1:           chart.Sha1,
		ChartVariables: variables,
	})

	if err != nil {
		logger.Errorf("failed to save variables err %s", err)
		return fmt.Errorf("failed to save variables")
	}
	return nil
}

func RemoveChartTemplate(name string, logger *zap.SugaredLogger) error {
	err := mongodb.NewChartColl().Delete(name)
	if err != nil {
		logger.Errorf("Failed to delete chart template %s, err: %s", name, err)
		return err
	}

	if err = fs.DeleteArchivedFileFromS3([]string{name}, configbase.ObjectStorageChartTemplatePath(name), logger); err != nil {
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

		err = os.RemoveAll(localBase)
		if err != nil {
			log.Errorf("failed to remove current template dir: %s, err: %s", localBase, err)
			return
		}

		err1 := fs.SaveAndUploadFiles(tree, []string{name}, localBase, s3Base, logger)
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

func processChartFromGitRepo(name string, args *fs.DownloadFromSourceArgs, logger *zap.SugaredLogger) (string, error) {
	var (
		wg   wait.Group
		sha1 string
		err  error
	)

	codehostDetail, err := systemconfig.New().GetCodeHost(args.CodehostID)
	if err != nil {
		log.Errorf("Failed to get codeHost by id %d, err: %s", args.CodehostID, err)
		return "", err
	}

	if codehostDetail.Type == setting.SourceFromOther {
		err = command.RunGitCmds(codehostDetail, args.Namespace, args.Namespace, args.Repo, args.Branch, "origin")
		if err != nil {
			log.Errorf("Failed to clone the repo, namespace: [%s], name: [%s], branch: [%s], error: %s", args.Namespace, args.Repo, args.Branch, err)
			return "", err
		}
	}

	base := path.Join(config.S3StoragePath(), args.Repo)
	filePath := strings.TrimLeft(args.Path, "/")
	currentChartPath := path.Join(base, filePath)
	wg.Start(func() {
		logger.Debug("Start to save and upload chart")
		localBase := configbase.LocalChartTemplatePath(name)

		err = os.RemoveAll(path.Join(localBase, path.Base(args.Path)))
		if err != nil {
			log.Errorf("failed to remove current template dir: %s, err: %s", localBase, err)
			return
		}

		err = copy.Copy(currentChartPath, path.Join(localBase, path.Base(args.Path)))
		if err != nil {
			logger.Errorf("Failed to save files to disk, err: %s", err)
			return
		}

		logger.Debug("Finish to save chart")
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

		fileName := fmt.Sprintf("%s.tar.gz", name)
		tarball := filepath.Join(tmpDir, fileName)

		chartDir := path.Join(tmpDir, "chart")
		mkdirErr := os.Mkdir(chartDir, 0755)
		if mkdirErr != nil {
			logger.Errorf("Failed to mkdir %s, err: %s", chartDir, mkdirErr)
			err = mkdirErr
			return
		}

		newFilePath := path.Join(chartDir, filepath.Base(args.Path))
		mkdirErr = os.Mkdir(newFilePath, 0755)
		if mkdirErr != nil {
			logger.Errorf("Failed to mkdir %s, err: %s", newFilePath, mkdirErr)
			err = mkdirErr
			return
		}

		copyErr := copy.Copy(currentChartPath, newFilePath)
		if copyErr != nil {
			logger.Errorf("Failed to copy directory, err: %s", copyErr)
			err = copyErr
			return
		}

		tree := os.DirFS(chartDir)
		if err1 = fsutil.Tar(tree, tarball); err1 != nil {
			logger.Errorf("Failed to archive files to %s, err: %s", tarball, err1)
			err = err1
			return
		}

		s3Storage, err2 := s3service.FindDefaultS3()
		if err2 != nil {
			logger.Errorf("Failed to find default s3, err:%v", err)
			err = err2
			return
		}

		forcedPathStyle := true
		if s3Storage.Provider == setting.ProviderSourceAli {
			forcedPathStyle = false
		}

		s3Client, err2 := s3tool.NewClient(s3Storage.Endpoint, s3Storage.Ak, s3Storage.Sk, s3Storage.Region, s3Storage.Insecure, forcedPathStyle)
		if err2 != nil {
			logger.Errorf("Failed to create s3 client, err: %s", err)
			err = err2
			return
		}

		s3Base := configbase.ObjectStorageChartTemplatePath(name)
		s3Path := filepath.Join(s3Storage.Subfolder, s3Base, fileName)
		err2 = s3Client.Upload(s3Storage.Bucket, tarball, s3Path)
		if err != nil {
			logger.Errorf("Failed to upload to s3 client, err: %s", err)
			err = err2
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
