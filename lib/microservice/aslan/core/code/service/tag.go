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

func CodehostListTags(codehostId int, projectName string, namespace string, log *xlog.Logger) ([]*gitlab.Tag, error) {
	projectId := fmt.Sprintf("%s/%s", namespace, projectName)
	opt := &codehost.CodeHostOption{
		CodeHostID: codehostId,
	}
	codehost, err := codehost.GetCodeHostInfo(opt)
	if err != nil {
		log.Error(err)
		return nil, e.ErrCodehostListTags.AddDesc("git client is nil")
	}

	if codehost.Type == CODEHOSTGITLAB {
		client, err := gitlab.NewGitlabClient(codehost.Address, codehost.AccessToken)
		if err != nil {
			log.Error(err)
			return nil, e.ErrCodehostListTags.AddDesc(err.Error())
		}

		brList, err := client.ListTags(projectId)
		if err != nil {
			return nil, err
		}
		return brList, nil
	} else if codehost.Type == gerrit.CodehostTypeGerrit {
		client := gerrit.NewClient(codehost.Address, codehost.AccessToken)
		result := make([]*gitlab.Tag, 0)
		tags, err := client.ListTags(projectName)
		if err != nil {
			return result, nil
		}

		for _, tag := range tags {
			result = append(result, &gitlab.Tag{
				Name:    tag.Ref,
				Message: tag.Message,
			})
		}

		return result, nil
	} else {
		//	github
		gitClient := git.NewGithubAppClient(codehost.AccessToken, APIServer, config.ProxyHTTPSAddr())
		opt := &github.ListOptions{Page: PAGE, PerPage: PERPAGE}
		tags, _, err := gitClient.Repositories.ListTags(context.Background(), namespace, projectName, opt)
		if err != nil {
			return nil, err
		}
		gitlabTags := make([]*gitlab.Tag, 0)
		for _, tag := range tags {
			gitTag := new(gitlab.Tag)
			gitTag.Name = *tag.Name
			gitTag.ZipballURL = *tag.ZipballURL
			gitTag.TarballURL = *tag.TarballURL

			gitlabTags = append(gitlabTags, gitTag)
		}
		return gitlabTags, nil
	}
}
