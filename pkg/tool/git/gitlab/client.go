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

package gitlab

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/xanzy/go-gitlab"

	"github.com/koderover/zadig/v2/pkg/tool/httpclient"
)

// TODO: LOU: unify the github/gitlab helpers

type listFunc func(options *gitlab.ListOptions) ([]interface{}, *gitlab.Response, error)

type ListOptions struct {
	// Page number of the results to fetch. Default: 1
	Page int
	// Results per page (max 100). Default: 30
	PerPage int

	// NoPaginated indicates if we need to fetch all result or just one page. True means fetching just one page
	NoPaginated bool

	MatchBranches bool
}

type Client struct {
	*gitlab.Client
}

func NewClient(id int, address, accessToken, proxyAddr string, enableProxy bool, skipTLS bool) (*Client, error) {
	var client *http.Client
	if enableProxy {
		proxyURL, err := url.Parse(proxyAddr)
		if err != nil {
			return nil, err
		}
		transport := &http.Transport{Proxy: http.ProxyURL(proxyURL), TLSClientConfig: &tls.Config{InsecureSkipVerify: skipTLS}}
		client = &http.Client{Transport: transport}
	} else {
		client = http.DefaultClient
		client.Transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: skipTLS}}
	}

	fmt.Println("============= generating client with insecureskipverify set to: %v", skipTLS)

	token, err := UpdateGitlabToken(id, accessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh gitlab token, err: %s", err)
	}

	cli, err := gitlab.NewOAuthClient(token, gitlab.WithBaseURL(address), gitlab.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("failed to create gitlab client, err: %s", err)
	}

	return &Client{Client: cli}, nil
}

func generateProjectName(owner, repo string) string {
	return fmt.Sprintf("%s/%s", owner, repo)
}

func paginated(f listFunc, opts *ListOptions) ([]interface{}, *gitlab.Response, error) {
	if opts == nil {
		opts = &ListOptions{}
	}
	if opts.Page == 0 {
		opts.Page = 1
	}
	if opts.PerPage == 0 {
		opts.PerPage = 100
	}
	ghOpts := &gitlab.ListOptions{Page: opts.Page, PerPage: opts.PerPage}

	if opts.NoPaginated {
		return f(ghOpts)
	}

	var all, result []interface{}
	var response *gitlab.Response
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

func wrap(obj interface{}, res *gitlab.Response, err error) (interface{}, error) {
	return obj, wrapError(res, err)
}

func wrapError(res *gitlab.Response, err error) error {
	if res == nil && err != nil {
		return err
	}

	if res.StatusCode > 399 {
		body, _ := io.ReadAll(res.Body)
		return httpclient.NewGenericServerResponse(res.StatusCode, res.Request.Method, string(body))
	}
	return nil
}
