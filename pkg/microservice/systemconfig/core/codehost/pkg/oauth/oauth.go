package oauth

import (
	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/codehost/pkg/oauth/github"
	"golang.org/x/oauth2"
	"net/http"
)

type CallbackOauth interface{
	LoginURL(state string)(loginURL string)
	HandleCallback(r *http.Request)(token *oauth2.Token,err error)
}

type Token struct {
	AccessToken string
	RefreshToken string
}

func Factory(provider ,redirectURI ,clientID,clientSecret,hostName string)CallbackOauth{
	switch provider {
	case"github":
		return github.NewGithubOauth(redirectURI ,clientID,clientSecret,hostName)
	default:
		github.NewGithubOauth(redirectURI ,clientID,clientSecret,hostName)
	}
}