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

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"sigs.k8s.io/yaml"

	"github.com/koderover/zadig/pkg/microservice/policy/core/service"
	"github.com/koderover/zadig/pkg/shared/client/policy"
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
	return presetRole()
}
func presetRole() error {
	g := new(errgroup.Group)
	g.Go(func() error {
		systemRole := &service.Role{}
		if err := yaml.Unmarshal(admin, systemRole); err != nil {
			log.DPanic(err)
		}
		return policy.NewDefault().CreateSystemRole(systemRole.Name, systemRole)
	})

	publicRoles := []*service.Role{}
	var readOnlyRole *service.Role
	var contributorRole *service.Role
	var projectAdminRole *service.Role
	if err := yaml.Unmarshal(readOnly, readOnlyRole); err != nil {
		log.DPanic(err)
	}
	if err := yaml.Unmarshal(contributor, contributorRole); err != nil {
		log.DPanic(err)
	}
	if err := yaml.Unmarshal(projectAdmin, projectAdminRole); err != nil {
		log.DPanic(err)
	}
	publicRoles = append(publicRoles, readOnlyRole, contributorRole, projectAdminRole)
	for _, v := range publicRoles {
		tmp := v
		g.Go(func() error {
			return policy.NewDefault().CreatePublicRole(tmp.Name, tmp)
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}
	return nil
}
