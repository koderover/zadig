package gitlab

import (
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/code/client"
	"github.com/koderover/zadig/pkg/shared/client/systemconfig"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/git/gitlab"
)

type Config struct{}

type Client struct {
	Client *gitlab.Client
}

func (c *Config) Open(id int, logger *zap.SugaredLogger) (client.CodeHostClient, error) {
	ch, err := systemconfig.New().GetCodeHost(id)
	if err != nil {
		return nil, e.ErrCodehostListBranches.AddDesc("git client is nil")
	}
	client, err := gitlab.NewClient(ch.Address, ch.AccessToken, config.ProxyHTTPSAddr(), ch.EnableProxy)
	if err != nil {
		return nil, err
	}
	return &Client{Client: client}, nil
}

func (c *Client) ListBranches(namespace, projectName, key string, page, perPage int) ([]*client.Branch, error) {
	bList, err := c.Client.ListBranches(namespace, projectName, key, &gitlab.ListOptions{
		Page:        page,
		PerPage:     perPage,
		NoPaginated: true,
	})
	if err != nil {
		return nil, err
	}
	var res []*client.Branch
	for _, b := range bList {
		res = append(res, &client.Branch{
			Name:      b.Name,
			Protected: b.Protected,
			Merged:    b.Merged,
		})
	}
	return res, nil
}

func (c *Client) ListTags() ([]*client.Tag, error) {
	return nil, nil
}
