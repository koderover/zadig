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

	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	fsservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/fs"
	yamlutil "github.com/koderover/zadig/pkg/util/yaml"
)

type DefaultValuesResp struct {
	DefaultValues string `json:"defaultValues"`
}

type YamlContentRequestArg struct {
	CodehostID  int    `json:"codehostID" form:"codehostID"`
	Owner       string `json:"owner" form:"owner"`
	Repo        string `json:"repo" form:"repo"`
	Branch      string `json:"branch" form:"branch"`
	RepoLink    string `json:"repoLink" form:"repoLink"`
	ValuesPaths string `json:"valuesPaths" form:"valuesPaths"`
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
		Name:     productInfo.Render.Name,
		Revision: productInfo.Render.Revision,
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
