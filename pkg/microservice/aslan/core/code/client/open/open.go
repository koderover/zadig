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
	"time"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/code/client"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/code/client/codehub"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/code/client/gerrit"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/code/client/gitee"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/code/client/github"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/code/client/gitlab"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/client/systemconfig"
	giteeClient "github.com/koderover/zadig/pkg/tool/gitee"
)

type ClientConfig interface {
	Open(id int, logger *zap.SugaredLogger) (client.CodeHostClient, error)
}

var ClientsConfig = map[string]func() ClientConfig{
	setting.SourceFromGitlab:  func() ClientConfig { return new(gitlab.Config) },
	setting.SourceFromGithub:  func() ClientConfig { return new(github.Config) },
	setting.SourceFromGerrit:  func() ClientConfig { return new(gerrit.Config) },
	setting.SourceFromCodeHub: func() ClientConfig { return new(codehub.Config) },
	setting.SourceFromGitee:   func() ClientConfig { return new(gitee.Config) },
}

func OpenClient(codehostID int, log *zap.SugaredLogger) (client.CodeHostClient, error) {
	ch, err := systemconfig.New().GetCodeHost(codehostID)
	if err != nil {
		return nil, err
	}

	// The normal expiration time is 86400
	if ch.Type == setting.SourceFromGitee && (time.Now().Unix()-ch.UpdatedAt) >= 86000 {
		accessToken, _ := giteeClient.RefreshAccessToken(ch.RefreshToken)
		if accessToken != nil {
			ch.AccessToken = accessToken.AccessToken
			ch.RefreshToken = accessToken.RefreshToken
			ch.UpdatedAt = int64(accessToken.CreatedAt)
			if err = systemconfig.New().UpdateCodeHost(ch.ID, ch); err != nil {
				return nil, fmt.Errorf("failed to update codehost,err:%s", err)
			}
		}
	}

	var c client.CodeHostClient
	f, ok := ClientsConfig[ch.Type]
	if !ok {
		return c, fmt.Errorf("unknow codehost type")
	}
	clientConfig := f()
	bs, err := json.Marshal(ch)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(bs, clientConfig); err != nil {
		return nil, err
	}
	return clientConfig.Open(codehostID, log)
}
