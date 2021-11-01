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
	_ "embed"
	"fmt"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"sigs.k8s.io/yaml"

	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/client/policy"
	"github.com/koderover/zadig/pkg/shared/client/user"
	"github.com/koderover/zadig/pkg/tool/log"
)

func init() {
	rootCmd.AddCommand(initCmd)
}

//go:embed contributor.yaml
var contributor []byte

//go:embed read-only.yaml
var readOnly []byte

//go:embed admin.yaml
var admin []byte

//go:embed project-admin.yaml
var projectAdmin []byte

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "init system config",
	Long:  `init system config.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := run(); err != nil {
			log.Fatal(err)
		}
	},
}

func run() error {
	return initSystemConfig()
}

func initSystemConfig() error {
	if err := presetSystemAdmin(); err != nil {
		return err
	}
	if err := presetRole(); err != nil {
		return err
	}

	return presetRoleBinding()
}

func presetSystemAdmin() error {
	return user.New().CreateUser(&user.CreateUserArgs{
		Name:     "admin",
		Password: "admin",
		Account:  "admin",
	})
}

func presetRoleBinding() error {
	return policy.NewDefault().CreateOrUpdateRoleBinding("*", &policy.RoleBinding{
		Name:   fmt.Sprintf(setting.SystemRoleBindingNameFmt, "admin"),
		UID:    "1",
		Role:   "admin",
		Public: true,
	})
}

func presetRole() error {
	g := new(errgroup.Group)
	g.Go(func() error {
		systemRole := &policy.Role{}
		if err := yaml.Unmarshal(admin, systemRole); err != nil {
			log.DPanic(err)
		}
		return policy.NewDefault().CreateSystemRole(systemRole.Name, systemRole)
	})

	rolesArray := [][]byte{readOnly, contributor, projectAdmin}

	for _, v := range rolesArray {
		role := &policy.Role{}
		if err := yaml.Unmarshal(v, role); err != nil {
			log.DPanic(err)
		}
		g.Go(func() error {
			return policy.NewDefault().CreatePublicRole(role.Name, role)
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}
	return nil
}
