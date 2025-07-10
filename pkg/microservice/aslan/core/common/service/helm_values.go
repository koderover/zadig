/*
Copyright 2025 The KodeRover Authors.

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
	templatemodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	fsservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/fs"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	yamlutil "github.com/koderover/zadig/v2/pkg/util/yaml"
)

func SyncYamlFromSource(yamlData *templatemodels.CustomYaml, curValue string, originValue string) (bool, string, error) {
	if yamlData == nil || !yamlData.AutoSync {
		return false, "", nil
	}
	if yamlData.Source == setting.SourceFromVariableSet {
		return syncYamlFromVariableSet(yamlData, curValue)
	}
	return syncYamlFromGit(yamlData, curValue, originValue)
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

func syncYamlFromGit(yamlData *templatemodels.CustomYaml, curValue string, originValue string) (bool, string, error) {
	if !fromGitRepo(yamlData.Source) {
		return false, "", nil
	}
	sourceDetail, err := UnMarshalSourceDetail(yamlData.SourceDetail)
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
	equal, err := yamlutil.Equal(string(valuesYAML), originValue)
	if err != nil || equal {
		return false, "", err
	}
	return true, string(valuesYAML), nil
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
