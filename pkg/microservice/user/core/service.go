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
	_ "embed"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	configbase "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/user/config"
	"github.com/koderover/zadig/pkg/microservice/user/core/repository"
	"github.com/koderover/zadig/pkg/microservice/user/core/repository/models"
	"github.com/koderover/zadig/pkg/microservice/user/core/service/user"
	"github.com/koderover/zadig/pkg/setting"
	gormtool "github.com/koderover/zadig/pkg/tool/gorm"
	"github.com/koderover/zadig/pkg/tool/log"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

func Start(_ context.Context) {
	log.Init(&log.Config{
		Level:       configbase.LogLevel(),
		Filename:    configbase.LogFile(),
		SendToFile:  configbase.SendLogToFile(),
		Development: configbase.Mode() != setting.ReleaseMode,
	})

	initDatabase()
	initUser()
}

func initDatabase() {
	err := gormtool.Open(configbase.MysqlUser(),
		configbase.MysqlPassword(),
		configbase.MysqlHost(),
		config.MysqlUserDB(),
	)
	if err != nil {
		log.Panicf("Failed to open database %s", config.MysqlUserDB())
	}

	err = gormtool.Open(configbase.MysqlUser(),
		configbase.MysqlPassword(),
		configbase.MysqlHost(),
		config.MysqlDexDB(),
	)

	repository.DB = gormtool.DB(config.MysqlUserDB())
	repository.DexDB = gormtool.DB(config.MysqlDexDB())

	err = gormtool.DB(config.MysqlDexDB()).AutoMigrate(&models.Connector{})
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// mongodb initialization
	mongotool.Init(ctx, configbase.MongoURI())
	if err := mongotool.Ping(ctx); err != nil {
		panic(fmt.Errorf("failed to connect to mongo, error: %s", err))
	}
}

func initUser() {
	//init default admin user
	log.Infof("============================ start to init default admin user ============================")
	err := user.InitializeAdmin()
	if err != nil {
		panic(fmt.Errorf("aslan preset system admin err:%s", err))
		return
	}
}

func Stop(_ context.Context) {
	gormtool.Close()
}

func Healthz() error {
	userDB, err := repository.DB.DB()
	if err != nil {
		log.Errorf("Healthz get db error:%s", err.Error())
		return err
	}
	if err := userDB.Ping(); err != nil {
		return err
	}

	dexDB, err := repository.DexDB.DB()
	if err != nil {
		log.Errorf("Healthz get dex db error:%s", err.Error())
		return err
	}

	return dexDB.Ping()
}
