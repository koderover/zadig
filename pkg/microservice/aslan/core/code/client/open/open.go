package open

import (
	"encoding/json"
	"fmt"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/code/client"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/code/client/codehub"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/code/client/gerrit"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/code/client/github"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/code/client/gitlab"
	"github.com/koderover/zadig/pkg/shared/client/systemconfig"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

type ClientConfig interface {
	Open(id int, logger *zap.SugaredLogger) (client.CodeHostClient, error)
}

var ClientsConfig = map[string]func() ClientConfig{
	"gitlab":  func() ClientConfig { return new(gitlab.Config) },
	"github":  func() ClientConfig { return new(github.Config) },
	"gerrit":  func() ClientConfig { return new(gerrit.Config) },
	"codehub": func() ClientConfig { return new(codehub.Config) },
}

func OpenClient(codehostID int, log *zap.SugaredLogger) (client.CodeHostClient, error) {
	ch, err := systemconfig.New().GetCodeHost(codehostID)
	if err != nil {
		return nil, e.ErrCodehostListBranches.AddDesc("git client is nil")
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
