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

package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/repository"
	"github.com/koderover/zadig/v2/pkg/setting"
	gormtool "github.com/koderover/zadig/v2/pkg/tool/gorm"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

var rootCmd = &cobra.Command{
	Use:   "ua",
	Short: "An upgrade assistant for Zadig",
	Long:  `ua is an upgrade assistant for Zadig to migrate schema, sync data and do any other things which is required in an upgrade.`,
}

// Execute executes the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringP("connection-string", "c", "", "mongodb connection string")
	rootCmd.PersistentFlags().StringP("database", "d", "", "name of the database")

	_ = viper.BindPFlag(setting.ENVMongoDBConnectionString, rootCmd.PersistentFlags().Lookup("connection-string"))
	_ = viper.BindPFlag(setting.ENVAslanDBName, rootCmd.PersistentFlags().Lookup("database"))

	initMysql()
}

func initConfig() {
	viper.AutomaticEnv()

	log.Init(&log.Config{
		Level:    "debug",
		NoCaller: true,
	})
}

func initMysql() {
	err := gormtool.Open(config.MysqlUser(),
		config.MysqlPassword(),
		config.MysqlHost(),
		config.MysqlUserDB(),
	)
	if err != nil {
		log.Panicf("Failed to open database %s, err: %v", config.MysqlUserDB(), err)
	}

	repository.DB = gormtool.DB(config.MysqlUserDB())
	sqlDB, err := repository.DB.DB()
	if err != nil {
		panic("failed to create sqldb for user database")
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(200)
}
