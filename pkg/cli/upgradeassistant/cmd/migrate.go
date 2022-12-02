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
	"context"
	"fmt"
	"time"

	"github.com/blang/semver/v4"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/util/sets"

	_ "github.com/koderover/zadig/pkg/cli/upgradeassistant/cmd/migrate"
	"github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/upgradepath"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

const oldestVersion = "1.3.0"

func init() {
	rootCmd.AddCommand(migrateCmd)

	migrateCmd.PersistentFlags().StringP("from-version", "f", oldestVersion, "current version to migrate from")
	migrateCmd.PersistentFlags().StringP("to-version", "t", "", "target version to migrate to")
	_ = viper.BindPFlag("fromVersion", migrateCmd.PersistentFlags().Lookup("from-version"))
	_ = viper.BindPFlag("toVersion", migrateCmd.PersistentFlags().Lookup("to-version"))
}

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "migrate database schema",
	Long:  `migrate database schema.`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return preRun()
	},
	Run: func(cmd *cobra.Command, args []string) {
		if err := run(); err != nil {
			log.Fatal(err)
		}
	},
	PostRun: func(cmd *cobra.Command, args []string) {
		if err := postRun(); err != nil {
			fmt.Println(err)
		}
	},
}

func isReleaseVersion(versionStr string) bool {
	version, _ := semver.Make(versionStr)
	return len(version.Pre) == 0 && len(version.Build) == 0
}

func findPreVersionFromList(targetVersion semver.Version, versionList semver.Versions) string {
	semver.Sort(versionList)
	curVersion := versionList[0]
	for _, version := range versionList {
		if version.Compare(targetVersion) >= 0 {
			return curVersion.FinalizeVersion()
		}
		curVersion = version
	}
	return targetVersion.FinalizeVersion()
}

func nextVersionFromList(targetVersion semver.Version, versionList semver.Versions) string {
	semver.Sort(versionList)
	for _, version := range versionList {
		if version.Compare(targetVersion) >= 0 {
			return version.FinalizeVersion()
		}
	}
	return targetVersion.FinalizeVersion()
}

func run() error {
	from := viper.GetString("fromVersion")
	to := viper.GetString("toVersion")

	log.Infof("Migrating from %s to %s", from, to)

	versionSets := sets.NewString()
	for _, rh := range upgradepath.RegisteredHandlers {
		versionSets.Insert(rh.FromVersion, rh.ToVersion)
	}

	if len(from) == 0 {
		from = oldestVersion
	}
	if len(to) == 0 {
		return fmt.Errorf("target version not assigned")
	}

	var versions semver.Versions
	for _, verStr := range versionSets.List() {
		semVersion, err := semver.Make(verStr)
		if err != nil {
			return fmt.Errorf("failed to parse version: %s, err: %s", verStr, err)
		}
		versions = append(versions, semVersion)
	}
	semver.Sort(versions)

	// for pre-release versions, find the closest previous release version
	if !versionSets.Has(from) {
		if !isReleaseVersion(from) {
			fromVersion, _ := semver.Make(from)
			from = findPreVersionFromList(fromVersion, versions)
			versions = append(versions, fromVersion)
		}
		versionSets.Insert(from)
	}
	// for pre-release versions, find the closest next release version
	if !versionSets.Has(to) {
		if !isReleaseVersion(to) {
			toVersion, _ := semver.Make(to)
			to = nextVersionFromList(toVersion, versions)
			versions = append(versions, toVersion)
		}
		versionSets.Insert(to)
	}

	for _, verStr := range versionSets.List() {
		semVersion, err := semver.Make(verStr)
		if err != nil {
			return fmt.Errorf("failed to parse version: %s, err: %s", verStr, err)
		}
		upgradepath.VersionDatas = append(upgradepath.VersionDatas, semVersion)
	}

	// sort versions by semver orders
	semver.Sort(upgradepath.VersionDatas)

	// add default handlers
	for i := 0; i < len(upgradepath.VersionDatas)-1; i++ {
		upgradepath.AddHandler(i, i+1, upgradepath.DefaultUpgradeHandler)
		upgradepath.AddHandler(i+1, i, upgradepath.DefaultRollBackHandler)
	}

	// add custom handlers
	for _, rh := range upgradepath.RegisteredHandlers {
		upgradepath.AddHandler(upgradepath.VersionDatas.VersionIndex(rh.FromVersion), upgradepath.VersionDatas.VersionIndex(rh.ToVersion), rh.Fn)
	}

	err := upgradepath.UpgradeWithBestPath(from, to)
	if err == nil {
		log.Info("Migration finished")
	}

	return err
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
