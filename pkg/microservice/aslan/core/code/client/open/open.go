/*
Copyright 2022 The KodeRover Authors.

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

package open

import (
	"encoding/json"
	"fmt"

	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/code/client"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/code/client/gerrit"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/code/client/git"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/code/client/gitee"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/code/client/github"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/code/client/gitlab"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/systemconfig"
)

type ClientConfig interface {
	Open(id int, logger *zap.SugaredLogger) (client.CodeHostClient, error)
}

var ClientsConfig = map[string]func() ClientConfig{
	setting.SourceFromOther:   func() ClientConfig { return new(git.Config) },
	setting.SourceFromGitlab:  func() ClientConfig { return new(gitlab.Config) },
	setting.SourceFromGithub:  func() ClientConfig { return new(github.Config) },
	setting.SourceFromGerrit:  func() ClientConfig { return new(gerrit.Config) },
	setting.SourceFromGitee:   func() ClientConfig { return new(gitee.Config) },
	setting.SourceFromGiteeEE: func() ClientConfig { return new(gitee.EEConfig) },
}

func OpenClient(ch *systemconfig.CodeHost, log *zap.SugaredLogger) (client.CodeHostClient, error) {
	var c client.CodeHostClient
	f, ok := ClientsConfig[ch.Type]
	if !ok {
		log.Error("unknow codehost type")
		return c, fmt.Errorf("unknow codehost type")
	}
	clientConfig := f()
	bs, err := json.Marshal(ch)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(bs, clientConfig); err != nil {
		log.Errorf("marsh err:%s", err)
		return nil, err
	}
	return clientConfig.Open(ch.ID, log)
}
