package oauth

import (
	"golang.org/x/oauth2"
	"net/http"
)

type CallbackOauth interface {
	LoginURL(state string) (loginURL string)
	HandleCallback(r *http.Request) (token *oauth2.Token, err error)
}

type Token struct {
	AccessToken  string
	RefreshToken string
}
