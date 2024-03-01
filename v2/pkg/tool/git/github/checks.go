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

	"github.com/google/go-github/v35/github"
)

func (c *Client) CreateCheckRun(ctx context.Context, owner, repo string, opts github.CreateCheckRunOptions) (*github.CheckRun, error) {
	created, err := wrap(c.Checks.CreateCheckRun(ctx, owner, repo, opts))
	if s, ok := created.(*github.CheckRun); ok {
		return s, err
	}

	return nil, err
}

func (c *Client) UpdateCheckRun(ctx context.Context, owner, repo string, checkRunID int64, opts github.UpdateCheckRunOptions) (*github.CheckRun, error) {
	updated, err := wrap(c.Checks.UpdateCheckRun(ctx, owner, repo, checkRunID, opts))
	if s, ok := updated.(*github.CheckRun); ok {
		return s, err
	}

	return nil, err
}
