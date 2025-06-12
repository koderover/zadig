/*
Copyright 2021 The KodeRover Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package oauth

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"

	"golang.org/x/oauth2"

	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/codehost/repository/models"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

type OAuth struct {
	oauth2Config *oauth2.Config
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

func New(callbackURL, clientID, clientSecret string, scopes []string, endpoint oauth2.Endpoint) *OAuth {
	return &OAuth{
		oauth2Config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Endpoint:     endpoint,
			RedirectURL:  callbackURL,
			Scopes:       scopes,
		},
	}
}

func (o *OAuth) LoginURL(state string) string {
	return o.oauth2Config.AuthCodeURL(state)
}

func (o *OAuth) HandleCallback(r *http.Request, c *models.CodeHost) (*oauth2.Token, error) {
	q := r.URL.Query()
	if errType := q.Get("error"); errType != "" {
		return nil, &OAuth2Error{errType, q.Get("error_description")}
	}

	httpClient := http.DefaultClient
	// if set http proxy
	proxies, err := commonrepo.NewProxyColl().List(&commonrepo.ProxyArgs{})
	if err == nil && len(proxies) != 0 && proxies[0].EnableRepoProxy && c.EnableProxy {
		log.Info("use proxy")
		port := proxies[0].Port
		ip := proxies[0].Address
		proxyRawUrl := fmt.Sprintf("http://%s:%d", ip, port)
		proxyUrl, err2 := url.Parse(proxyRawUrl)
		if err2 == nil {
			httpClient.Transport = &http.Transport{
				Proxy:           http.ProxyURL(proxyUrl),
				TLSClientConfig: &tls.Config{InsecureSkipVerify: c.DisableSSL},
			}
		}

	} else {
		httpClient.Transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: c.DisableSSL}}
	}

	ctx := context.Background()
	ctx = context.WithValue(ctx, oauth2.HTTPClient, httpClient)
	return o.oauth2Config.Exchange(ctx, q.Get("code"))
}
