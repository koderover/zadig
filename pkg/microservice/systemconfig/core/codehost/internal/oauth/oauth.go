package oauth

import (
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
)

type Oauth interface {
	LoginURL(state string) (loginURL string)
	HandleCallback(r *http.Request) (*oauth2.Token, error)
}

type OAuth2Error struct {
	Err         string
	Description string
}

func (e *OAuth2Error) Error() string {
	if e.Description == "" {
		return e.Err
	}
	return fmt.Sprintf("%s: %s", e.Err, e.Description)
}
