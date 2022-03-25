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
	var (
		client     *gitee.APIClient
		HttpClient *http.Client
	)

	conf := gitee.NewConfiguration()
	dc := http.DefaultClient
	if proxyAddr != "" {
		p, err := url.Parse(proxyAddr)
		if err == nil {
			proxy := http.ProxyURL(p)
			trans := &http.Transport{
				Proxy: proxy,
			}
			dc = &http.Client{Transport: trans}
		}
	}

	if accessToken != "" {
		ctx := context.WithValue(context.Background(), oauth2.HTTPClient, dc)
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: accessToken},
		)
		HttpClient = oauth2.NewClient(ctx, ts)
	} else {
		HttpClient = dc
	}
	conf.HTTPClient = HttpClient
	client = gitee.NewAPIClient(conf)

	return &Client{client}, nil
}
