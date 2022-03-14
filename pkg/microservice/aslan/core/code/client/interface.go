package client

import (
	"fmt"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/code/client/codehub"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/code/client/gerrit"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/code/client/github"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/code/client/gitlab"
)

type CodeHostClient interface {
	ListBranches(namespace, projectName, key string, page, perPage int) ([]*Branch, error)
	ListTags() ([]*Tag, error)
}

type Branch struct {
	Name      string `json:"name"`
	Protected bool   `json:"protected"`
	Merged    bool   `json:"merged"`
}

type Tag struct {
	Name       string `json:"name"`
	ZipballURL string `json:"zipball_url"`
	TarballURL string `json:"tarball_url"`
	Message    string `json:"message"`
}

type ClientConfig interface {
	Open(id int, logger *zap.SugaredLogger) (CodeHostClient, error)
}

var ClientsConfig = map[string]func() ClientConfig{
	"gitlab":  func() ClientConfig { return new(gitlab.Config) },
	"github":  func() ClientConfig { return new(github.Config) },
	"gerrit":  func() ClientConfig { return new(gerrit.Config) },
	"codehub": func() ClientConfig { return new(codehub.Config) },
}

func openClient(codehostType string, codehostID int) (CodeHostClient, error) {
	var c CodeHostClient

	f, ok := ClientsConfig[codehostType]
	if !ok {
		return c, fmt.Errorf("unknow codehost type")
	}
	clientConfig := f()
	return clientConfig.Open(codehostID), nil
}
