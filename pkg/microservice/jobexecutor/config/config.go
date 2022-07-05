/*
Copyright 2022 The KodeRover Authors.

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
	"github.com/koderover/zadig/pkg/setting"
	"github.com/spf13/viper"

	_ "github.com/koderover/zadig/pkg/config"
)

func JobConfigFile() string {
	return viper.GetString(setting.JobConfigFile)
}

func Path() string {
	return viper.GetString(setting.Path)
}

func DockerHost() string {
	return viper.GetString(setting.DockerHost)
}

func Home() string {
	return viper.GetString(setting.Home)
}
