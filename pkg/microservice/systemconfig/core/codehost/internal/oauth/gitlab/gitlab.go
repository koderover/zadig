package gitlab

import (
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/gitlab"

	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/codehost/internal/oauth"
)

type oAuth struct {
	RedirectURI  string
	ClientID     string
	ClientSecret string
	Address      string
}

func New(redirectURI, clientID, clientSecret, address string) oauth.Oauth {
	return &oAuth{
		RedirectURI:  redirectURI,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Address:      address,
	}
}

func (o *oAuth) oauth2Config() *oauth2.Config {
	endpoint := gitlab.Endpoint
	if o.Address != "" {
		endpoint = oauth2.Endpoint{
			AuthURL:  o.Address + "/oauth/authorize",
			TokenURL: o.Address + "/oauth/token",
		}
	}
	return &oauth2.Config{
		ClientID:     o.ClientID,
		ClientSecret: o.ClientSecret,
		Endpoint:     endpoint,
		RedirectURL:  o.RedirectURI,
		Scopes:       []string{"api", "read_user"},
	}
}

type oauth2Error struct {
	error       string
	description string
}

func (e *oauth2Error) Error() string {
	if e.description == "" {
		return e.error
	}
	return fmt.Sprintf("%s: %s", e.error, e.description)
}

func (o *oAuth) LoginURL(state string) string {
	return o.oauth2Config().AuthCodeURL(state)
}

func (o *oAuth) HandleCallback(r *http.Request) (token *oauth2.Token, err error) {
	q := r.URL.Query()
	if errType := q.Get("error"); errType != "" {
		return nil, &oauth2Error{errType, q.Get("error_description")}
	}
	return o.oauth2Config().Exchange(r.Context(), q.Get("code"))
}
