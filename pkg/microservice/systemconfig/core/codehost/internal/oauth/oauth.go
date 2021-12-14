package oauth

import (
	"net/http"

	"golang.org/x/oauth2"
)

type Oauth interface {
	LoginURL(state string) (loginURL string)
	HandleCallback(r *http.Request) (*oauth2.Token, error)
}
