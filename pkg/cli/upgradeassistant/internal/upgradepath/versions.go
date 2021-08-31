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
	res, ok := v[version]
	if !ok {
		return V130
	}

	return res
}

func (v versions) To(version string) int {
	res, ok := v[version]
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

	return max.String()
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
