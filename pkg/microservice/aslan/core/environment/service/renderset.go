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
	"sync"

	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"

	templatemodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	fsservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/fs"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
	yamlutil "github.com/koderover/zadig/pkg/util/yaml"
)

type DefaultValuesResp struct {
	DefaultValues string                     `json:"defaultValues"`
	YamlData      *templatemodels.CustomYaml `json:"yaml_data,omitempty"`
}

type YamlContentRequestArg struct {
	CodehostID  int    `json:"codehostID" form:"codehostID"`
	Owner       string `json:"owner" form:"owner"`
	Repo        string `json:"repo" form:"repo"`
	Namespace   string `json:"namespace" form:"namespace"`
	Branch      string `json:"branch" form:"branch"`
	RepoLink    string `json:"repoLink" form:"repoLink"`
	ValuesPaths string `json:"valuesPaths" form:"valuesPaths"`
}

func fromGitRepo(source string) bool {
	if source == "" {
		return true
	}
	if source == setting.SourceFromGitRepo {
		return true
	}
	return false
}

func syncYamlFromVariableSet(yamlData *templatemodels.CustomYaml, curValue string) (bool, string, error) {
	if yamlData.Source != setting.SourceFromVariableSet {
		return false, "", nil
	}
	variableSet, err := commonrepo.NewVariableSetColl().Find(&commonrepo.VariableSetFindOption{
		ID: yamlData.SourceID,
	})
	if err != nil {
		return false, "", err
	}
	equal, err := yamlutil.Equal(variableSet.VariableYaml, curValue)
	if err != nil || equal {
		return false, "", err
	}
	return true, variableSet.VariableYaml, nil
}

func syncYamlFromGit(yamlData *templatemodels.CustomYaml, curValue string) (bool, string, error) {
	if !fromGitRepo(yamlData.Source) {
		return false, "", nil
	}
	sourceDetail, err := service.UnMarshalSourceDetail(yamlData.SourceDetail)
	if err != nil {
		return false, "", err
	}
	if sourceDetail.GitRepoConfig == nil {
		log.Warnf("git repo config is nil")
		return false, "", nil
	}
	repoConfig := sourceDetail.GitRepoConfig

	valuesYAML, err := fsservice.DownloadFileFromSource(&fsservice.DownloadFromSourceArgs{
		CodehostID: repoConfig.CodehostID,
		Namespace:  repoConfig.Namespace,
		Owner:      repoConfig.Owner,
		Repo:       repoConfig.Repo,
		Path:       sourceDetail.LoadPath,
		Branch:     repoConfig.Branch,
	})
	if err != nil {
		return false, "", err
	}
	equal, err := yamlutil.Equal(string(valuesYAML), curValue)
	if err != nil || equal {
		return false, "", err
	}
	return true, string(valuesYAML), nil
}

// SyncYamlFromSource sync values.yaml from source
// NOTE for git source currently only support gitHub and gitlab
func SyncYamlFromSource(yamlData *templatemodels.CustomYaml, curValue string) (bool, string, error) {
	if yamlData == nil || !yamlData.AutoSync {
		return false, "", nil
	}
	if yamlData.Source == setting.SourceFromVariableSet {
		return syncYamlFromVariableSet(yamlData, curValue)
	}
	return syncYamlFromGit(yamlData, curValue)
}

func GetDefaultValues(productName, envName string, log *zap.SugaredLogger) (*DefaultValuesResp, error) {
	ret := &DefaultValuesResp{}

	productInfo, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    productName,
		EnvName: envName,
	})
	if err == mongo.ErrNoDocuments {
		return ret, nil
	}
	if err != nil {
		log.Errorf("failed to query product info, productName %s envName %s err %s", productName, envName, err)
		return nil, fmt.Errorf("failed to query product info, productName %s envName %s", productName, envName)
	}

	if productInfo.Render == nil {
		return nil, fmt.Errorf("invalid product, nil render data")
	}

	opt := &commonrepo.RenderSetFindOption{
		Name:        productInfo.Render.Name,
		Revision:    productInfo.Render.Revision,
		ProductTmpl: productName,
		EnvName:     productInfo.EnvName,
	}
	rendersetObj, existed, err := commonrepo.NewRenderSetColl().FindRenderSet(opt)
	if err != nil {
		log.Errorf("failed to query renderset info, name %s err %s", productInfo.Render.Name, err)
		return nil, err
	}
	if !existed {
		return ret, nil
	}
	ret.DefaultValues = rendersetObj.DefaultValues
	err = service.FillGitNamespace(rendersetObj.YamlData)
	if err != nil {
		// Note, since user can always reselect the git info, error should not block normal logic
		log.Warnf("failed to fill git namespace data, err: %s", err)
	}
	ret.YamlData = rendersetObj.YamlData
	return ret, nil
}

func GetMergedYamlContent(arg *YamlContentRequestArg, paths []string) (string, error) {
	var (
		fileContentMap sync.Map
		wg             sync.WaitGroup
		err            error
	)
	for i, filePath := range paths {
		wg.Add(1)
		go func(index int, path string) {
			defer wg.Done()
			fileContent, errDownload := fsservice.DownloadFileFromSource(
				&fsservice.DownloadFromSourceArgs{
					CodehostID: arg.CodehostID,
					Owner:      arg.Owner,
					Namespace:  arg.Namespace,
					Repo:       arg.Repo,
					Path:       path,
					Branch:     arg.Branch,
					RepoLink:   arg.RepoLink,
				})
			if errDownload != nil {
				err = errors.Wrapf(errDownload, fmt.Sprintf("failed to download file from git, path %s", path))
				return
			}
			fileContentMap.Store(index, fileContent)
		}(i, filePath)
	}
	wg.Wait()

	if err != nil {
		return "", err
	}

	contentArr := make([][]byte, 0, len(paths))
	for i := 0; i < len(paths); i++ {
		contentObj, _ := fileContentMap.Load(i)
		contentArr = append(contentArr, contentObj.([]byte))
	}
	ret, err := yamlutil.Merge(contentArr)
	if err != nil {
		return "", errors.Wrapf(err, "failed to merge files")
	}
	return string(ret), nil
}
