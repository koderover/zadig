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
	"fmt"

	"github.com/xanzy/go-gitlab"
)

type Client struct {
	*gitlab.Client
}

func NewGitlabClient(address, accessToken string) (*Client, error) {
	cli, err := gitlab.NewOAuthClient(accessToken, gitlab.WithBaseURL(address))
	if err != nil {
		return nil, fmt.Errorf("set base url failed, err:%v", err)
	}

	client := &Client{
		Client: cli,
	}

	return client, nil
}
