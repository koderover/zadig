package gerrit

import (
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/code/client"
	"github.com/koderover/zadig/pkg/shared/client/systemconfig"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/gerrit"
)

type Config struct{}

type Client struct {
	Client *gerrit.Client
}

func (c *Config) Open(id int, logger *zap.SugaredLogger) (client.CodeHostClient, error) {
	ch, err := systemconfig.New().GetCodeHost(id)
	if err != nil {
		return nil, e.ErrCodehostListBranches.AddDesc("git client is nil")
	}
	gerritClient := gerrit.NewClient(ch.Address, ch.AccessToken, config.ProxyHTTPSAddr(), ch.EnableProxy)
	return &Client{Client: gerritClient}, nil
}

func (c *Client) ListBranches(namespace, projectName, key string, page, perPage int) ([]*client.Branch, error) {
	bList, err := c.Client.ListBranches(projectName)
	if err != nil {
		return nil, err
	}
	var res []*client.Branch
	for _, o := range bList {
		res = append(res, &client.Branch{
			Name: o,
		})
	}
	return res, nil
}

func (c *Client) ListTags() ([]*client.Tag, error) {
	return nil, nil
}
