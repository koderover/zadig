/*
Copyright 2021 The KodeRover Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package github

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/bradleyfalzon/ghinstallation"
	"github.com/google/go-github/v35/github"
	"github.com/gregjones/httpcache"
	"golang.org/x/oauth2"

	"github.com/koderover/zadig/pkg/tool/httpclient"
)

type listFunc func(options *github.ListOptions) ([]interface{}, *github.Response, error)

type Config struct {
	BaseURL     string
	AccessToken string
	Proxy       string
	AppKey      string
	AppID       int
	Owner       string

	HTTPClient *http.Client
}

type ListOptions struct {
	// Page number of the results to fetch. Default: 1
	Page int
	// Results per page (max 100). Default: 30
	PerPage int

	// NoPaginated indicates if we need to fetch all result or just one page. True means fetching just one page
	NoPaginated bool
}

type Client struct {
	*github.Client
}

func NewClient(cfg *Config) *Client {
	if cfg == nil {
		return &Client{Client: github.NewClient(nil)}
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		dc := http.DefaultClient
		if cfg.Proxy != "" {
			p, err := url.Parse(cfg.Proxy)
			if err == nil {
				proxy := http.ProxyURL(p)
				trans := &http.Transport{
					Proxy: proxy,
				}
				dc = &http.Client{Transport: trans}
			}
		}

		if cfg.AccessToken != "" {
			ctx := context.WithValue(context.Background(), oauth2.HTTPClient, dc)
			ts := oauth2.StaticTokenSource(
				&oauth2.Token{AccessToken: cfg.AccessToken},
			)
			httpClient = oauth2.NewClient(ctx, ts)
		} else {
			httpClient = dc
		}

	}

	gc := github.NewClient(httpClient)

	if cfg.BaseURL != "" {
		u, _ := url.Parse(cfg.BaseURL)
		gc.BaseURL = u
	}

	return &Client{Client: gc}
}

// NewAppClient inits GitHub app client according to user's installation ID
func NewAppClient(cfg *Config) (*Client, error) {
	keyBytes, err := base64.StdEncoding.DecodeString(cfg.AppKey)
	if err != nil {
		return nil, err
	}

	httpTransport := httpcache.NewMemoryCacheTransport()
	tr := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: -1,
		}).DialContext,
		DisableKeepAlives:     true,
		MaxIdleConns:          10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	if len(cfg.Proxy) != 0 {
		p, err := url.Parse(cfg.Proxy)
		if err == nil {
			proxy := http.ProxyURL(p)
			tr.Proxy = proxy
		}
	}

	installationID, err := getInstallationID(int64(cfg.AppID), cfg.AppKey, cfg.Owner)
	if err != nil {
		return nil, err
	}

	httpTransport.Transport = tr
	itr, err := ghinstallation.New(httpTransport, int64(cfg.AppID), installationID, keyBytes)
	if err != nil {
		return nil, err
	}

	// token, _ := itr.Token(context.TODO())
	// c := &Client{Client: gc, InstallationToken: token}
	c := NewClient(&Config{HTTPClient: &http.Client{Transport: itr}})

	return c, nil
}

func getInstallationID(appID int64, appKey, owner string) (int64, error) {
	keyBytes, err := base64.StdEncoding.DecodeString(appKey)
	if err != nil {
		return 0, err
	}

	trans, err := ghinstallation.NewAppsTransport(httpcache.NewMemoryCacheTransport(), appID, keyBytes)
	if err != nil {
		return 0, err
	}

	gc := NewClient(&Config{HTTPClient: &http.Client{Transport: trans}})
	installs, err := gc.ListInstallations(context.TODO(), nil)
	if err != nil {
		return 0, err
	}

	for _, inst := range installs {
		if inst.Account != nil && inst.Account.Login != nil && *inst.Account.Login == owner {
			return inst.GetID(), nil
		}
	}

	return 0, fmt.Errorf("%s has no installation", owner)
}

func paginated(f listFunc, opts *ListOptions) ([]interface{}, *github.Response, error) {
	if opts == nil {
		opts = &ListOptions{}
	}
	if opts.Page == 0 {
		opts.Page = 1
	}
	if opts.PerPage == 0 {
		opts.PerPage = 100
	}
	ghOpts := &github.ListOptions{Page: opts.Page, PerPage: opts.PerPage}

	if opts.NoPaginated {
		return f(ghOpts)
	}

	var all, result []interface{}
	var response *github.Response
	var err error

	for ghOpts.Page > 0 {
		result, response, err = f(ghOpts)
		err = wrapError(response, err)
		if err != nil {
			return nil, response, err
		}

		all = append(all, result...)
		ghOpts.Page = response.NextPage
	}

	return all, response, err
}

func wrap(obj interface{}, res *github.Response, err error) (interface{}, error) {
	return obj, wrapError(res, err)
}

func wrapError(res *github.Response, err error) error {
	if err != nil {
		return err
	}

	if res.StatusCode > 399 {
		body, _ := io.ReadAll(res.Body)
		return httpclient.NewGenericServerResponse(res.StatusCode, res.Request.Method, string(body))
	}

	return nil
}
