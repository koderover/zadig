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

	"github.com/koderover/zadig/pkg/microservice/aslan/core/code/client"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/code/client/open"
)

const (
	codeHostGitlab  = "gitlab"
	OrgKind         = "org"
	GroupKind       = "group"
	UserKind        = "user"
	CodeHostCodeHub = "codehub"
)

func CodeHostListNamespaces(codeHostID int, keyword string, log *zap.SugaredLogger) ([]*client.Namespace, error) {
	cli, err := open.OpenClient(codeHostID, log)
	if err != nil {
		log.Errorf("open client err:%s", err)
		return nil, err
	}
	ns, err := cli.ListNamespaces(keyword)
	if err != nil {
		log.Errorf("list namespace err:%s", err)
		return nil, err
	}
	return ns, nil
}
