package open

import (
	"fmt"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/code/client"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/code/client/codehub"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/code/client/gerrit"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/code/client/github"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/code/client/gitlab"
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

func OpenClient(codehostType string, codehostID int, log *zap.SugaredLogger) (client.CodeHostClient, error) {
	var c client.CodeHostClient

	f, ok := ClientsConfig[codehostType]
	if !ok {
		return c, fmt.Errorf("unknow codehost type")
	}
	clientConfig := f()
	return clientConfig.Open(codehostID, log)
}
