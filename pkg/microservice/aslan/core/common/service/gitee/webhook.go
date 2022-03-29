package gitee

import (
	"context"
	"strconv"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	gitservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/git"
	"github.com/koderover/zadig/pkg/tool/git"
)

func (c *Client) CreateWebHook(owner, repo string) (string, error) {
	hook, err := c.CreateHook(c.AccessToken, owner, repo, &git.Hook{
		URL:    config.WebHookURL(),
		Secret: gitservice.GetHookSecret(),
	})
	if err != nil {
		return "", err
	}

	return strconv.Itoa(int(hook.ID)), nil
}

func (c *Client) DeleteWebHook(owner, repo, hookID string) error {
	hookIDInt, err := strconv.ParseInt(hookID, 10, 64)
	if err != nil {
		return err
	}
	return c.DeleteHook(context.TODO(), owner, repo, hookIDInt)
}
