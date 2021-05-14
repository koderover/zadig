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

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/service/codehost"

	git "github.com/koderover/zadig/lib/microservice/aslan/core/common/service/github"
	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/tool/gerrit"
	"github.com/koderover/zadig/lib/tool/gitlab"
	"github.com/koderover/zadig/lib/tool/xlog"
)

func CodehostListPRs(codehostId int, projectName, namespace, targetBr string, log *xlog.Logger) ([]*gitlab.MergeRequest, error) {
	projectId := fmt.Sprintf("%s/%s", namespace, projectName)
	opt := &codehost.CodeHostOption{
		CodeHostID: codehostId,
	}
	codehost, err := codehost.GetCodeHostInfo(opt)
	if err != nil {
		return nil, e.ErrCodehostListPrs.AddDesc("git client is nil")
	}

	if codehost.Type == CODEHOSTGITLAB {
		client, err := gitlab.NewGitlabClient(codehost.Address, codehost.AccessToken)
		if err != nil {
			log.Error(err)
			return nil, e.ErrCodehostListPrs.AddDesc(err.Error())
		}

		prs, err := client.ListOpened(projectId, targetBr)
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
		gitClient := git.NewGithubAppClient(codehost.AccessToken, APIServer, config.ProxyHTTPSAddr())
		opt := &github.PullRequestListOptions{
			ListOptions: github.ListOptions{Page: PAGE, PerPage: PERPAGE},
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
