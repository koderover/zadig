package reviewfeedback

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/url"
	"sync"

	githubapi "github.com/google/go-github/v35/github"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	repo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/shared/client/systemconfig"
	githubtool "github.com/koderover/zadig/v2/pkg/tool/git/github"
	"golang.org/x/oauth2"
)

// App transports refresh installation tokens themselves; the cache is shared
// by scheduled collection and review-thread webhooks.
type appClientCacheKey struct {
	CodehostID, AppID    int
	Owner, AppKey, Proxy string
	InsecureSkipVerify   bool
}

var appClientMu sync.Mutex

var appClients = map[appClientCacheKey]*githubapi.Client{}

func newGitHubFeedbackClient(ctx context.Context, pr *models.AIReviewFeedback, host *systemconfig.CodeHost) (*githubapi.Client, error) {
	appClientMu.Lock()
	defer appClientMu.Unlock()
	proxyAddress := config.ProxyHTTPSAddr()
	var appConfig *githubtool.Config
	if bot := host.Bot.GetGitHub(); bot != nil {
		if !host.EnableProxy {
			proxyAddress = ""
		}
		appConfig = &githubtool.Config{AppID: bot.AppID, AppKey: base64.StdEncoding.EncodeToString([]byte(bot.PrivateKey)), Owner: pr.RepoOwner, Proxy: proxyAddress, InsecureSkipVerify: host.DisableSSL, DisableCache: true}
	} else {
		apps, err := repo.NewGithubAppColl().Find(ctx)
		if err != nil {
			return nil, err
		}
		if len(apps) > 0 {
			appConfig = &githubtool.Config{AppID: apps[0].AppID, AppKey: apps[0].AppKey, Owner: pr.RepoOwner, Proxy: proxyAddress, DisableCache: true}
		}
	}
	// Invalidate only this codehost's clients when its credentials or transport settings change.
	for key := range appClients {
		if key.CodehostID == host.ID && (appConfig == nil || key.AppID != appConfig.AppID || key.AppKey != appConfig.AppKey || key.Proxy != appConfig.Proxy || key.InsecureSkipVerify != appConfig.InsecureSkipVerify) {
			delete(appClients, key)
		}
	}
	if appConfig != nil {
		cacheKey := appClientCacheKey{CodehostID: host.ID, AppID: appConfig.AppID, Owner: pr.RepoOwner, AppKey: appConfig.AppKey, Proxy: appConfig.Proxy, InsecureSkipVerify: appConfig.InsecureSkipVerify}
		if cli := appClients[cacheKey]; cli != nil {
			return cli, nil
		}
		cli, err := githubtool.NewAppClientWithContext(ctx, appConfig)
		if err != nil {
			return nil, err
		}
		appClients[cacheKey] = cli.Client
		return cli.Client, nil
	}
	base := http.DefaultTransport.(*http.Transport).Clone()
	if host.EnableProxy {
		proxy, err := url.Parse(config.ProxyHTTPSAddr())
		if err != nil {
			return nil, err
		}
		base.Proxy = http.ProxyURL(proxy)
	}
	transport := &oauth2.Transport{Source: oauth2.StaticTokenSource(&oauth2.Token{AccessToken: host.AccessToken}), Base: base}
	return githubapi.NewClient(&http.Client{Transport: transport}), nil
}

// The next scheduled attempt rediscovers the installation after a request fails.
func discardGitHubFeedbackClient(client *githubapi.Client) {
	appClientMu.Lock()
	defer appClientMu.Unlock()
	for key, cached := range appClients {
		if cached == client {
			delete(appClients, key)
		}
	}
}
