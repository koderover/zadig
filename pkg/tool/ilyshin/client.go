package ilyshin

import (
	"crypto/tls"

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
		httpclient.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true}),
	)
	return &Client{
		Client:      c,
		Address:     address,
		AccessToken: accessToken,
	}
}
