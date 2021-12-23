package webhook

type TriggerYaml struct {
	Stages   []string `yaml:"stages"`
	Build    Build    `yaml:"build"`
	Deploy   Deploy   `yaml:"deploy"`
	Test     Test     `yaml:"test"`
	Rules    Rules    `yaml:"rules"`
	CacheSet CacheSet `yaml:"cache_set"`
}

type Build struct {
	Services []BuildServices `yaml:"services"`
}

type BuildServices struct {
	Name      string      `yaml:"name"`
	Module    string      `yaml:"module"`
	Deploy    bool        `yaml:"deploy"`
	Variables []Variables `yaml:"variables"`
}

type Variables struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}
type Deploy struct {
	//Strategy         string   `yaml:"strategy"`
	Envs             []string `yaml:"envs"`
	EnvRecyclePolicy string   `yaml:"env_recycle_policy"`
	EnvUpdatePolicy  string   `yaml:"env_update_policy"`
	EnvName          string   `yaml:"env_name"`
	Namespace        []string `yaml:"namespace"`
	BaseNamespace    string   `yaml:"base_namespace"`
}

type Test struct {
	Cases []Cases `yaml:"cases"`
}

type Cases struct {
	Name      string      `yaml:"name"`
	Repo      *Repo       `yaml:"repo"`
	Variables []Variables `yaml:"variables"`
}

type Repo struct {
	Name   string `yaml:"name"`
	Branch string `yaml:"branch"`
	Pr     string `yaml:"pr"`
}

type Rules struct {
	Branchs      []string          `yaml:"branchs"`
	Events       []string          `yaml:"events"`
	Strategy     Strategy          `yaml:"strategy"`
	MatchFolders *MatchFoldersElem `yaml:"match_folders"`
}

type Strategy struct {
	AutoCancel bool `yaml:"auto_cancel"`
}

type MatchFoldersElem struct {
	MatchSwitch          string             `yaml:"match_switch"`
	MatchFoldersTree     []MatchFoldersTree `yaml:"match_folders_tree"`
	MatchFoldersSpecific string             `yaml:"match_folders_specific"`
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
