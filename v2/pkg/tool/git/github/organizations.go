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

func (c *Client) ListOrganizationsForAuthenticatedUser(ctx context.Context, opts *ListOptions) ([]*github.Organization, error) {
	organizations, err := wrap(paginated(func(o *github.ListOptions) ([]interface{}, *github.Response, error) {
		os, r, err := c.Organizations.List(ctx, "", o)
		var res []interface{}
		for _, o := range os {
			res = append(res, o)
		}
		return res, r, err
	}, opts))

	if err != nil {
		return nil, err
	}

	var res []*github.Organization
	os, ok := organizations.([]interface{})
	if !ok {
		return nil, nil
	}
	for _, o := range os {
		res = append(res, o.(*github.Organization))
	}

	return res, err
}
