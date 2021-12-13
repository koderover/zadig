package github

import (
	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/codehost/pkg/oauth"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"net/http"
)

type Oauth struct {
	RedirectURI  string `json:"redirectURI"`
	ClientID     string `json:"clientID"`
	ClientSecret string `json:"clientSecret"`
	HostName     string `json:"host_name"`
}

func NewGithubOauth(callbackURL, clientID, clientSecret, hostName string) oauth.CallbackOauth {
	return &Oauth{
		RedirectURI:  callbackURL,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		HostName:     hostName,
	}
}

func (g *Oauth) oauth2Config() *oauth2.Config {
	endpoint := github.Endpoint
	if g.HostName != "" {
		endpoint = oauth2.Endpoint{
			AuthURL:  g.HostName + "/login/oauth/authorize",
			TokenURL: g.HostName + "/login/oauth/access_token",
		}
	}
	return &oauth2.Config{
		ClientID:     g.ClientID,
		ClientSecret: g.ClientSecret,
		Endpoint:     endpoint,
		RedirectURL:  g.RedirectURI,
		Scopes:       []string{"repo", "user"},
	}
}

type oauth2Error struct {
	error            string
	errorDescription string
}

func (e *oauth2Error) Error() string {
	if e.errorDescription == "" {
		return e.error
	}
	return e.error + ": " + e.errorDescription
}

func (g *Oauth) LoginURL(state string) (loginURL string) {
	return g.oauth2Config().AuthCodeURL(state)
}

func (g *Oauth) HandleCallback(r *http.Request) (token *oauth2.Token, err error) {
	q := r.URL.Query()
	if errType := q.Get("error"); errType != "" {
		return nil, &oauth2Error{errType, q.Get("error_description")}
	}
	return g.oauth2Config().Exchange(r.Context(), q.Get("code"))
}
