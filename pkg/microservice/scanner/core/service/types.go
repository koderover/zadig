package service

import "fmt"

type ScannerContext struct {
	// sonar parameter
	SonarParameters []*SonarKV `yaml:"sonar_parameters"`

	// other type of scanner requires whole script to run
	Scripts []string `yaml:"scripts"`

	// Proxy Info
	Proxy *Proxy `yaml:"proxy"`

	// Repos list of repos the scanner need
	Repos []*Repo `yaml:"repos"`
}

type SonarKV struct {
	Key   string `yaml:"key"`
	Value string `yaml:"value"`
}

// Proxy information
type Proxy struct {
	Type            string `yaml:"type"`
	Address         string `yaml:"address"`
	Port            int    `yaml:"port"`
	NeedPassword    bool   `yaml:"need_password"`
	Username        string `yaml:"username"`
	Password        string `yaml:"password"`
	EnableRepoProxy bool   `yaml:"enable_repo_proxy"`
}

func (p *Proxy) GetProxyURL() string {
	var uri string
	if p.NeedPassword {
		uri = fmt.Sprintf("%s://%s:%s@%s:%d",
			p.Type,
			p.Username,
			p.Password,
			p.Address,
			p.Port,
		)
		return uri
	}

	uri = fmt.Sprintf("%s://%s:%d",
		p.Type,
		p.Address,
		p.Port,
	)
	return uri
}

// Repo ...
type Repo struct {
	Source       string `yaml:"source"`
	Address      string `yaml:"address"`
	Owner        string `yaml:"owner"`
	Name         string `yaml:"name"`
	RemoteName   string `yaml:"remote_name"`
	Branch       string `yaml:"branch"`
	PR           int    `yaml:"pr"`
	Tag          string `yaml:"tag"`
	CheckoutPath string `yaml:"checkout_path"`
	SubModules   bool   `yaml:"submodules"`
	OauthToken   string `yaml:"oauthToken"`
	User         string `yaml:"username"`
	Password     string `yaml:"password"`
	CheckoutRef  string `yaml:"checkout_ref"`
	EnableProxy  bool   `yaml:"enable_proxy"`
}
