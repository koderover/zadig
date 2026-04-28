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
	"database/sql"
	_ "embed"
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"

	configbase "github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/microservice/user/config"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/repository"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/repository/models"
	permissionservice "github.com/koderover/zadig/v2/pkg/microservice/user/core/service/permission"
	"github.com/koderover/zadig/v2/pkg/setting"
	gormtool "github.com/koderover/zadig/v2/pkg/tool/gorm"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
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
	InitializeUserDBAndTables()

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
	if err != nil {
		log.Panicf("Failed to open database %s", config.MysqlDexDB())
	}

	repository.DB = gormtool.DB(config.MysqlUserDB())
	sqlDB, err := repository.DB.DB()
	if err != nil {
		panic("failed to create sqldb for user database")
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(200)
	repository.DexDB = gormtool.DB(config.MysqlDexDB())

	// mysql model migration
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

	initializeSystemActions()
}

func Stop(_ context.Context) {
	gormtool.Close()
}

//go:embed init/mysql.sql
var userSchema []byte

//go:embed init/dm_mysql.sql
var dmUserSchema []byte

//go:embed init/dex_database.sql
var dexSchema []byte

//go:embed init/action_initialization.sql
var actionData []byte

//go:embed init/role_template_initialization.sql
var roleTemplateData []byte

//go:embed init/dm_action_initialization.sql
var dmActionData []byte

//go:embed init/dm_role_template_initialization.sql
var dmRoleTemplateData []byte

var readOnlyAction = []string{
	permissionservice.VerbGetDelivery,
	permissionservice.VerbGetTest,
	permissionservice.VerbGetService,
	permissionservice.VerbGetProductionService,
	permissionservice.VerbGetBuild,
	permissionservice.VerbGetWorkflow,
	permissionservice.VerbGetEnvironment,
	permissionservice.VerbGetProductionEnv,
	permissionservice.VerbGetScan,
	permissionservice.VerbGetSprint,
}

func InitializeUserDBAndTables() {
	if len(userSchema) == 0 {
		return
	}

	if !configbase.MysqlUseDM() {
		dsn := fmt.Sprintf(
			"%s:%s@tcp(%s)/?charset=utf8&multiStatements=true",
			configbase.MysqlUser(), configbase.MysqlPassword(), configbase.MysqlHost(),
		)
		db, err := sql.Open("mysql", dsn)
		if err != nil {
			log.Panic(err)
		}
		defer db.Close()

		initSql := fmt.Sprintf(string(userSchema), config.MysqlUserDB(), config.MysqlUserDB())
		_, err = db.Exec(initSql)
		if err != nil {
			log.Panic(err)
		}

		dexDatabaseSql := fmt.Sprintf(string(dexSchema), config.MysqlDexDB())
		_, err = db.Exec(dexDatabaseSql)

		if err != nil {
			log.Panic(err)
		}
	} else {
		dsn := fmt.Sprintf(
			"dm://%s:%s@%s",
			configbase.MysqlUser(), configbase.MysqlPassword(), configbase.MysqlHost(),
		)
		db, err := sql.Open("dm", dsn)
		if err != nil {
			log.Panic(err)
		}
		defer db.Close()

		schemaArr := strings.Split(string(dmUserSchema), "\n\n")
		for _, schema := range schemaArr {
			_, err = db.Exec(schema)
			if err != nil {
				log.Panic(err)
			}
		}
	}

}

func initializeSystemActions() {
	fmt.Println("initializing system actions...")
	if !configbase.MysqlUseDM() {
		err := repository.DB.Exec(string(actionData)).Error
		if err != nil {
			log.Panic(err)
		}
		err = repository.DB.Exec(string(roleTemplateData)).Error
		if err != nil {
			log.Panic(err)
		}
	} else {
		// @todo need to optimize the dm action sql
		// dm doesn't support ON DUPLICATE KEY UPDATE, but it can be replace by MERGE INTO
		err := repository.DB.Exec(string(dmActionData)).Error
		if err != nil {
			log.Panic(err)
		}
		err = repository.DB.Exec(string(dmRoleTemplateData)).Error
		if err != nil {
			log.Panic(err)

		}
	}
	fmt.Println("system actions initialized...")
}
