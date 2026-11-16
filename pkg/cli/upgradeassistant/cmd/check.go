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

	"github.com/koderover/zadig/v2/pkg/shared/client/plutusenterprise"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

func init() {
	rootCmd.AddCommand(checkUpgradeCmd)

	checkUpgradeCmd.PersistentFlags().StringP("from-version", "f", oldestVersion, "current version to migrate from")
	checkUpgradeCmd.PersistentFlags().StringP("to-version", "t", "", "target version to migrate to")
}

var checkUpgradeCmd = &cobra.Command{
	Use:   "check",
	Short: "check if the upgrade is possible",
	Long:  `check if the upgrade is possible.`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return preRun()
	},
	Run: func(cmd *cobra.Command, args []string) {
		from, _ := cmd.Flags().GetString("from-version")
		to, _ := cmd.Flags().GetString("to-version")
		if err := runCheckUpgrade(from, to); err != nil {
			log.Fatal(err)
		}
	},
	PostRun: func(cmd *cobra.Command, args []string) {
		if err := postRun(); err != nil {
			fmt.Println(err)
		}
	},
}

func runCheckUpgrade(from, to string) error {
	if len(from) == 0 {
		from = oldestVersion
	}
	if len(to) == 0 {
		return fmt.Errorf("target version not assigned")
	}

	log.Infof("Checking upgrade from %s to %s", from, to)

	resp, err := plutusenterprise.New().CheckUpgrade(from, to)
	if err != nil {
		nerr := e.ErrUpgradeNotAllowed.AddErr(err)
		log.Error(nerr)
		return nerr
	}
	if !resp.AllowUpgrade {
		nerr := e.ErrUpgradeNotAllowed
		log.Error(nerr)
		return nerr
	}

	log.Info("Upgrade check passed")
	return nil
}
