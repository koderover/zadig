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
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/mongo"

	configbase "github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/microservice/user/config"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/repository"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/repository/orm"
	permissionservice "github.com/koderover/zadig/v2/pkg/microservice/user/core/service/permission"
	"github.com/koderover/zadig/v2/pkg/setting"
	gormtool "github.com/koderover/zadig/v2/pkg/tool/gorm"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
	"github.com/koderover/zadig/v2/pkg/types"
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
	initializeSystemRoles()
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

func initializeSystemRoles() {
	log.Infof("start initializing system roles")
	// check if the mysql Role exists
	var roleCount int64
	err := repository.DB.Table("role").Count(&roleCount).Error
	if err != nil {
		// if we failed to count the mysql role table, panic and restart.
		log.Panicf("Failed to count roles in the mysql role table to do the data initialization, error: %s", err)
	}

	tx := repository.DB.Begin()

	adminRole := &models.NewRole{
		Name:        "admin",
		Description: "拥有系统中任何操作的权限",
		Type:        int64(setting.RoleTypeSystem),
		Namespace:   "*",
	}

	err = orm.CreateRole(adminRole, tx)
	if err != nil {
		tx.Rollback()
		log.Panicf("failed to initialize admin role for system, tearing down user service...")
	}

	roleIDMap := make(map[string]uint)
	actionIDMap := make(map[string]uint)

	// initialize user group, for ONCE
	gid, _ := uuid.NewUUID()
	err = orm.CreateUserGroup(&models.UserGroup{
		GroupID:     gid.String(),
		GroupName:   types.AllUserGroupName,
		Description: "系统中的所有用户",
		Type:        int64(setting.RoleTypeSystem),
	}, tx)

	// create the role below and corresponding action binding for each project:
	// 1. project-admin
	// 2. read-only
	// 3. read-project-only
	projectList, err := mongodb.NewProjectColl().List()
	if err != nil && err != mongo.ErrNoDocuments {
		tx.Rollback()
		log.Panicf("Failed to get project list to create project default role, error: %s", err)
	}

	log.Infof("projectList count: %v, err: %+v", len(projectList), err)

	for _, project := range projectList {
		projectAdminRole := &models.NewRole{
			Name:        "project-admin",
			Description: "拥有指定项目中任何操作的权限",
			Type:        int64(setting.RoleTypeSystem),
			Namespace:   project.ProductName,
		}
		readOnlyRole := &models.NewRole{
			Name:        "read-only",
			Description: "拥有指定项目中所有资源的读权限",
			Type:        int64(setting.RoleTypeSystem),
			Namespace:   project.ProductName,
		}
		readProjectOnlyRole := &models.NewRole{
			Name:        "read-project-only",
			Description: "拥有指定项目本身的读权限，无权限查看和操作项目内资源",
			Type:        int64(setting.RoleTypeSystem),
			Namespace:   project.ProductName,
		}
		err = orm.BulkCreateRole([]*models.NewRole{projectAdminRole, readOnlyRole, readProjectOnlyRole}, tx)
		if err != nil {
			tx.Rollback()
			log.Panicf("failed to create system default role for project: %s, error: %s", project.ProductName, err)
		}
		roleIDMap[fmt.Sprintf("%s+%s", projectAdminRole.Name, projectAdminRole.Namespace)] = projectAdminRole.ID
		roleIDMap[fmt.Sprintf("%s+%s", readOnlyRole.Name, readOnlyRole.Namespace)] = readOnlyRole.ID
		roleIDMap[fmt.Sprintf("%s+%s", readProjectOnlyRole.Name, readProjectOnlyRole.Namespace)] = readProjectOnlyRole.ID

		actionIDList := make([]uint, 0)
		for _, verb := range readOnlyAction {
			if _, ok := actionIDMap[verb]; !ok {
				action, err := orm.GetActionByVerb(verb, repository.DB)
				if err != nil {
					tx.Rollback()
					log.Panicf("unexpected database error getting action, err: %s", err)
				}
				// if we found one, save it into the cache
				actionIDMap[verb] = action.ID
			}

			// after the cache was done, getting the action id and add it to the list
			actionIDList = append(actionIDList, actionIDMap[verb])
		}

		// after all the action counted for, bulk create some role-action bindings
		err = orm.BulkCreateRoleActionBindings(readOnlyRole.ID, actionIDList, tx)
		if err != nil {
			tx.Rollback()
			log.Panicf("failed to create action binding for role %s in namespace %s, error: %s", readOnlyRole.Name, readOnlyRole.Namespace, err)
		}
	}

	tx.Commit()
	log.Info("System roles initialized successfully!")
}
