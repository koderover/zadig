package gitlab

import (
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/gitlab"

	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/codehost/internal/oauth"
)

type oAuth struct {
	oauth2Config *oauth2.Config
}

func New(callbackURL, clientID, clientSecret, address string) oauth.Oauth {
	endpoint := gitlab.Endpoint
	if address != "" {
		endpoint = oauth2.Endpoint{
			AuthURL:  address + "/oauth/authorize",
			TokenURL: address + "/oauth/token",
		}
	}
	return &oAuth{
		oauth2Config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Endpoint:     endpoint,
			RedirectURL:  callbackURL,
			Scopes:       []string{"api", "read_user"},
		},
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
	return o.oauth2Config.AuthCodeURL(state)
}

func (o *oAuth) HandleCallback(r *http.Request) (token *oauth2.Token, err error) {
	q := r.URL.Query()
	if errType := q.Get("error"); errType != "" {
		return nil, &oauth2Error{errType, q.Get("error_description")}
	}
	return o.oauth2Config.Exchange(r.Context(), q.Get("code"))
}
