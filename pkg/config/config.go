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
	"fmt"

	"github.com/spf13/viper"

	"github.com/koderover/zadig/pkg/setting"
)

func Mode() string {
	mode := viper.GetString(setting.ENVMode)
	if mode == "" {
		return setting.DebugMode
	}

	return mode
}

func LogLevel() string {
	return "debug"
}

func SendLogToFile() bool {
	return true
}

func LogPath() string {
	return fmt.Sprintf("/var/log/%s/", setting.ProductName)
}

func LogName() string {
	return "product.log"
}

func RequestLogName() string {
	return "request.log"
}

func LogFile() string {
	return LogPath() + LogName()
}

func RequestLogFile() string {
	return LogPath() + RequestLogName()
}

func AslanURL() string {
	return viper.GetString(setting.ENVAslanURL)
}

func PoetryAPIServer() string {
	return viper.GetString(setting.ENVPoetryAPIServer)
}

func PoetryAPIRootKey() string {
	return viper.GetString(setting.ENVPoetryAPIRootKey)
}
