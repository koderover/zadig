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
	"log"

	"github.com/spf13/viper"

	"github.com/koderover/zadig/lib/setting"
	"github.com/koderover/zadig/lib/util"
)

func init() {
	exists, err := util.FileExists(setting.LocalConfig)
	if err != nil {
		log.Fatal(err)
	}

	// load env file for local test
	if exists {
		viper.SetConfigFile(setting.LocalConfig)
		if err := viper.ReadInConfig(); err != nil {
			log.Fatal(err)
		}
	}

	// load all Environment variables
	viper.AutomaticEnv()
}
