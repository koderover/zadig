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

	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	"sigs.k8s.io/yaml"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	templatemodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/command"
	fsservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/fs"
	helmservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/helm"
	commontypes "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/types"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/systemconfig"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

type DefaultValuesResp struct {
	DefaultVariable string                     `json:"default_variable"`
	YamlData        *templatemodels.CustomYaml `json:"yaml_data,omitempty"`
}

type YamlContentRequestArg struct {
	CodehostID int    `json:"codehostID" form:"codehostID"`
	Owner      string `json:"owner" form:"owner"`
	Repo       string `json:"repo" form:"repo"`
	Namespace  string `json:"namespace" form:"namespace"`
	Branch     string `json:"branch" form:"branch"`
	RepoLink   string `json:"repoLink" form:"repoLink"`
	ValuesPath string `json:"valuesPath" form:"valuesPath"`
}

func GetDefaultValues(productName, envName string, production bool, log *zap.SugaredLogger) (*DefaultValuesResp, error) {
	ret := &DefaultValuesResp{}

	productInfo, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:       productName,
		EnvName:    envName,
		Production: &production,
	})
	if err == mongo.ErrNoDocuments {
		return ret, nil
	}
	if err != nil {
		log.Errorf("failed to query product info, productName %s envName %s err %s", productName, envName, err)
		return nil, fmt.Errorf("failed to query product info, productName %s envName %s", productName, envName)
	}

	ret.DefaultVariable = productInfo.DefaultValues
	err = service.FillGitNamespace(productInfo.YamlData)
	if err != nil {
		// Note, since user can always reselect the git info, error should not block normal logic
		log.Warnf("failed to fill git namespace data, err: %s", err)
	}
	ret.YamlData = productInfo.YamlData

	return ret, nil
}

func GetMergedYamlContent(arg *YamlContentRequestArg) (string, error) {
	var (
		err error
	)
	detail, err := systemconfig.New().GetCodeHost(arg.CodehostID)
	if err != nil {
		log.Errorf("GetGitRepoInfo GetCodehostDetail err:%s", err)
		return "", e.ErrListRepoDir.AddDesc(err.Error())
	}
	if detail.Type == setting.SourceFromOther {
		err = command.RunGitCmds(detail, arg.Namespace, arg.Namespace, arg.Repo, arg.Branch, "origin")
		if err != nil {
			log.Errorf("GetGitRepoInfo runGitCmds err:%s", err)
			return "", e.ErrListRepoDir.AddDesc(err.Error())
		}
	}

	var fileContent []byte
	isOtherTypeRepo := detail.Type == setting.SourceFromOther
	if !isOtherTypeRepo {
		fileContent, err = fsservice.DownloadFileFromSource(
			&fsservice.DownloadFromSourceArgs{
				CodehostID: arg.CodehostID,
				Owner:      arg.Owner,
				Namespace:  arg.Namespace,
				Repo:       arg.Repo,
				Path:       arg.ValuesPath,
				Branch:     arg.Branch,
				RepoLink:   arg.RepoLink,
			})
		if err != nil {
			err = errors.Wrapf(err, "failed to download file from git, path %s", arg.ValuesPath)
			return "", err
		}
	} else {
		base := path.Join(config.S3StoragePath(), arg.Repo)
		relativePath := path.Join(base, arg.ValuesPath)
		fileContent, err = os.ReadFile(relativePath)
		if err != nil {
			err = errors.Wrapf(err, "failed to read file from git repo, relative path %s", relativePath)
			return "", err
		}
	}

	// check if the file content is a valid yaml
	yamlMap, err := helmservice.GetValuesMapFromString(string(fileContent))
	if err != nil {
		return "", fmt.Errorf("failed to generate yaml map from file content, err: %s", err)
	}

	yamlContent, err := yaml.Marshal(yamlMap)
	if err != nil {
		return "", fmt.Errorf("failed to marshal yaml map to string, err: %s", err)
	}
	return string(yamlContent), nil
}

func GetGlobalVariables(productName, envName string, production bool, log *zap.SugaredLogger) ([]*commontypes.GlobalVariableKV, int64, error) {
	productInfo, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:       productName,
		EnvName:    envName,
		Production: &production,
	})
	if err == mongo.ErrNoDocuments {
		return nil, 0, nil
	}
	if err != nil {
		log.Errorf("failed to query product info, productName %s envName %s err %s", productName, envName, err)
		return nil, 0, fmt.Errorf("failed to query product info, productName %s envName %s", productName, envName)
	}

	return productInfo.GlobalVariables, productInfo.UpdateTime, nil
}
