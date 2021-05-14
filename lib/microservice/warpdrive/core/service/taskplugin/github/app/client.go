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

package app

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/bradleyfalzon/ghinstallation"
	"github.com/google/go-github/v35/github"
	"github.com/gregjones/httpcache"
)

// Config ...
type Config struct {
	// AppKey is a base64 encoded string
	AppKey string
	AppID  int
}

// Client ...
type Client struct {
	git             *github.Client
	installationIDs sync.Map
}

// NewAppClient ...
func NewAppClient(cfg *Config) (*Client, error) {

	keyBytes, err := base64.StdEncoding.DecodeString(cfg.AppKey)
	if err != nil {
		return nil, err
	}

	trans, err := ghinstallation.NewAppsTransport(httpcache.NewMemoryCacheTransport(), int64(cfg.AppID), keyBytes)
	if err != nil {
		return nil, err
	}

	gitCli := github.NewClient(&http.Client{Transport: trans})

	c := &Client{git: gitCli}

	return c, nil
}

// FindInstallationID ...
func (c *Client) FindInstallationID(key string) (int, error) {

	if key == "" {
		return 0, errors.New("empty key name")
	}

	installID, ok := c.installationIDs.Load(key)
	if ok {
		return installID.(int), nil
	}

	opt := &github.ListOptions{Page: 1, PerPage: 100}

	for opt.Page > 0 {
		installs, resp, err := c.git.Apps.ListInstallations(context.Background(), opt)
		if err != nil {
			return 0, err
		}

		for _, inst := range installs {
			if inst.Account != nil && inst.Account.Login != nil && *inst.Account.Login == key {
				c.installationIDs.Store(key, int(inst.GetID()))
				return int(inst.GetID()), nil
			}
		}
		opt.Page = resp.NextPage
	}

	return 0, fmt.Errorf("%s has no installation", key)
}
