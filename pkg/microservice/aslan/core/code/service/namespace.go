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

const (
	codeHostGitlab  = "gitlab"
	OrgKind         = "org"
	GroupKind       = "group"
	UserKind        = "user"
	CodeHostCodeHub = "codehub"
)

func CodeHostListNamespaces(codeHostID int, keyword string, log *zap.SugaredLogger) ([]*Namespace, error) {
	ch, err := systemconfig.New().GetCodeHost(codeHostID)
	if err != nil {
		return nil, e.ErrCodehostListNamespaces.AddDesc("git client is nil")
	}

	if ch.Type == codeHostGitlab {
		client, err := gitlab.NewClient(ch.Address, ch.AccessToken, config.ProxyHTTPSAddr(), ch.EnableProxy)
		if err != nil {
			log.Error(err)
			return nil, e.ErrCodehostListNamespaces.AddDesc(err.Error())
		}

		nsList, err := client.ListNamespaces(keyword, nil)
		if err != nil {
			return nil, err
		}
		return ToNamespaces(nsList), nil
	} else if ch.Type == gerrit.CodehostTypeGerrit {
		return []*Namespace{{
			Name: gerrit.DefaultNamespace,
			Path: gerrit.DefaultNamespace,
			Kind: OrgKind,
		}}, nil
	} else if ch.Type == CodeHostCodeHub {
		codeHubClient := codehub.NewCodeHubClient(ch.AccessKey, ch.SecretKey, ch.Region, config.ProxyHTTPSAddr(), ch.EnableProxy)
		codeHubNamespace, err := codeHubClient.NamespaceList()
		if err != nil {
			return nil, err
		}
		return ToNamespaces(codeHubNamespace), nil
	} else {
		//	github
		gh := git.NewClient(ch.AccessToken, config.ProxyHTTPSAddr(), ch.EnableProxy)
		namespaces := make([]*Namespace, 0)

		user, err := gh.GetAuthenticatedUser(context.TODO())
		if err != nil {
			return nil, err
		}
		namespaces = append(namespaces, ToNamespaces(user)...)

		organizations, err := gh.ListOrganizationsForAuthenticatedUser(context.TODO(), nil)
		if err != nil {
			return nil, err
		}
		namespaces = append(namespaces, ToNamespaces(organizations)...)

		return namespaces, nil
	}
}
