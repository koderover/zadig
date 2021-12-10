package factory

import (
	"errors"
	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/codehost/pkg/oauth"
	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/codehost/pkg/oauth/github"
	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/codehost/pkg/oauth/gitlab"
)

func Factory(provider, callbackURL, clientID, clientSecret, hostName string) (oauth.CallbackOauth, error) {
	switch provider {
	case "github":
		return github.NewGithubOauth(callbackURL, clientID, clientSecret, hostName), nil
	case "gitlab":
		return gitlab.NewGitlabOauth(callbackURL, clientID, clientSecret, hostName), nil
	}
	return nil, errors.New("illegal provider")
}
