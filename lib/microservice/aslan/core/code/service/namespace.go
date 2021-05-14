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
	"github.com/koderover/zadig/lib/setting"
	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/tool/gerrit"
	"github.com/koderover/zadig/lib/tool/gitlab"
	"github.com/koderover/zadig/lib/tool/xlog"
)

func CodehostListNamespaces(codehostId int, keyword string, log *xlog.Logger) ([]*gitlab.Namespace, error) {
	opt := &codehost.CodeHostOption{
		CodeHostID: codehostId,
	}
	codehost, err := codehost.GetCodeHostInfo(opt)
	if err != nil {
		return nil, e.ErrCodehostListNamespaces.AddDesc("git client is nil")
	}

	if codehost.Type == CODEHOSTGITLAB {
		client, err := gitlab.NewGitlabClient(codehost.Address, codehost.AccessToken)
		if err != nil {
			log.Error(err)
			return nil, e.ErrCodehostListNamespaces.AddDesc(err.Error())
		}

		nsList, err := client.ListNamespaces(keyword)
		if err != nil {
			return nil, err
		}
		return nsList, nil
	} else if codehost.Type == gerrit.CodehostTypeGerrit {
		return []*gitlab.Namespace{{
			Name: gerrit.DefaultNamespace,
			Path: gerrit.DefaultNamespace,
			Kind: ORGKIND,
		}}, nil
	} else {
		//	github
		gitClient := git.NewGithubAppClient(codehost.AccessToken, APIServer, config.ProxyHTTPSAddr())
		opt := &github.ListOptions{Page: PAGE, PerPage: PERPAGE}
		organizations, _, err := gitClient.Organizations.List(context.Background(), "", opt)
		if err != nil {
			return nil, err
		}
		namespaces := make([]*gitlab.Namespace, 0)
		for _, organization := range organizations {
			namespace := new(gitlab.Namespace)
			namespace.Name = organization.GetLogin()
			namespace.Path = organization.GetLogin()
			namespace.Kind = ORGKIND

			namespaces = append(namespaces, namespace)
		}
		return namespaces, nil
	}
}

const (
	CODEHOSTGITLAB = "gitlab"
	ORGKIND        = "org"
	GROUPKIND      = "group"
	UESRKIND       = "user"
	PAGE           = 1
	PERPAGE        = 100
	GITHUBPerPage  = 1000
	APIServer      = setting.GitHubAPIServer
)
