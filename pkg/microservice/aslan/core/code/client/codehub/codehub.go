package codehub

import (
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/code/client"
	"github.com/koderover/zadig/pkg/shared/client/systemconfig"
	"github.com/koderover/zadig/pkg/tool/codehub"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

type Config struct{}

type Client struct {
	Client *codehub.CodeHubClient
}

func (c *Config) Open(id int, logger *zap.SugaredLogger) (client.CodeHostClient, error) {
	ch, err := systemconfig.New().GetCodeHost(id)
	if err != nil {
		return nil, e.ErrCodehostListBranches.AddDesc("git client is nil")
	}
	codehubClient := codehub.NewCodeHubClient(ch.AccessKey, ch.SecretKey, ch.Region, config.ProxyHTTPSAddr(), ch.EnableProxy)
	return &Client{Client: codehubClient}, nil
}

func (c *Client) ListBranches(namespace, projectName, key string, page, perPage int) ([]*client.Branch, error) {
	bList, err := c.Client.BranchList(projectName)
	if err != nil {
		return nil, err
	}
	var res []*client.Branch
	for _, o := range bList {
		res = append(res, &client.Branch{
			Name:      o.Name,
			Protected: o.Protected,
			Merged:    o.Merged,
		})
	}
	return res, nil
}

func (c *Client) ListTags() ([]*client.Tag, error) {
	return nil, nil
}
