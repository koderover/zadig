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
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/sync/errgroup"

	"github.com/koderover/zadig/pkg/microservice/policy/core/service"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/client/policy"
	"github.com/koderover/zadig/pkg/tool/log"
)

func init() {
	rootCmd.AddCommand(migrateCmd)
}

var migrateCmd = &cobra.Command{
	Use:   "init",
	Short: "init preset role",
	Long:  `init preset role.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := run(); err != nil {
			log.Fatal(err)
		}
	},
}

func run() error {
	return presetRole()
}

func presetRole() error {
	ss := viper.Get("yaml")
	fmt.Println(ss)
	return nil
	g := new(errgroup.Group)
	g.Go(func() error {
		return policy.NewDefault().CreateSystemRole(&service.Role{
			Name: "admin",
			Rules: []*service.Rule{&service.Rule{
				Verbs:     []string{"*"},
				Resources: []string{"*"},
			}},
		})
	})

	g.Go(func() error {
		return policy.NewDefault().CreatePublicRole(&service.Role{
			Name: string(setting.Contributor),
			Rules: []*service.Rule{&service.Rule{
				Verbs:     []string{"get_workflow", "run_workflow"},
				Kind:      "resource",
				Resources: []string{"Workflow"},
			}, &service.Rule{
				Verbs:     []string{"get_environment", "config_environment", "manage_environment", "delete_environment"},
				Kind:      "resource",
				Resources: []string{"Environment"},
			}, &service.Rule{
				Verbs:     []string{"get_build", "get_service"},
				Kind:      "resource",
				Resources: []string{"Service"},
			}, &service.Rule{
				Verbs:     []string{"get_test"},
				Kind:      "resource",
				Resources: []string{"Test"},
			}},
		})
	})
	g.Go(func() error {
		return policy.NewDefault().CreatePublicRole(&service.Role{
			Name: string(setting.ReadOnly),
			Rules: []*service.Rule{&service.Rule{
				Verbs:     []string{"get_workflow"},
				Kind:      "resource",
				Resources: []string{"Workflow"},
			}, &service.Rule{
				Verbs:     []string{"get_environment"},
				Kind:      "resource",
				Resources: []string{"Environment"},
			}, &service.Rule{
				Verbs:     []string{"get_build", "get_service"},
				Kind:      "resource",
				Resources: []string{"Service"},
			}, &service.Rule{
				Verbs:     []string{"get_test"},
				Kind:      "resource",
				Resources: []string{"Test"},
			}, &service.Rule{
				Verbs:     []string{"get_delivery"},
				Kind:      "resource",
				Resources: []string{"Delivery"},
			}},
		})
	})
	if err := g.Wait(); err != nil {
		return err
	}
	return nil
}
