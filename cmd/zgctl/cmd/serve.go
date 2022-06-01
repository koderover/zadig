/*
Copyright 2022 The KodeRover Authors.

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
	"fmt"
	"os"
	"path/filepath"

	"github.com/dgraph-io/badger/v3"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"

	"github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/zgctl"
	ginmiddleware "github.com/koderover/zadig/pkg/middleware/gin"
	"github.com/koderover/zadig/pkg/tool/log"
)

var listenAddr string

func init() {
	serveCmd.Flags().StringVar(&listenAddr, "listen-addr", ":8080", "listen address of zgctl serve")

	rootCmd.AddCommand(serveCmd)
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "provide APIs for IDE Plugin",
	Long:  "serve is a sub cmd used to provide APIs for IDE Plugin.",
	RunE: func(cmd *cobra.Command, args []string) error {
		r := gin.Default()
		injectMiddlewares(r)

		homeDir := os.ExpandEnv(homeDir)
		dbPath := filepath.Join(homeDir, "db")
		db, err := badger.Open(badger.DefaultOptions(dbPath))
		if err != nil {
			return fmt.Errorf("failed to open db %q: %s", dbPath, err)
		}
		defer db.Close()

		apiv1 := r.Group("/api/v1")
		router := zgctl.NewRouter(&zgctl.RouterConfig{
			ZadigHost:  zadigHost,
			ZadigToken: zadigToken,
			HomeDir:    homeDir,
			DB:         db,
		})
		router.Inject(apiv1)

		return r.Run(listenAddr)
	},
}

func injectMiddlewares(g *gin.Engine) {
	g.Use(ginmiddleware.Response())
	g.Use(ginmiddleware.RequestID())
	g.Use(ginmiddleware.RequestLog(log.NewFileLogger(config.RequestLogFile())))
	g.Use(gin.Recovery())
}
