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
	"github.com/koderover/zadig/pkg/shared/codehost"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/gerrit"
	"github.com/koderover/zadig/pkg/tool/git/gitlab"
)

func CodeHostListTags(codeHostID int, projectName string, namespace string, log *zap.SugaredLogger) ([]*Tag, error) {
	opt := &codehost.Option{
		CodeHostID: codeHostID,
	}
	ch, err := codehost.GetCodeHostInfo(opt)
	if err != nil {
		log.Error(err)
		return nil, e.ErrCodehostListTags.AddDesc("git client is nil")
	}

	if ch.Type == codeHostGitlab {
		client, err := gitlab.NewClient(ch.Address, ch.AccessToken)
		if err != nil {
			log.Error(err)
			return nil, e.ErrCodehostListTags.AddDesc(err.Error())
		}

		brList, err := client.ListTags(namespace, projectName, nil)
		if err != nil {
			return nil, err
		}
		return ToTags(brList), nil
	} else if ch.Type == gerrit.CodehostTypeGerrit {
		client := gerrit.NewClient(ch.Address, ch.AccessToken)
		tags, err := client.ListTags(projectName)
		if err != nil {
			return nil, err
		}

		return ToTags(tags), nil
	} else {
		//	github
		gh := git.NewClient(ch.AccessToken, config.ProxyHTTPSAddr())
		tags, err := gh.ListTags(context.TODO(), namespace, projectName, nil)
		if err != nil {
			return nil, err
		}

		return ToTags(tags), nil
	}
}
