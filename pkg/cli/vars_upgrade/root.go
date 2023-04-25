/*
Copyright 2023 The KodeRover Authors.

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

package vars_upgrade

import (
	"context"
	"fmt"
	"time"

	"github.com/koderover/zadig/pkg/cli/upgradeassistant/cmd/migrate"

	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var rootCmd = &cobra.Command{
	Use:   "vars-upgrade",
	Short: "An upgrade assistant for Zadig",
	Long:  `vars-upgrade is an upgrade assistant for Zadig to migrate schema, sync data and do any other things which is required in an upgrade.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := run(); err != nil {
			log.Fatal(err)
		}
	},
	PreRun: func(cmd *cobra.Command, args []string) {
		if err := preRun(); err != nil {
			log.Fatal(err)
		}
	},
	PostRun: func(cmd *cobra.Command, args []string) {
		if err := postRun(); err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringP("connection-string", "c", "", "mongodb connection string")
	rootCmd.PersistentFlags().StringP("database", "d", "", "name of the database")
	rootCmd.PersistentFlags().BoolP("write", "r", false, "write data")
	rootCmd.PersistentFlags().BoolP("message", "m", false, "detailed message")
	rootCmd.PersistentFlags().StringP("templates", "t", "", "appointed templates")
	rootCmd.PersistentFlags().StringP("projects", "p", "", "appointed project")

	_ = viper.BindPFlag(setting.ENVMongoDBConnectionString, rootCmd.PersistentFlags().Lookup("connection-string"))
	_ = viper.BindPFlag(setting.ENVAslanDBName, rootCmd.PersistentFlags().Lookup("database"))
	_ = viper.BindPFlag("Write", rootCmd.PersistentFlags().Lookup("write"))
	_ = viper.BindPFlag("Message", rootCmd.PersistentFlags().Lookup("message"))
	_ = viper.BindPFlag("Templates", rootCmd.PersistentFlags().Lookup("templates"))
	_ = viper.BindPFlag("Projects", rootCmd.PersistentFlags().Lookup("projects"))
}

func initConfig() {
	viper.AutomaticEnv()

	log.Init(&log.Config{
		Level:    "debug",
		NoCaller: true,
	})
}

func run() error {
	write = viper.GetBool("Write")
	outPutMessages = viper.GetBool("Message")
	appointedTemplates = viper.GetString("Templates")
	appointedProjects = viper.GetString("Projects")

	// 1.15.0 - 1.16.0
	log.Infof("************ executing from 1.15.0 to 1.16.0 ************")
	err := migrate.V1150ToV1160()
	if err != nil {
		return err
	}

	// 1.16.0 数据变更
	log.Infof("************ executing 1.16.0 variables ************")
	err = handlerServices()
	if err != nil {
		return err
	}

	err = handlerEnvVars()
	if err != nil {
		return err
	}

	// 1.16.0 - 1.17.0
	log.Infof("************ executing from 1.16.0 to 1.17.0 ************")
	err = migrate.V1160ToV1170()
	if err != nil {
		return err
	}

	// 1.17.0 - 1.18.0
	log.Infof("************ executing from 1.17.0 to 1.18.0 ************")
	err = migrate.V1170ToV1180()
	if err != nil {
		return err
	}

	return nil
}

func preRun() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	mongotool.Init(ctx, viper.GetString(setting.ENVMongoDBConnectionString))
	if err := mongotool.Ping(ctx); err != nil {
		return fmt.Errorf("failed to connect to mongo, error: %s", err)
	}

	return nil
}

func postRun() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := mongotool.Close(ctx); err != nil {
		return fmt.Errorf("failed to close mongo connection, error: %s", err)
	}

	return nil
}

func Execute() error {
	return rootCmd.Execute()
}
