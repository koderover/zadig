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

func CodeHostListProjects(codeHostID int, namespace, namespaceType string, page, perPage int, keyword string, log *zap.SugaredLogger) ([]*Project, error) {
	ch, err := systemconfig.New().GetCodeHost(codeHostID)
	if err != nil {
		log.Error(err)
		return nil, e.ErrCodehostListProjects.AddDesc("git client is nil")
	}
	if ch.Type == codeHostGitlab {
		client, err := gitlab.NewClient(ch.Address, ch.AccessToken, config.ProxyHTTPSAddr(), ch.EnableProxy)
		if err != nil {
			log.Error(err)
			return nil, e.ErrCodehostListProjects.AddDesc(err.Error())
		}

		if namespaceType == GroupKind {
			projects, err := client.ListGroupProjects(namespace, keyword, nil)
			if err != nil {
				log.Error(err)
				return nil, e.ErrCodehostListProjects.AddDesc(err.Error())
			}
			return ToProjects(projects), nil
		}

		// user
		projects, err := client.ListUserProjects(namespace, keyword, &gitlab.ListOptions{
			Page:        page,
			PerPage:     perPage,
			NoPaginated: true,
		})
		if err != nil {
			log.Error(err)
			return nil, e.ErrCodehostListProjects.AddDesc(err.Error())
		}
		return ToProjects(projects), nil

	} else if ch.Type == gerrit.CodehostTypeGerrit {
		cli := gerrit.NewClient(ch.Address, ch.AccessToken, config.ProxyHTTPSAddr(), ch.EnableProxy)
		projects, err := cli.ListProjectsByKey(keyword)
		if err != nil {
			log.Error(err)
			return nil, e.ErrCodehostListProjects.AddDesc(err.Error())
		}
		return ToProjects(projects), nil
	} else if ch.Type == CodeHostCodeHub {
		codeHubClient := codehub.NewCodeHubClient(ch.AccessKey, ch.SecretKey, ch.Region, config.ProxyHTTPSAddr(), ch.EnableProxy)
		projects, err := codeHubClient.RepoList(namespace, keyword, 100)
		if err != nil {
			log.Error(err)
			return nil, e.ErrCodehostListProjects.AddDesc(err.Error())
		}
		return ToProjects(projects), nil
	} else {
		//	github
		gh := git.NewClient(ch.AccessToken, config.ProxyHTTPSAddr(), ch.EnableProxy)
		repos, err := gh.ListRepositoriesForAuthenticatedUser(context.TODO(), nil)
		if err != nil {
			return nil, err
		}
		var projects []*Project
		for _, p := range ToProjects(repos) {
			if p.Namespace != namespace {
				continue
			}
			projects = append(projects, p)
		}

		return projects, nil
	}
}
