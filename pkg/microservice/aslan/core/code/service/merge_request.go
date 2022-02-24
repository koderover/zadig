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

	"github.com/google/go-github/v35/github"
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	git "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/github"
	"github.com/koderover/zadig/pkg/shared/client/systemconfig"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/gerrit"
	"github.com/koderover/zadig/pkg/tool/git/gitlab"
)

func CodeHostListPRs(codeHostID int, projectName, namespace, targetBr string, log *zap.SugaredLogger) ([]*PullRequest, error) {
	ch, err := systemconfig.New().GetCodeHost(codeHostID)
	if err != nil {
		return nil, e.ErrCodehostListPrs.AddDesc("git client is nil")
	}

	if ch.Type == codeHostGitlab {
		client, err := gitlab.NewClient(ch.Address, ch.AccessToken, config.ProxyHTTPSAddr(), ch.EnableProxy)
		if err != nil {
			log.Error(err)
			return nil, e.ErrCodehostListPrs.AddDesc(err.Error())
		}

		prs, err := client.ListOpenedProjectMergeRequests(namespace, projectName, targetBr, nil)
		if err != nil {
			log.Error(err)
			return nil, e.ErrCodehostListPrs.AddDesc(err.Error())
		}

		return ToPullRequests(prs), nil

	} else if ch.Type == gerrit.CodehostTypeGerrit {
		return nil, nil
	} else if ch.Type == CodeHostCodeHub {
		return nil, nil
	} else {
		//	github
		gh := git.NewClient(ch.AccessToken, config.ProxyHTTPSAddr(), ch.EnableProxy)
		pullRequests, err := gh.ListPullRequests(context.TODO(), namespace, projectName, &github.PullRequestListOptions{
			ListOptions: github.ListOptions{PerPage: 100},
		})
		if err != nil {
			return nil, err
		}
		return ToPullRequests(pullRequests), nil
	}
}
