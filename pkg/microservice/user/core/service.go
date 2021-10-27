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

	"gorm.io/gorm"

	"github.com/koderover/zadig/pkg/config"
	config2 "github.com/koderover/zadig/pkg/microservice/user/config"
	"github.com/koderover/zadig/pkg/setting"
	gormtool "github.com/koderover/zadig/pkg/tool/gorm"
	"github.com/koderover/zadig/pkg/tool/log"
)

var DB *gorm.DB

type Model struct {
	CreatedAt int64 `json:"created_at"`
	UpdatedAt int64 `json:"updated_at"`
}

func Start(_ context.Context) {
	log.Init(&log.Config{
		Level:       config.LogLevel(),
		Filename:    config.LogFile(),
		SendToFile:  config.SendLogToFile(),
		Development: config.Mode() != setting.ReleaseMode,
	})

	initDatabase()
	DB = gormtool.DB(config2.MysqlUserDB())
}

func initDatabase() {
	err := gormtool.Open(config.MysqlUser(),
		config.MysqlPassword(),
		config.MysqlHost(),
		config2.MysqlUserDB(),
	)
	if err != nil {
		log.Panicf("Failed to open database %s", config2.MysqlUserDB())
	}
}

func Stop(_ context.Context) {
	gormtool.Close()
}
