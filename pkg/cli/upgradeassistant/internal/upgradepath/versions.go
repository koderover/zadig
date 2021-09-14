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

	"github.com/koderover/zadig/version"
)

const (
	Latest = iota + 1
	V130
	V131
)

var versionMap versions = map[string]int{
	"1.3.0": V130,
	"1.3.1": V131,
}

type versions map[string]int

func (v versions) From(version string) int {
	f, err := semver.Make(version)
	if err != nil {
		return Latest
	}
	res, ok := v[f.FinalizeVersion()]
	if !ok {
		return Latest
	}

	return res
}

func (v versions) To(version string) int {
	t, err := semver.Make(version)
	if err != nil {
		return Latest
	}
	res, ok := v[t.FinalizeVersion()]
	if !ok {
		return Latest
	}

	return res
}

func (v versions) Max() int {
	return v[v.MaxVersionString()]
}

func (v versions) MaxVersionString() string {
	max, _ := semver.Make("0.0.1")
	for ver := range v {
		sv, _ := semver.Make(ver)
		if sv.GT(max) {
			max = sv
		}
	}

	return max.FinalizeVersion()
}

func init() {
	maxVersion, _ := semver.Make(versionMap.MaxVersionString())
	currentVersion, _ := semver.Make(version.Version)
	if currentVersion.GT(maxVersion) {
		AddHandler(versionMap.Max(), Latest, UpgradeToLatest)
		AddHandler(Latest, versionMap.Max(), RollbackFromLatest)
	}
}

func UpgradeToLatest() error {
	return nil
}

func RollbackFromLatest() error {
	return nil
}
