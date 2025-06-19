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
	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/code/client"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/code/client/open"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/systemconfig"
)

const (
	codeHostGitlab = "gitlab"
	OrgKind        = "org"
	GroupKind      = "group"
	UserKind       = "user"
	EnterpriseKind = "enterprise"
)

func CodeHostListNamespaces(codeHostID int, keyword string, log *zap.SugaredLogger) ([]*client.Namespace, error) {
	ch, err := systemconfig.New().GetCodeHost(codeHostID)
	if err != nil {
		log.Errorf("get code host info err:%s", err)
		return make([]*client.Namespace, 0), nil
	}

	if ch.Type == setting.SourceFromOther {
		return []*client.Namespace{}, nil
	}

	cli, err := open.OpenClient(ch, log)
	if err != nil {
		log.Errorf("open client err:%s", err)
		return make([]*client.Namespace, 0), nil
	}
	ns, err := cli.ListNamespaces(keyword)
	if err != nil {
		log.Errorf("list namespace err:%s", err)
		return make([]*client.Namespace, 0), nil
	}
	return ns, nil
}
