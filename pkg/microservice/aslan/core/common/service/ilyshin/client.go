package ilyshin

import (
	"github.com/koderover/zadig/pkg/tool/ilyshin"
)

type Client struct {
	*ilyshin.Client
}

func NewClient(address, accessToken string) *Client {
	c := ilyshin.NewClient(address, accessToken)
	return &Client{
		Client: c,
	}
}
