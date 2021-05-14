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
	"encoding/base64"
	"fmt"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	commonrepo "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	e "github.com/koderover/zadig/lib/tool/errors"
)

func GithubAppList() ([]*commonmodels.GithubApp, error) {
	githubAPPs, err := commonrepo.NewGithubAppColl().Find()
	if err != nil {
		return nil, err
	}
	for _, githubAPP := range githubAPPs {
		githubAPP.EnableGitCheck = config.EnableGitCheck()
	}
	return githubAPPs, nil
}

func GithubAppAdd(args *commonmodels.GithubApp) error {
	_, err := base64.StdEncoding.DecodeString(args.AppKey)
	if err != nil {
		return e.ErrGithubWebHook.AddErr(fmt.Errorf("appKey base64 decode failed"))
	}

	appList, _ := GithubAppList()
	if len(appList) == 0 {
		return commonrepo.NewGithubAppColl().Create(args)
	}
	args.ID = appList[0].ID
	return commonrepo.NewGithubAppColl().Upsert(args)
}

func GithubAppDelete(id string) error {
	return commonrepo.NewGithubAppColl().Delete(id)
}
