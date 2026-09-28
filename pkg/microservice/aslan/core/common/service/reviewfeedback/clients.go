package reviewfeedback

import (
	"context"
	"net/http"
	"net/url"

	githubapi "github.com/google/go-github/v35/github"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	repo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/shared/client/systemconfig"
	githubtool "github.com/koderover/zadig/v2/pkg/tool/git/github"
	"golang.org/x/oauth2"
)

// Accessed only while workerMu is held. App transports refresh installation
// tokens themselves; sharing clients avoids installation discovery per PR.
type appClientCacheKey struct {
	AppID                int
	Owner, AppKey, Proxy string
}

var appClients = map[appClientCacheKey]*githubapi.Client{}

func newGitHubFeedbackClient(ctx context.Context, pr *models.AIReviewFeedback, host *systemconfig.CodeHost) (*githubapi.Client, error) {
	apps, err := repo.NewGithubAppColl().Find(ctx)
	if err != nil {
		return nil, err
	}
	proxyAddress := config.ProxyHTTPSAddr()
	// Discard cached clients when the configured App or proxy changes.
	for key := range appClients {
		if len(apps) == 0 || key.AppID != apps[0].AppID || key.AppKey != apps[0].AppKey || key.Proxy != proxyAddress {
			delete(appClients, key)
		}
	}
	if len(apps) > 0 {
		app := apps[0]
		cacheKey := appClientCacheKey{AppID: app.AppID, Owner: pr.RepoOwner, AppKey: app.AppKey, Proxy: proxyAddress}
		if cli := appClients[cacheKey]; cli != nil {
			return cli, nil
		}
		cli, err := githubtool.NewAppClientWithContext(ctx, &githubtool.Config{AppKey: app.AppKey, AppID: app.AppID, Owner: pr.RepoOwner, Proxy: proxyAddress}, func(base http.RoundTripper) http.RoundTripper { return base })
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
	for key, cached := range appClients {
		if cached == client {
			delete(appClients, key)
		}
	}
}
