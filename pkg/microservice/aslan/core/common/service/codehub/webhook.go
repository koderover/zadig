package codehub

import (
	"fmt"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	gitservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/git"
	"github.com/koderover/zadig/pkg/tool/codehub"
)

func (c *Client) CreateWebHook(owner, repo string) (string, error) {
	fmt.Println(fmt.Sprintf("owner:%s", owner))
	fmt.Println(fmt.Sprintf("repo:%s", repo))
	return c.AddWebhook(owner, repo, &codehub.AddCodehubHookPayload{
		HookURL:    config.WebHookURL(),
		Token:      gitservice.GetHookSecret(),
		HookEvents: []string{codehub.PushEvent, codehub.BranchOrTagCreateEvent, codehub.PullRequestEvent},
	})
}

func (c *Client) DeleteWebHook(owner, repo, hookID string) error {
	return c.DeleteCodehubWebhook(owner, repo, hookID)
}
