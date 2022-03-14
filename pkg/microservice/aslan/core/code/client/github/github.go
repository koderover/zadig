package github

import (
	"context"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/code/client"
	"github.com/koderover/zadig/pkg/shared/client/systemconfig"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/git/github"
)

type Config struct {
}

type Client struct {
	Client *github.Client
}

func (c *Config) Open(id int, logger *zap.SugaredLogger) (client.CodeHostClient, error) {
	ch, err := systemconfig.New().GetCodeHost(id)
	if err != nil {
		return nil, e.ErrCodehostListBranches.AddDesc("git client is nil")
	}
	cfg := &github.Config{
		AccessToken: ch.AccessToken,
	}
	if ch.EnableProxy {
		cfg.Proxy = config.ProxyHTTPSAddr()
	}
	return &Client{
		Client: github.NewClient(cfg),
	}, nil
}

func (c *Client) ListBranches(namespace, projectName, key string, page, perPage int) ([]*client.Branch, error) {
	bList, err := c.Client.ListBranches(context.TODO(), namespace, projectName, nil)
	if err != nil {
		return nil, err
	}
	var res []*client.Branch
	for _, o := range bList {
		res = append(res, &client.Branch{
			Name:      o.GetName(),
			Protected: o.GetProtected(),
		})
	}
	return res, nil
}

func (c *Client) ListTags() ([]*client.Tag, error) {
	return nil, nil
}
