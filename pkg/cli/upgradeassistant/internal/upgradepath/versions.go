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

package upgradepath

import (
	"github.com/blang/semver/v4"
	"github.com/koderover/zadig/pkg/tool/log"
)

var VersionDatas versionList

type versionList semver.Versions

func (v versionList) VersionIndex(version string) int {
	for index, _version := range VersionDatas {
		if _version.String() == version {
			return index
		}
	}
	return 0
}

func DefaultUpgradeHandler() error {
	log.Info("default upgrade handler ")
	return nil
}

func DefaultRollBackHandler() error {
	log.Info("default rollback handler ")
	return nil
}
