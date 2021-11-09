package policy

import (
	"github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/tool/httpclient"
)

type Client struct {
	*httpclient.Client

	host string
}

func New() *Client {
	host := config.PolicyServiceAddress()

	c := httpclient.New(
		httpclient.SetHostURL(host + "/api/v1"),
	)

	return &Client{
		Client: c,
		host:   host,
	}
}
