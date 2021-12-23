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
	"sort"

	"github.com/blang/semver/v4"
)

const (
	Latest = iota + 1
	V130
	V131
	V140
	V150
	V160
	V170
	V171
	V180
)

var versionMap versions = map[string]int{
	"1.3.0": V130,
	"1.3.1": V131,
	"1.4.0": V140,
	"1.5.0": V150,
	"1.6.0": V160,
	"1.7.0": V170,
	"1.7.1": V171,
	"1.8.0": V180,
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
	versionList := make([]string, 0, len(versionMap))
	for v := range versionMap {
		versionList = append(versionList, v)
	}

	sort.Strings(versionList)

	for i := 0; i < len(versionList)-1; i++ {
		lowVersion := versionList[i]
		highVersion := versionList[i+1]
		AddHandler(versionMap[lowVersion], versionMap[highVersion], defaultUpgradeHandler)
		AddHandler(versionMap[highVersion], versionMap[lowVersion], defaultRollBackHandler)
	}
}

func defaultUpgradeHandler() error {
	return nil
}

func defaultRollBackHandler() error {
	return nil
}
