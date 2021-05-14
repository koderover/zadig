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
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/bradleyfalzon/ghinstallation"
	"github.com/google/go-github/v35/github"
	"github.com/gregjones/httpcache"
	"golang.org/x/oauth2"
)

// Config ...
type Config struct {
	APIServer string
	ProxyAddr string
	NoCache   bool

	AccessToken string
	HookURL     string
	HookSecret  string

	AppKey         string
	AppID          int
	InstallationID int
}

// Client ...
type Client struct {
	Git               *github.Client
	InstallationToken string
	//Repo              *RepoService
	Checks *CheckService
}

// NewGithubClient constructor
func NewGithubClient(cfg *Config) *Client {

	httpClient := http.DefaultClient

	if len(cfg.ProxyAddr) != 0 {
		p, err := url.Parse(cfg.ProxyAddr)
		if err == nil {
			proxy := http.ProxyURL(p)
			trans := &http.Transport{
				Proxy: proxy,
			}
			httpClient = &http.Client{Transport: trans}
			fmt.Printf("github api is using proxy: %s\n", cfg.ProxyAddr)
		}
	}

	ctx := context.Background()
	ctx = context.WithValue(
		ctx,
		oauth2.HTTPClient,
		httpClient,
	)

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: cfg.AccessToken},
	)

	tc := oauth2.NewClient(ctx, ts)

	var gitCli *github.Client
	if cfg.NoCache {
		gitCli = github.NewClient(tc)
	} else {
		httpTransport := httpcache.NewMemoryCacheTransport()
		httpTransport.Transport = tc.Transport
		gitCli = github.NewClient(&http.Client{Transport: httpTransport})
	}

	if cfg.APIServer != "" {
		u, _ := url.Parse(cfg.APIServer)
		gitCli.BaseURL = u
	}

	c := &Client{Git: gitCli}

	//c.Repo = &RepoService{client: c}
	c.Checks = &CheckService{client: c}

	return c
}

// NewDynamicClient init git client & logger according to user's installation ID
func NewDynamicClient(cfg *Config) (*Client, error) {
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

	if len(cfg.ProxyAddr) != 0 {
		p, err := url.Parse(cfg.ProxyAddr)
		if err == nil {
			proxy := http.ProxyURL(p)
			tr.Proxy = proxy
			fmt.Printf("github api is using proxy: %s\n", cfg.ProxyAddr)
		}
	}

	httpTransport.Transport = tr
	itr, err := ghinstallation.New(httpTransport, int64(cfg.AppID), int64(cfg.InstallationID), keyBytes)
	if err != nil {
		return nil, err
	}

	token, _ := itr.Token(context.Background())

	gitCli := github.NewClient(&http.Client{Transport: itr})

	c := &Client{Git: gitCli, InstallationToken: token}

	//c.Repo = &RepoService{client: c}
	c.Checks = &CheckService{client: c}

	return c, nil
}

func NewGithubAppClient(accessToken, apiServer, proxyAddress string) *github.Client {
	gitCfg := &Config{
		AccessToken: accessToken,
		APIServer:   apiServer,
		ProxyAddr:   proxyAddress,
	}

	client := NewGithubClient(gitCfg)
	return client.Git
}
