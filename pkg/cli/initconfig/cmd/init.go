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
	rootCmd.AddCommand(initRoleCmd)
}

//go:embed contributor.yaml
var contributor []byte

//go:embed read-only.yaml
var readOnly []byte

//go:embed admin.yaml
var admin []byte

var initRoleCmd = &cobra.Command{
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
	return initSystemConfig()
}

func initSystemConfig() {
	presetRole()
}
func presetRole() error {
	g := new(errgroup.Group)
	g.Go(func() error {
		adminRole := &service.Role{}
		if err := yaml.Unmarshal(admin, adminRole); err != nil {
			log.DPanic(err)
		}
		return policy.NewDefault().CreateSystemRole(adminRole)
	})

	var publicRoles []*service.Role
	contributorRole := &service.Role{}
	if err := yaml.Unmarshal(contributor, contributorRole); err != nil {
		log.DPanic(err)
	}
	readOnlyRole := &service.Role{}
	if err := yaml.Unmarshal(readOnly, readOnlyRole); err != nil {
		log.DPanic(err)
	}
	publicRoles = append(publicRoles, contributorRole, readOnlyRole)

	for _, v := range publicRoles {
		g.Go(func() error {
			return policy.NewDefault().CreatePublicRole(v)
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}
	return nil
}
