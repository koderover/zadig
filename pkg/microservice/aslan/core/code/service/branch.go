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
	"context"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	git "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/github"
	"github.com/koderover/zadig/pkg/shared/client/systemconfig"
	"github.com/koderover/zadig/pkg/tool/codehub"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/gerrit"
	"github.com/koderover/zadig/pkg/tool/git/gitlab"
)

func CodeHostListBranches(codeHostID int, projectName, namespace, key string, page, perPage int, log *zap.SugaredLogger) ([]*Branch, error) {
	ch, err := systemconfig.New().GetCodeHost(codeHostID)
	if err != nil {
		return nil, e.ErrCodehostListBranches.AddDesc("git client is nil")
	}

	if ch.Type == codeHostGitlab {
		client, err := gitlab.NewClient(ch.Address, ch.AccessToken, config.ProxyHTTPSAddr(), ch.EnableProxy)
		if err != nil {
			log.Errorf("get gitlab client failed, err:%v", err)
			return nil, e.ErrCodehostListBranches.AddDesc(err.Error())
		}

		brList, err := client.ListBranches(namespace, projectName, key, &gitlab.ListOptions{
			Page:        page,
			PerPage:     perPage,
			NoPaginated: true,
		})
		if err != nil {
			return nil, err
		}
		return ToBranches(brList), nil
	} else if ch.Type == gerrit.CodehostTypeGerrit {
		cli := gerrit.NewClient(ch.Address, ch.AccessToken, config.ProxyHTTPSAddr(), ch.EnableProxy)
		branches, err := cli.ListBranches(projectName)
		if err != nil {
			return nil, err
		}
		return ToBranches(branches), nil
	} else if ch.Type == CodeHostCodeHub {
		codeHubClient := codehub.NewCodeHubClient(ch.AccessKey, ch.SecretKey, ch.Region, config.ProxyHTTPSAddr(), ch.EnableProxy)
		branchList, err := codeHubClient.BranchList(projectName)
		if err != nil {
			return nil, err
		}
		return ToBranches(branchList), nil
	} else {
		//	github
		gh := git.NewClient(ch.AccessToken, config.ProxyHTTPSAddr(), ch.EnableProxy)
		branches, err := gh.ListBranches(context.TODO(), namespace, projectName, nil)
		if err != nil {
			return nil, err
		}
		return ToBranches(branches), nil
	}
}
