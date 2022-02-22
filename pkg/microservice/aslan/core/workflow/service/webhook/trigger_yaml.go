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

package webhook

import (
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
)

type Stage string

const (
	StageBuild  Stage = "build"
	StageDeploy Stage = "deploy"
	StageTest   Stage = "test"
)

type DeployStrategy string

const (
	DeployStrategySingle  DeployStrategy = "single"
	DeployStrategyDynamic DeployStrategy = "dynamic"
	DeployStrategyBase    DeployStrategy = "base"
)

type EnvRecyclePolicy string

const (
	EnvRecyclePolicySuccess EnvRecyclePolicy = "success"
	EnvRecyclePolicyAlways  EnvRecyclePolicy = "always"
	EnvRecyclePolicyNever   EnvRecyclePolicy = "never"
)

type TestRepoStrategy string

const (
	TestRepoStrategyDefault     TestRepoStrategy = "default"
	TestRepoStrategyCurrentRepo TestRepoStrategy = "currentRepo"
)

type TriggerYaml struct {
	Stages   []Stage          `yaml:"stages"`
	Build    []*BuildServices `yaml:"build"`
	Deploy   *Deploy          `yaml:"deploy"`
	Test     []*Test          `yaml:"test"`
	Rules    *Rules           `yaml:"rules"`
	CacheSet *CacheSet        `yaml:"cache_set"`
}

type Build struct {
	Services []*BuildServices `yaml:"services"`
}

type BuildServices struct {
	Name          string       `yaml:"name"`
	ServiceModule string       `yaml:"service_module"`
	Variables     []*Variables `yaml:"variables"`
}

type Variables struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

/*
Strategy
 single:
	Update to the specified environment;
 dynamic:
	Dynamic selection of idle environment;
 base:
	Create a new environment based on the baseline environment
	And Need to set EnvRecyclePolicy：success/always/never
*/
type Deploy struct {
	Strategy         DeployStrategy   `yaml:"strategy"`
	Envsname         []string         `yaml:"envs_name"`
	EnvRecyclePolicy EnvRecyclePolicy `yaml:"env_recycle_policy"`
	BaseNamespace    string           `yaml:"base_env"`
}

type Test struct {
	Name      string       `yaml:"name"`
	Repo      *Repo        `yaml:"repo"`
	Variables []*Variables `yaml:"variables"`
}

/*
Strategy
 default：
	Code repository information configured by default on the zadig platform
 currentRepo：
	Use current changed code information
*/
type Repo struct {
	Strategy TestRepoStrategy `yaml:"strategy"`
}

type Rules struct {
	Branchs      []string               `yaml:"branchs"`
	Events       []config.HookEventType `yaml:"events"`
	Strategy     *StrategyRules         `yaml:"strategy"`
	MatchFolders *MatchFoldersElem      `yaml:"match_folders"`
}

type StrategyRules struct {
	AutoCancel bool `yaml:"auto_cancel"`
}

type MatchFoldersElem struct {
	MatchSwitch          string              `yaml:"match_switch"`
	MatchFoldersTree     []*MatchFoldersTree `yaml:"match_folders_tree"`
	MatchFoldersSpecific string              `yaml:"match_folders_specific"`
}

type MatchFoldersTree struct {
	Name          string   `yaml:"name"`
	ServiceModule string   `yaml:"service_module"`
	FileTree      []string `yaml:"file_tree"`
}

type CacheSet struct {
	IgnoreCache bool `yaml:"ignore_cache"`
	ResetCache  bool `yaml:"reset_cache"`
}
