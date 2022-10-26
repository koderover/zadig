/*
Copyright 2022 The KodeRover Authors.

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

package gitee

import (
	"context"

	"gitee.com/openeuler/go-gitee/gitee"
	"github.com/antihax/optional"
)

func (c *Client) ListOrganizationsForAuthenticatedUser(ctx context.Context) ([]gitee.Group, error) {
	resp, _, err := c.OrganizationsApi.GetV5UserOrgs(ctx, &gitee.GetV5UserOrgsOpts{
		PerPage: optional.NewInt32(100),
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *Client) ListEnterprises(ctx context.Context) ([]gitee.EnterpriseBasic, error) {
	resp, _, err := c.EnterprisesApi.GetV5UserEnterprises(ctx, &gitee.GetV5UserEnterprisesOpts{
		PerPage: optional.NewInt32(100),
		Admin:   optional.NewBool(false),
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}
