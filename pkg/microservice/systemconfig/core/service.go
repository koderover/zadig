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

package core

import (
	"context"

	configbase "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/systemconfig/config"
	"github.com/koderover/zadig/pkg/setting"
	gormtool "github.com/koderover/zadig/pkg/tool/gorm"
	"github.com/koderover/zadig/pkg/tool/log"
)

func Start(_ context.Context) {
	log.Init(&log.Config{
		Level:       configbase.LogLevel(),
		Filename:    configbase.LogFile(),
		SendToFile:  configbase.SendLogToFile(),
		Development: configbase.Mode() != setting.ReleaseMode,
	})

	initDatabase()
}

func initDatabase() {
	err := gormtool.Open(config.DexMysqlUser(),
		config.DexMysqlPassword(),
		config.DexMysqlHost(),
		config.DexMysqlDB(),
	)
	if err != nil {
		log.Panicf("Failed to open database %s", config.DexMysqlDB())
	}
}

func Stop(_ context.Context) {
	gormtool.Close()
}
