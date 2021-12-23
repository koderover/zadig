package webhook

type TriggerYaml struct {
	Stages   []string         `yaml:"stages"`
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
	Name      string       `yaml:"name"`
	Module    string       `yaml:"module"`
	Variables []*Variables `yaml:"variables"`
}

type Variables struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

/*
# 选项：
# 更新指定环境：single
# 动态选择空闲环境：all
# 更新基准环境：base，需要设置回收策略：success/always/never
*/
type Deploy struct {
	Strategy         string   `yaml:"strategy"`
	Envsname         []string `yaml:"envs_name"`
	EnvRecyclePolicy string   `yaml:"env_recycle_policy"`
	EnvUpdatePolicy  string   `yaml:"env_update_policy"`
	//EnvName          string   `yaml:"env_name"`
	BaseNamespace string `yaml:"base_env"`
}

type Test struct {
	Name      string       `yaml:"name"`
	Repo      *Repo        `yaml:"repo"`
	Variables []*Variables `yaml:"variables"`
}

/*
# default：在 zadig 平台默认配置的代码仓库信息
# currentRepo：使用当前变动的代码信息
*/
type Repo struct {
	Strategy string `yaml:"strategy"`
}

type Rules struct {
	Branchs      []string          `yaml:"branchs"`
	Events       []string          `yaml:"events"`
	Strategy     *StrategyRules    `yaml:"strategy"`
	MatchFolders *MatchFoldersElem `yaml:"match_folders"`
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
	Name     string   `yaml:"name"`
	Module   string   `yaml:"module"`
	FileTree []string `yaml:"file_tree"`
}

type CacheSet struct {
	IgnoreCache bool `yaml:"ignore_cache"`
	ResetCache  bool `yaml:"reset_cache"`
}
