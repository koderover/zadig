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

package config

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/coreos/go-oidc"
	"github.com/spf13/viper"

	_ "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/setting"
)

func IssuerURL() string {
	return viper.GetString(setting.IssuerURL)
}

func Debug() string {
	return viper.GetString(setting.Debug)
}

func ClientID() string {
	return viper.GetString(setting.ClientID)
}

func ClientSecret() string {
	return viper.GetString(setting.ClientSecret)
}

func RedirectURI() string {
	return viper.GetString(setting.RedirectURI)
}

func Scopes() []string {
	return strings.Split(viper.GetString(setting.Scopes), ",")
}

func Client() *http.Client {
	return http.DefaultClient
}

func Provider() *oidc.Provider {
	ctx := oidc.ClientContext(context.Background(), Client())
	provider, err := oidc.NewProvider(ctx, IssuerURL())
	if err != nil {
		panic(fmt.Sprintf("init provider error:%s", err.Error()))
	}
	return provider
}

func Verifier() *oidc.IDTokenVerifier {
	return Provider().Verifier(&oidc.Config{ClientID: ClientID()})
}

func User() string {
	return viper.GetString(setting.MysqlUser)
}

func Password() string {
	return viper.GetString(setting.Password)
}

func Host() string {
	return viper.GetString(setting.Host)
}

func Name() string {
	return viper.GetString(setting.Name)
}
