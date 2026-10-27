package rotation

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"

	githubapi "github.com/google/go-github/v35/github"
	common "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/gitee"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/github"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/gitlab"
	codehost "github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/codehost/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
	githubtool "github.com/koderover/zadig/v2/pkg/tool/git/github"
	gitlabtool "github.com/koderover/zadig/v2/pkg/tool/git/gitlab"
	giteetool "github.com/koderover/zadig/v2/pkg/tool/gitee"
	"github.com/koderover/zadig/v2/pkg/tool/httpclient"
	gitlabapi "github.com/xanzy/go-gitlab"
)

func matchCodeHost(address string, hosts []*codehost.CodeHost) (*codehost.CodeHost, error) {
	var matches []*codehost.CodeHost
	var candidates []string
	for _, host := range hosts {
		if strings.TrimRight(host.Address, "/") == strings.TrimRight(address, "/") {
			matches = append(matches, host)
			candidates = append(candidates, fmt.Sprintf("%d(%s)", host.ID, host.Alias))
		}
	}
	if len(matches) != 1 {
		return nil, fmt.Errorf("expected exactly one code host; candidates=%v", candidates)
	}
	if matches[0].AccessToken == "" {
		return nil, fmt.Errorf("code host %d(%s) has no access token", matches[0].ID, matches[0].Alias)
	}
	return matches[0], nil
}

type hookAction struct {
	hook    *common.WebHook
	host    *codehost.CodeHost
	refresh func(secret, owner, repo, hookID string) error
}

func (a *hookAction) failure(operation string, err error) error {
	// Never print SDK response bodies: they may contain credentials or the new secret.
	reason := "request failed; verify connectivity and token permissions"
	var gh *githubapi.ErrorResponse
	var gl *gitlabapi.ErrorResponse
	var response *httpclient.Error
	if errors.As(err, &gh) && gh.Response != nil {
		reason = fmt.Sprintf("HTTP %d", gh.Response.StatusCode)
	}
	if errors.As(err, &gl) && gl.Response != nil {
		reason = fmt.Sprintf("HTTP %d", gl.Response.StatusCode)
	}
	if errors.As(err, &response) {
		reason = fmt.Sprintf("HTTP %d", response.Code)
	}
	return fmt.Errorf("webhook %s/%s, code host %d(%s), hook %s: %s: %s", a.hook.Owner, a.hook.Repo, a.host.ID, a.host.Alias, a.hook.HookID, operation, reason)
}

func prepareHook(ctx context.Context, hook *common.WebHook, host *codehost.CodeHost, proxy string) (*hookAction, error) {
	a := &hookAction{hook: hook, host: host}
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
	// Static clients never refresh OAuth tokens or write codehost state during dry-run.
	switch host.Type {
	case setting.SourceFromGithub:
		cfg := &githubtool.Config{AccessToken: host.AccessToken}
		if host.EnableProxy {
			cfg.Proxy = proxy
		}
		c := &github.Client{Client: githubtool.NewClient(cfg)}
		_, _, err = c.Repositories.GetHook(ctx, hook.Owner, hook.Repo, id)
		a.refresh = c.RefreshWebHookSecret
	case setting.SourceFromGitlab:
		cli, clientErr := gitlabtool.NewOAuthTokenClient(host.Address, host.AccessToken, proxy, host.EnableProxy, host.DisableSSL)
		if clientErr != nil {
			return nil, a.failure("create client", clientErr)
		}
		c := &gitlab.Client{Client: cli}
		_, _, err = c.Projects.GetProjectHook(hook.Owner+"/"+hook.Repo, int(id), gitlabapi.WithContext(ctx))
		a.refresh = c.RefreshWebHookSecret
	case setting.SourceFromGitee, setting.SourceFromGiteeEE:
		if id > math.MaxInt32 {
			return nil, fmt.Errorf("webhook %s/%s, code host %d(%s): hook ID exceeds Gitee API range", hook.Owner, hook.Repo, host.ID, host.Alias)
		}
		c := &gitee.Client{Client: giteetool.NewStaticClient(strings.TrimRight(host.Address, "/"), host.AccessToken, proxy, host.EnableProxy)}
		var responseStatus int
		_, resp, requestErr := c.WebhooksApi.GetV5ReposOwnerRepoHooksId(ctx, hook.Owner, hook.Repo, int32(id), nil)
		err = requestErr
		if resp != nil {
			responseStatus = resp.StatusCode
		}
		if err != nil && responseStatus != 0 {
			return nil, fmt.Errorf("webhook %s/%s, code host %d(%s), hook %s: read remote hook: HTTP %d", hook.Owner, hook.Repo, host.ID, host.Alias, hook.HookID, responseStatus)
		}
		a.refresh = c.RefreshWebHookSecret
	default:
		return nil, fmt.Errorf("code host %d(%s): unsupported provider %s", host.ID, host.Alias, host.Type)
	}
	if err != nil {
		return nil, a.failure("read remote hook", err)
	}
	return a, nil
}

func (a *hookAction) update(secret string) error {
	if err := a.refresh(secret, a.hook.Owner, a.hook.Repo, a.hook.HookID); err != nil {
		return a.failure("update remote hook; restore the final maintenance backup before retrying", err)
	}
	return nil
}
