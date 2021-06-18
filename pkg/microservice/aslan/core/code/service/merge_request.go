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
	"fmt"

	"github.com/google/go-github/v35/github"
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/codehost"
	git "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/github"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/gerrit"
	"github.com/koderover/zadig/pkg/tool/gitlab"
)

func CodehostListPRs(codehostID int, projectName, namespace, targetBr string, log *zap.SugaredLogger) ([]*gitlab.MergeRequest, error) {
	projectID := fmt.Sprintf("%s/%s", namespace, projectName)
	opt := &codehost.Option{
		CodeHostID: codehostID,
	}
	codehost, err := codehost.GetCodeHostInfo(opt)
	if err != nil {
		return nil, e.ErrCodehostListPrs.AddDesc("git client is nil")
	}

	if codehost.Type == codeHostGitlab {
		client, err := gitlab.NewGitlabClient(codehost.Address, codehost.AccessToken)
		if err != nil {
			log.Error(err)
			return nil, e.ErrCodehostListPrs.AddDesc(err.Error())
		}

		prs, err := client.ListOpened(projectID, targetBr)
		if err != nil {
			log.Error(err)
			return nil, e.ErrCodehostListPrs.AddDesc(err.Error())
		}
		if len(prs) == 0 {
			prs = []*gitlab.MergeRequest{}
		}
		return prs, nil

	} else if codehost.Type == gerrit.CodehostTypeGerrit {
		return make([]*gitlab.MergeRequest, 0), nil
	} else {
		//	github
		gitClient := git.NewGithubAppClient(codehost.AccessToken, setting.GitHubAPIServer, config.ProxyHTTPSAddr())
		opt := &github.PullRequestListOptions{
			ListOptions: github.ListOptions{Page: page, PerPage: perPage},
		}
		pullRequests, _, err := gitClient.PullRequests.List(context.Background(), namespace, projectName, opt)
		if err != nil {
			return nil, err
		}
		mergeRequests := make([]*gitlab.MergeRequest, 0)
		for _, pullRequest := range pullRequests {
			mergeRequest := new(gitlab.MergeRequest)
			mergeRequest.MRID = *pullRequest.Number
			mergeRequest.CreatedAt = pullRequest.CreatedAt.Unix()
			mergeRequest.UpdatedAt = pullRequest.UpdatedAt.Unix()
			mergeRequest.State = *pullRequest.State
			mergeRequest.User = *pullRequest.User.Login
			mergeRequest.Number = *pullRequest.Number
			mergeRequest.AuthorUsername = *pullRequest.User.Login
			mergeRequest.Title = *pullRequest.Title
			mergeRequest.SourceBranch = *pullRequest.Head.Ref
			mergeRequest.TargetBranch = *pullRequest.Base.Ref

			mergeRequests = append(mergeRequests, mergeRequest)
		}
		return mergeRequests, nil
	}
}
