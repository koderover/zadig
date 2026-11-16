package rotation

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	githubapi "github.com/google/go-github/v35/github"
	common "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	codehost "github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/codehost/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/git"
	githubtool "github.com/koderover/zadig/v2/pkg/tool/git/github"
	gitlabtool "github.com/koderover/zadig/v2/pkg/tool/git/gitlab"
	giteetool "github.com/koderover/zadig/v2/pkg/tool/gitee"
	"github.com/koderover/zadig/v2/pkg/tool/httpclient"
	gitlabapi "github.com/xanzy/go-gitlab"
)

const webhookRequestTimeout = 30 * time.Second

func prepareHookForHosts(ctx context.Context, hook *common.WebHook, hosts []*codehost.CodeHost, proxy string, allowOrphans bool) (*hookAction, error) {
	var failures []string
	for _, host := range hosts {
		if strings.TrimRight(host.Address, "/") != strings.TrimRight(hook.Address, "/") {
			continue
		}
		action, err := prepareHook(ctx, hook, host, proxy)
		if err == nil {
			return action, nil
		}
		failures = append(failures, err.Error())
	}
	if len(failures) == 0 {
		if allowOrphans {
			return nil, nil
		}
		return nil, fmt.Errorf("webhook %s/%s hook %s: no matching code host; use --allow-orphan-webhooks only after arranging manual rotation", hook.Owner, hook.Repo, hook.HookID)
	}
	return nil, fmt.Errorf("webhook %s/%s hook %s: no code host can read hook: %s", hook.Owner, hook.Repo, hook.HookID, strings.Join(failures, "; "))
}

type hookAction struct {
	hook    *common.WebHook
	host    *codehost.CodeHost
	refresh func(ctx context.Context, secret, callbackURL string) error
}

func (a *hookAction) failure(operation string, err error) error {
	// Never print SDK response bodies: they may contain credentials or the new secret.
	reason := "request failed; verify connectivity and token permissions"
	var gh *githubapi.ErrorResponse
	var gl *gitlabapi.ErrorResponse
	var response *httpclient.Error
	switch {
	case errors.Is(err, context.Canceled):
		reason = "request canceled"
	case errors.Is(err, context.DeadlineExceeded):
		reason = "request timed out"
	case errors.As(err, &gh) && gh.Response != nil:
		reason = fmt.Sprintf("HTTP %d", gh.Response.StatusCode)
	case errors.As(err, &gl) && gl.Response != nil:
		reason = fmt.Sprintf("HTTP %d", gl.Response.StatusCode)
	case errors.As(err, &response):
		reason = fmt.Sprintf("HTTP %d", response.Code)
	}
	return fmt.Errorf("webhook %s/%s, code host %d(%s), hook %s: %s: %s", a.hook.Owner, a.hook.Repo, a.host.ID, a.host.Alias, a.hook.HookID, operation, reason)
}

func prepareHook(ctx context.Context, hook *common.WebHook, host *codehost.CodeHost, proxy string) (*hookAction, error) {
	a := &hookAction{hook: hook, host: host}
	if host.AccessToken == "" {
		return nil, fmt.Errorf("code host %d(%s) has no access token", host.ID, host.Alias)
	}
	if host.EnableProxy {
		p, err := url.Parse(proxy)
		if err != nil || p.Scheme == "" || p.Host == "" {
			return nil, fmt.Errorf("code host %d(%s): enabled proxy has no valid URL", host.ID, host.Alias)
		}
	}
	id, err := strconv.ParseInt(hook.HookID, 10, 64)
	if err != nil || id <= 0 {
		return nil, fmt.Errorf("webhook %s/%s, code host %d(%s): invalid hook ID", hook.Owner, hook.Repo, host.ID, host.Alias)
	}
	ctx, cancel := context.WithTimeout(ctx, webhookRequestTimeout)
	defer cancel()
	// Static clients never refresh OAuth tokens or write codehost state during dry-run.
	switch host.Type {
	case setting.SourceFromGithub:
		cfg := &githubtool.Config{AccessToken: host.AccessToken}
		if host.EnableProxy {
			cfg.Proxy = proxy
		}
		c := githubtool.NewClient(cfg)
		_, _, err = c.Repositories.GetHook(ctx, hook.Owner, hook.Repo, id)
		a.refresh = func(ctx context.Context, secret, callbackURL string) error {
			_, err := c.UpdateHook(ctx, hook.Owner, hook.Repo, id, &git.Hook{URL: callbackURL, Secret: secret})
			return err
		}
	case setting.SourceFromGitlab:
		c, clientErr := gitlabtool.NewOAuthTokenClient(host.Address, host.AccessToken, proxy, host.EnableProxy, host.DisableSSL)
		if clientErr != nil {
			return nil, a.failure("create client", clientErr)
		}
		_, _, err = c.Projects.GetProjectHook(hook.Owner+"/"+hook.Repo, int(id), gitlabapi.WithContext(ctx))
		a.refresh = func(ctx context.Context, secret, callbackURL string) error {
			_, _, err := c.Projects.EditProjectHook(hook.Owner+"/"+hook.Repo, int(id), &gitlabapi.EditProjectHookOptions{
				URL: gitlabapi.String(callbackURL), Token: gitlabapi.String(secret),
			}, gitlabapi.WithContext(ctx))
			return err
		}
	case setting.SourceFromGitee, setting.SourceFromGiteeEE:
		if id > math.MaxInt32 {
			return nil, fmt.Errorf("webhook %s/%s, code host %d(%s): hook ID exceeds Gitee API range", hook.Owner, hook.Repo, host.ID, host.Alias)
		}
		c := giteetool.NewStaticClient(strings.TrimRight(host.Address, "/"), host.AccessToken, proxy, host.EnableProxy)
		_, resp, requestErr := c.WebhooksApi.GetV5ReposOwnerRepoHooksId(ctx, hook.Owner, hook.Repo, int32(id), nil)
		err = requestErr
		if err != nil && resp != nil && resp.StatusCode >= 300 {
			err = httpclient.NewGenericServerResponse(resp.StatusCode, http.MethodGet, "")
		}
		a.refresh = func(ctx context.Context, secret, callbackURL string) error {
			return c.UpdateHookSecret(ctx, hook.Owner, hook.Repo, id, &git.Hook{URL: callbackURL, Secret: secret})
		}
	default:
		return nil, fmt.Errorf("code host %d(%s): unsupported provider %s", host.ID, host.Alias, host.Type)
	}
	if err != nil {
		return nil, a.failure("read remote hook", err)
	}
	return a, nil
}
