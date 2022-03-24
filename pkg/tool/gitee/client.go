package gitee

import (
	"context"
	"net/http"
	"net/url"

	"gitee.com/openeuler/go-gitee/gitee"
	"golang.org/x/oauth2"
)

type Client struct {
	*gitee.APIClient
}

func NewClient(accessToken, proxyAddr string, enableProxy bool) (*Client, error) {
	var client *gitee.APIClient

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: accessToken},
	)

	conf := gitee.NewConfiguration()
	conf.HTTPClient = oauth2.NewClient(context.Background(), ts)

	if enableProxy {
		proxyURL, err := url.Parse(proxyAddr)
		if err != nil {
			return nil, err
		}
		transport := &http.Transport{Proxy: http.ProxyURL(proxyURL)}
		conf.HTTPClient = &http.Client{
			Transport: &oauth2.Transport{
				Base:   transport,
				Source: oauth2.ReuseTokenSource(nil, ts),
			},
		}
	}

	client = gitee.NewAPIClient(conf)

	return &Client{client}, nil
}
