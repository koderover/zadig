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
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/codehost"
	git "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/github"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/gerrit"
	"github.com/koderover/zadig/pkg/tool/gitlab"
)

const (
	codeHostGitlab = "gitlab"
	OrgKind        = "org"
	GroupKind      = "group"
	UserKind       = "user"
	page           = 1
	perPage        = 100
)

func CodehostListNamespaces(codehostID int, keyword string, log *zap.SugaredLogger) ([]*gitlab.Namespace, error) {
	opt := &codehost.Option{
		CodeHostID: codehostID,
	}
	codehost, err := codehost.GetCodeHostInfo(opt)
	if err != nil {
		return nil, e.ErrCodehostListNamespaces.AddDesc("git client is nil")
	}

	if codehost.Type == codeHostGitlab {
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
			Kind: OrgKind,
		}}, nil
	} else {
		//	github
		gitClient := git.NewClient(codehost.AccessToken, config.ProxyHTTPSAddr())
		namespaces := make([]*gitlab.Namespace, 0)

		// authenticated user
		user, _, err := gitClient.Users.Get(context.Background(), "")
		if err != nil {
			return nil, err
		}
		namespace := &gitlab.Namespace{
			Name: user.GetLogin(),
			Path: user.GetLogin(),
			Kind: UserKind,
		}
		namespaces = append(namespaces, namespace)

		// organizations for the authenticated user
		opt := &github.ListOptions{Page: page, PerPage: perPage}
		organizations, _, err := gitClient.Organizations.List(context.Background(), "", opt)
		if err != nil {
			return nil, err
		}
		for _, organization := range organizations {
			namespace := &gitlab.Namespace{
				Name: organization.GetLogin(),
				Path: organization.GetLogin(),
				Kind: OrgKind,
			}

			namespaces = append(namespaces, namespace)
		}
		return namespaces, nil
	}
}
