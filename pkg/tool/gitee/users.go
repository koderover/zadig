package gitee

import (
	"context"

	"gitee.com/openeuler/go-gitee/gitee"
)

func (c *Client) GetAuthenticatedUser(ctx context.Context) (gitee.User, error) {
	ur, _, err := c.UsersApi.GetV5User(ctx, &gitee.GetV5UserOpts{})
	if err != nil {
		return gitee.User{}, err
	}

	return ur, err
}
