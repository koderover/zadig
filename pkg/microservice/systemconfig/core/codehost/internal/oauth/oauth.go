package oauth

import (
	"net/http"

	"golang.org/x/oauth2"
)

type Oauth interface {
	LoginURL(state string) (loginURL string)
	HandleCallback(r *http.Request) (token *oauth2.Token, err error)
}

type Token struct {
	AccessToken  string
	RefreshToken string
}
