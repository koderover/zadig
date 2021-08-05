package ilyshin

import (
	"github.com/koderover/zadig/pkg/tool/httpclient"
)

type Client struct {
	*httpclient.Client
	Address     string
	AccessToken string
}

func NewClient(address, accessToken string) *Client {
	c := httpclient.New(
		httpclient.SetAuthScheme("Bearer"),
		httpclient.SetAuthToken(accessToken),
		httpclient.SetHostURL(address),
	)

	return &Client{
		Client:      c,
		Address:     address,
		AccessToken: accessToken,
	}
}
