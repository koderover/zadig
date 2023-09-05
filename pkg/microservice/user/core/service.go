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
	"time"

	_ "github.com/go-sql-driver/mysql"
	configbase "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/user/config"
	"github.com/koderover/zadig/pkg/microservice/user/core/repository"
	"github.com/koderover/zadig/pkg/microservice/user/core/repository/models"
	"github.com/koderover/zadig/pkg/microservice/user/core/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/user/core/repository/orm"
	userservice "github.com/koderover/zadig/pkg/microservice/user/core/service/permission"
	"github.com/koderover/zadig/pkg/setting"
	gormtool "github.com/koderover/zadig/pkg/tool/gorm"
	"github.com/koderover/zadig/pkg/tool/log"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
	"go.mongodb.org/mongo-driver/mongo"
)

func Start(_ context.Context) {
	log.Init(&log.Config{
		Level:       configbase.LogLevel(),
		Filename:    configbase.LogFile(),
		SendToFile:  configbase.SendLogToFile(),
		Development: configbase.Mode() != setting.ReleaseMode,
	})

	userservice.GenerateOPABundle()
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

	repository.DB = gormtool.DB(config.MysqlUserDB())
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
	syncUserRoleBinding()
}

func Stop(_ context.Context) {
	gormtool.Close()
}

//go:embed init/mysql.sql
var userSchema []byte

//go:embed init/dex_database.sql
var dexSchema []byte

//go:embed init/action_initialization.sql
var actionData []byte

var readOnlyAction = []string{
	userservice.VerbGetDelivery,
	userservice.VerbGetTest,
	userservice.VerbGetService,
	userservice.VerbGetProductionService,
	userservice.VerbGetBuild,
	userservice.VerbGetWorkflow,
	userservice.VerbGetEnvironment,
	userservice.VerbGetProductionEnv,
	userservice.VerbGetScan,
}

func InitializeUserDBAndTables() {
	if len(userSchema) == 0 {
		return
	}
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
}

func initializeSystemActions() {
	var count int64
	err := repository.DB.Table(fmt.Sprintf("%s.action", config.MysqlUserDB())).Count(&count).Error
	if err != nil {
		panic("failed to count")
	}

	if count == 0 {
		fmt.Println("initializing system actions...")
		err := repository.DB.Exec(fmt.Sprintf(string(actionData), config.MysqlUserDB()))

		if err != nil {
			log.Panic(err)
		}
		fmt.Println("system actions initialized...")
	}
}

// syncUserRoleBinding sync all the roles and role binding into mysql after 1.7
// NOTE:
// this action will only be performed once regardless of the version, the execution condition is there are no roles in mysql table
// since this could be a lengthy procedure, the helm installation process need to be modified.
func syncUserRoleBinding() {
	// check if the mysql Role exists
	var roleCount int64
	err := repository.DB.Table("role").Count(&roleCount).Error
	if err != nil {
		// if we failed to count the mysql role table, panic and restart.
		log.Panicf("Failed to count roles in the mysql role table to do the data initialization, error: %s", err)
	}

	if roleCount > 0 {
		return
	}

	tx := repository.DB.Begin()

	// if there are no role presented in the roles table, it means that the move all the roles and corresponding role binding into mysql
	allRoles, err := mongodb.NewRoleColl().List()
	if err != nil {
		if err != mongo.ErrNoDocuments {
			log.Panicf("failed to list all roles from previous system, error: %s", err)
		} else {
			// if no roles is in the previous mongodb, it is a fresh installation. We create the default role, which is just system admin, and finish
			adminRole := &models.NewRole{
				Name:        "admin",
				Description: "",
				Type:        int64(setting.RoleTypeSystem),
				Namespace:   "*",
			}

			err := orm.CreateRole(adminRole, tx)
			if err != nil {
				tx.Rollback()
				log.Panicf("failed to initialize admin role for system, tearing down user service...")
			}
		}
		return
	}

	roleIDMap := make(map[string]uint)
	actionIDMap := make(map[string]uint)

	// create the role below and corresponding action binding for each project:
	// 1. project-admin
	// 2. read-only
	// 3. read-project-only
	projectList, err := mongodb.NewProjectColl().List()
	if err != nil {
		if err != mongo.ErrNoDocuments {
			log.Panicf("Failed to get project list to create project default role, error: %s", err)
		} else {
			return
		}
	}

	for _, project := range projectList {
		projectAdminRole := &models.NewRole{
			Name:        "project-admin",
			Description: "",
			Type:        int64(setting.RoleTypeSystem),
			Namespace:   project.ProductName,
		}
		readOnlyRole := &models.NewRole{
			Name:        "read-only",
			Description: "",
			Type:        int64(setting.RoleTypeSystem),
			Namespace:   project.ProductName,
		}
		readProjectOnlyRole := &models.NewRole{
			Name:        "read-project-only",
			Description: "",
			Type:        int64(setting.RoleTypeSystem),
			Namespace:   project.ProductName,
		}
		err = orm.BulkCreateRole([]*models.NewRole{projectAdminRole, readOnlyRole, readProjectOnlyRole}, tx)
		if err != nil {
			tx.Rollback()
			log.Panicf("failed to create system default role for project: %s, error: %s", project.ProductName, err)
		}
		// TODO: Delete this debug code
		if readOnlyRole.ID == 0 {
			tx.Rollback()
			log.Panicf("readOnlyRole does not have an ID")
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

RoleLoop:
	for _, role := range allRoles {
		// create corresponding mysql role
		mysqlRole := &models.NewRole{
			Name:        role.Name,
			Description: role.Desc,
			Namespace:   role.Namespace,
		}

		if role.Type == setting.ResourceTypeSystem {
			mysqlRole.Type = int64(setting.RoleTypeSystem)
		} else {
			mysqlRole.Type = int64(setting.RoleTypeCustom)
		}

		// special case for "project-admin", "contributor", "read-only" and "read-project-only"
		// this will be dealt for each project
		if role.Namespace == "" {
			continue RoleLoop
		} else {
			err = orm.CreateRole(mysqlRole, tx)
			if err != nil {
				tx.Rollback()
				log.Panicf("failed to create role: %s for namespace %s, error: %s", role.Namespace, role.Namespace, err)
			}
		}

		// save the role information into the map (mainly for id)
		identity := fmt.Sprintf("%s+%s", mysqlRole.Name, mysqlRole.Namespace)
		roleIDMap[identity] = mysqlRole.ID

		// after the role and role binding is created, create its corresponding action binding
		actionIDList := make([]uint, 0)
		for _, resourceAction := range role.Rules {
		VerbLoop:
			for _, verb := range resourceAction.Verbs {
				// admins and project-admins, which only have a verb "*", does not need role
				if verb == "*" {
					continue RoleLoop
				}

				if _, ok := actionIDMap[verb]; !ok {
					action, err := orm.GetActionByVerb(verb, repository.DB)
					if err != nil {
						tx.Rollback()
						log.Panicf("unexpected database error getting action, err: %s", err)
					}
					// if we found one, save it into the cache
					if action.ID != 0 {
						actionIDMap[verb] = action.ID
					} else {
						log.Errorf("failed to find action: %s", verb)
						// otherwise do nothing
						continue VerbLoop
					}
				}

				// after the cache was done, getting the action id and add it to the list
				actionIDList = append(actionIDList, actionIDMap[verb])
			}
		}
		// after all the action counted for, bulk create some role-action bindings
		err = orm.BulkCreateRoleActionBindings(mysqlRole.ID, actionIDList, tx)
		if err != nil {
			tx.Rollback()
			log.Panicf("failed to create action binding for role %s in namespace %s, error: %s", mysqlRole.Name, mysqlRole.Namespace, err)
		}
	}

	// after syncing all the roles into the database, sync the user-role binding into the mysql table and we are done
	rbList, err := mongodb.NewRoleBindingColl().List()
	if err != nil {
		if err != mongo.ErrNoDocuments {
			tx.Rollback()
			log.Panicf("failed to find role bindings to sync, error: %s", err)
		} else {
			return
		}
	}

	rbmap := make(map[string][]uint)

	for _, rb := range rbList {
		// dangerous, but ok for the system
		uid := rb.Subjects[0].UID
		if uid == "*" {
			// TODO: throw it into the group binding, we will deal with it later.
			continue
		}

		// the role_ref.namespace is not really reliable, so we will just use namespace, special case list:
		// 1. admin: role_ref.name = admin, role_ref.namespace = *, namespace = *
		// 2. project_admin: role_ref.name = project-admin, role_ref.namespace = "", namespace = project_key
		// 3. read_only: role_ref.name = read-only, role_ref.namespace = "", namespace = project_key
		// 4. read_project_only: role_ref.name = read-project-only, role_ref.namespace = "", namespace = project_key
		roleKey := fmt.Sprintf("%s+%s", rb.RoleRef.Name, rb.Namespace)
		if roleID, ok := roleIDMap[roleKey]; ok {
			rbmap[uid] = append(rbmap[uid], roleID)
		} else {
			// if the role is not found, there is a possibility that the role has been deleted, we just print error logs.
			log.Errorf("role: %s in namespace: %s not found, skip creating role binding between uid: %s and role: %s...", rb.RoleRef.Name, rb.Namespace, uid, rb.RoleRef.Name)
			continue
		}
	}

	for uid, roleIDList := range rbmap {
		err = orm.BulkCreateRoleBindingForUser(uid, roleIDList, tx)
		if err != nil {
			tx.Rollback()
			log.Panicf("failed to batch create role bindings for user: %s, error is: %s", uid, err)
		}
	}

	tx.Commit()
	log.Info("User role and role binding synchronization done successfully!")
}
