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
	"strings"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/codehost"
	git "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/github"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/gerrit"
	"github.com/koderover/zadig/pkg/tool/gitlab"
)

func CodehostListProjects(codehostID int, namespace, namespaceType, keyword string, log *zap.SugaredLogger) ([]*gitlab.Project, error) {
	opt := &codehost.Option{
		CodeHostID: codehostID,
	}
	codehost, err := codehost.GetCodeHostInfo(opt)
	if err != nil {
		log.Error(err)
		return nil, e.ErrCodehostListProjects.AddDesc("git client is nil")
	}
	if codehost.Type == codeHostGitlab {
		client, err := gitlab.NewGitlabClient(codehost.Address, codehost.AccessToken)
		if err != nil {
			log.Error(err)
			return nil, e.ErrCodehostListProjects.AddDesc(err.Error())
		}

		if namespaceType == GroupKind {
			projects, err := client.ListGroupProjects(namespace, keyword)
			if err != nil {
				log.Error(err)
				return nil, e.ErrCodehostListProjects.AddDesc(err.Error())
			}
			return projects, nil
		}

		//	user
		projects, err := client.ListUserProjects(namespace, keyword)
		if err != nil {
			log.Error(err)
			return nil, e.ErrCodehostListProjects.AddDesc(err.Error())
		}
		return projects, nil

	} else if codehost.Type == gerrit.CodehostTypeGerrit {
		cli := gerrit.NewClient(codehost.Address, codehost.AccessToken)
		projects, err := cli.ListProjectsByKey(keyword)
		result := make([]*gitlab.Project, 0)
		if err == nil {
			id := 0
			for _, p := range projects {
				result = append(result, &gitlab.Project{
					ID:            id,                    // fake id
					Name:          gerrit.Unescape(p.ID), // id could have %2F
					Description:   p.Description,
					DefaultBranch: "master",
					Namespace:     gerrit.DefaultNamespace,
				})
				id++
			}
		}
		return result, err
	} else {
		//	github
		gitClient := git.NewGithubAppClient(codehost.AccessToken, setting.GitHubAPIServer, config.ProxyHTTPSAddr())
		opt := &github.RepositoryListOptions{
			ListOptions: github.ListOptions{Page: page, PerPage: perPage},
		}

		projects := make([]*gitlab.Project, 0)
		repos, _, err := gitClient.Repositories.List(context.Background(), "", opt)
		if err != nil {
			return nil, err
		}

		for _, repo := range repos {
			if repo.Owner == nil || *repo.Owner.Login != namespace {
				continue
			}
			if keyword != "" && !strings.Contains(repo.GetFullName(), keyword) {
				continue
			}
			projects = append(projects, getProject(repo))
		}
		return projects, nil
	}
}

func getProject(repo *github.Repository) *gitlab.Project {
	project := new(gitlab.Project)
	project.ID = int(*repo.ID)
	project.Name = *repo.Name
	project.DefaultBranch = *repo.DefaultBranch
	project.Namespace = *repo.Owner.Login
	return project
}
