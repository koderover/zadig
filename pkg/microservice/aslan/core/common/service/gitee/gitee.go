package gitee

import (
	"github.com/koderover/zadig/pkg/microservice/aslan/core/code/client/gitee"

	giteeClient "github.com/koderover/zadig/pkg/tool/gitee"
)

func NewClient(accessToken, proxyAddress string, enableProxy bool) gitee.Client {
	client, _ := giteeClient.NewClient(accessToken, proxyAddress, enableProxy)
	return gitee.Client{
		Client:      client,
		AccessToken: accessToken,
	}
}
