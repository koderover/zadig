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

package dockerfile

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/27149chen/afero"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"

	configbase "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/fs"
	"github.com/koderover/zadig/pkg/setting"
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
	ChartTemplates  []*Chart                      `json:"chartTemplates"`
}

func GetChartTemplate(name string, logger *zap.SugaredLogger) (*Chart, error) {
	chart, err := commonrepo.NewChartColl().Get(name)
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

	variables := make([]*commonmodels.ChartVariable, 0, len(chart.ChartVariables))
	for _, v := range chart.ChartVariables {
		variables = append(variables, &commonmodels.ChartVariable{
			Key:   v.Key,
			Value: v.Value,
		})
	}

	return &Chart{
		Name:       name,
		CodehostID: chart.CodeHostID,
		Owner:      chart.Owner,
		Repo:       chart.Repo,
		Path:       chart.Path,
		Branch:     chart.Branch,
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

	return getFileContent(name, chart.Path, filePath, fileName, logger)
}

func getFileContent(name, path, filePath, fileName string, logger *zap.SugaredLogger) ([]byte, error) {
	localBase := configbase.LocalChartTemplatePath(name)
	s3Base := configbase.ObjectStorageChartTemplatePath(name)
	if err := fs.PreloadFiles(name, localBase, s3Base, logger); err != nil {
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

func parseTemplateVariables(name, path string, logger *zap.SugaredLogger) ([]string, error) {
	valueYamlContent, err := getFileContent(name, path, "", setting.ValuesYaml, logger)
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

	sha1, err := processChartFromSource(name, args, logger)
	if err != nil {
		logger.Errorf("Failed to create chart %s, err: %s", name, err)
		return err
	}

	variablesNames, err := parseTemplateVariables(name, args.Path, logger)
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
		Repo:           args.Repo,
		Path:           args.Path,
		Branch:         args.Branch,
		CodeHostID:     args.CodehostID,
		Sha1:           sha1,
		ChartVariables: variables,
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

	variablesNames, err := parseTemplateVariables(name, args.Path, logger)
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

	return mongodb.NewChartColl().Update(&models.Chart{
		Name:           name,
		Owner:          args.Owner,
		Repo:           args.Repo,
		Path:           args.Path,
		Branch:         args.Branch,
		CodeHostID:     args.CodehostID,
		Sha1:           sha1,
		ChartVariables: variables,
	})
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
