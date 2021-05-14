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

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/service/codehost"
	git "github.com/koderover/zadig/lib/microservice/aslan/core/common/service/github"
	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/tool/gerrit"
	"github.com/koderover/zadig/lib/tool/gitlab"
	"github.com/koderover/zadig/lib/tool/xlog"
)

func CodehostListProjects(codehostId int, namespace, namespaceType, keyword string, log *xlog.Logger) ([]*gitlab.Project, error) {
	opt := &codehost.CodeHostOption{
		CodeHostID: codehostId,
	}
	codehost, err := codehost.GetCodeHostInfo(opt)
	if err != nil {
		log.Error(err)
		return nil, e.ErrCodehostListProjects.AddDesc("git client is nil")
	}
	if codehost.Type == CODEHOSTGITLAB {
		client, err := gitlab.NewGitlabClient(codehost.Address, codehost.AccessToken)
		if err != nil {
			log.Error(err)
			return nil, e.ErrCodehostListProjects.AddDesc(err.Error())
		}

		if namespaceType == GROUPKIND {
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
				id += 1
			}
		}
		return result, err
	} else {
		//	github
		gitClient := git.NewGithubAppClient(codehost.AccessToken, APIServer, config.ProxyHTTPSAddr())
		opt := &github.RepositoryListByOrgOptions{
			ListOptions: github.ListOptions{Page: PAGE, PerPage: GITHUBPerPage},
		}
		projects := make([]*gitlab.Project, 0)
		if namespaceType == ORGKIND {
			repos, _, err := gitClient.Repositories.ListByOrg(context.Background(), namespace, opt)
			if err != nil {
				return nil, err
			}

			for _, repo := range repos {
				projects = append(projects, getProject(repo))
			}
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
