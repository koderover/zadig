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
	"strings"

	"github.com/spf13/viper"

	_ "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/setting"
)

func init() {
	viper.SetDefault(setting.ENVUserPort, "80")
}

func IssuerURL() string {
	return viper.GetString(setting.ENVIssuerURL)
}

func ClientID() string {
	return viper.GetString(setting.ENVClientID)
}

func ClientSecret() string {
	return viper.GetString(setting.ENVClientSecret)
}

func RedirectURI() string {
	return viper.GetString(setting.ENVRedirectURI)
}

func Scopes() []string {
	return strings.Split(viper.GetString(setting.ENVScopes), ",")
}

func MysqlUserDB() string {
	return viper.GetString(setting.ENVMysqlUserDB)
}

func MysqlDexDB() string {
	return viper.GetString(setting.ENVMysqlDexDB)
}

func TokenExpiresAt() int {
	return viper.GetInt(setting.ENVTokenExpiresAt)
}
