package ilyshin

import (
	"strconv"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	gitservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/git"
)

func (c *Client) CreateWebHook(owner, repo string) (string, error) {
	projectHook, err := c.AddProjectHook(owner, repo, config.WebHookURL(), gitservice.GetHookSecret())

	return strconv.Itoa(projectHook.ID), err
}

func (c *Client) DeleteWebHook(owner, repo, hookID string) error {
	hookIDInt, err := strconv.Atoi(hookID)
	if err != nil {
		return err
	}
	return c.DeleteProjectHook(owner, repo, hookIDInt)
}
