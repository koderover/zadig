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

func CodehostListBranches(codehostId int, projectName, namespace string, log *xlog.Logger) ([]*gitlab.Branch, error) {
	projectId := fmt.Sprintf("%s/%s", namespace, projectName)

	opt := &codehost.CodeHostOption{
		CodeHostID: codehostId,
	}
	codehost, err := codehost.GetCodeHostInfo(opt)
	if err != nil {
		return nil, e.ErrCodehostListBranches.AddDesc("git client is nil")
	}

	if codehost.Type == CODEHOSTGITLAB {
		client, err := gitlab.NewGitlabClient(codehost.Address, codehost.AccessToken)
		if err != nil {
			log.Errorf("get gitlab client failed, err:%v", err)
			return nil, e.ErrCodehostListBranches.AddDesc(err.Error())
		}

		brList, err := client.ListBranches(projectId)
		if err != nil {
			return nil, err
		}
		return brList, nil
	} else if codehost.Type == gerrit.CodehostTypeGerrit {
		cli := gerrit.NewClient(codehost.Address, codehost.AccessToken)
		result := make([]*gitlab.Branch, 0)
		branches, err := cli.ListBranches(projectName)
		if err == nil {
			for _, branch := range branches {
				result = append(result, &gitlab.Branch{
					Name:      branch,
					Protected: false,
					Merged:    false,
				})
			}
		}

		return result, err
	} else {
		//	github
		gitClient := git.NewGithubAppClient(codehost.AccessToken, APIServer, config.ProxyHTTPSAddr())
		opt := &github.BranchListOptions{ListOptions: github.ListOptions{Page: PAGE, PerPage: PERPAGE}}
		branches, _, err := gitClient.Repositories.ListBranches(context.Background(), namespace, projectName, opt)
		if err != nil {
			return nil, err
		}
		gitBranches := make([]*gitlab.Branch, 0)
		for _, branch := range branches {
			gitBranch := new(gitlab.Branch)
			gitBranch.Name = *branch.Name
			gitBranch.Protected = *branch.Protected

			gitBranches = append(gitBranches, gitBranch)
		}
		return gitBranches, nil
	}
}
