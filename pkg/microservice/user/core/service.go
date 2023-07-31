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
	"github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/user/core/repository/models"
	"github.com/koderover/zadig/pkg/microservice/user/core/service/user"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
	"gorm.io/gorm"

	config2 "github.com/koderover/zadig/pkg/microservice/user/config"
	gormtool "github.com/koderover/zadig/pkg/tool/gorm"
)

var DB *gorm.DB
var DexDB *gorm.DB

func Start(_ context.Context) {
	log.Init(&log.Config{
		Level:       config.LogLevel(),
		Filename:    config.LogFile(),
		SendToFile:  config.SendLogToFile(),
		Development: config.Mode() != setting.ReleaseMode,
	})

	initDatabase()
	initUser()
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

	err = gormtool.Open(config.MysqlUser(),
		config.MysqlPassword(),
		config.MysqlHost(),
		config2.MysqlDexDB(),
	)

	DB = gormtool.DB(config2.MysqlUserDB())
	DexDB = gormtool.DB(config2.MysqlDexDB())

	err = gormtool.DB(config2.MysqlDexDB()).AutoMigrate(&models.Connector{})
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// mongodb initialization
	mongotool.Init(ctx, config.MongoURI())
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
	userDB, err := DB.DB()
	if err != nil {
		log.Errorf("Healthz get db error:%s", err.Error())
		return err
	}
	if err := userDB.Ping(); err != nil {
		return err
	}

	dexDB, err := DexDB.DB()
	if err != nil {
		log.Errorf("Healthz get dex db error:%s", err.Error())
		return err
	}

	return dexDB.Ping()
}
