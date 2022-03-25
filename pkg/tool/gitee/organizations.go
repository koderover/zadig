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
