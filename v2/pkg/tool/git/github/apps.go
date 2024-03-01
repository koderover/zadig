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

func (c *Client) ListInstallations(ctx context.Context, opts *ListOptions) ([]*github.Installation, error) {
	installations, err := wrap(paginated(func(o *github.ListOptions) ([]interface{}, *github.Response, error) {
		is, r, err := c.Apps.ListInstallations(ctx, o)
		var res []interface{}
		for _, i := range is {
			res = append(res, i)
		}
		return res, r, err
	}, opts))

	if err != nil {
		return nil, err
	}

	var res []*github.Installation
	is, ok := installations.([]interface{})
	if !ok {
		return nil, nil
	}
	for _, i := range is {
		res = append(res, i.(*github.Installation))
	}

	return res, err
}
