package config

import (
	"github.com/koderover/zadig/pkg/tool/httpclient"
)

type Client struct {
	*httpclient.Client

	host string
}

func New(host string) *Client {
	c := httpclient.New(
		httpclient.SetHostURL(host),
	)

	return &Client{
		Client: c,
		host:   host,
	}
}
