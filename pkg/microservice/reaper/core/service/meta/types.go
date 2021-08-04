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

package meta

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"path"
	"strings"

	"github.com/koderover/zadig/pkg/microservice/reaper/config"
)

// Context ...
type Context struct {
	// Workspace 容器工作目录 [必填]
	Workspace string `yaml:"workspace"`

	// CleanWorkspace 是否清理工作目录 [选填, 默认为 false]
	CleanWorkspace bool `yaml:"clean_workspace"`

	// Paths 执行脚本Path
	Paths string `yaml:"-"`

	// Proxy 翻墙配置信息
	Proxy *Proxy `yaml:"proxy"`

	// Envs 用户注入环境变量, 包括安装脚本环境变量 [optional]
	Envs EnvVar `yaml:"envs"`

	// SecretEnvs 用户注入敏感信息环境变量, value不能在stdout stderr中输出 [optional]
	SecretEnvs EnvVar `yaml:"secret_envs"`

	// Installs 安装程序脚本 [optional]
	Installs []*Install `yaml:"installs"`

	// Repos 用户需要下载的repos
	Repos []*Repo `yaml:"repos"`

	// Scripts 执行主要编译脚本
	Scripts []string `yaml:"scripts"`

	// PostScripts 后置编译脚本
	PostScripts []string `yaml:"post_scripts"`

	// PMDeployScripts 物理机部署脚本
	PMDeployScripts []string `yaml:"pm_deploy_scripts"`

	// SSH ssh连接参数
	SSHs []*SSH `yaml:"sshs"`

	// GinkgoTest 执行 ginkgo test 配置
	GinkgoTest *GinkgoTest `yaml:"ginkgo_test"`

	// Archive: 归档配置 [optional]
	Archive *Archive `yaml:"archive"`

	// DockerRegistry: 镜像仓库配置 [optional]
	DockerRegistry *DockerRegistry `yaml:"docker_registry"`

	// DockerBuildContext image 构建context
	DockerBuildCtx *DockerBuildCtx `yaml:"docker_build_ctx"`

	// FileArchiveCtx 二进制包构建
	FileArchiveCtx *FileArchiveCtx `yaml:"file_archive_ctx"`

	// Git Github/Gitlab 配置
	Git *Git `yaml:"git"`

	// Caches Caches配置
	Caches []string `yaml:"caches"`

	// testType
	TestType string `yaml:"test_type"`

	// Classic build
	ClassicBuild bool `yaml:"classic_build"`

	// StorageURI
	StorageURI string `yaml:"storage_uri"`
	// PipelineName
	PipelineName string `yaml:"pipeline_name"`
	// TaskID
	TaskID int64 `yaml:"task_id"`
	// ServiceName
	ServiceName string `yaml:"service_name"`

	// ResetCache ignore workspace cache [runtime]
	ResetCache bool `yaml:"reset_cache"`

	// IgnoreCache ignore docker build cache [runtime]
	IgnoreCache bool `yaml:"ignore_cache"`

	StorageEndpoint string `yaml:"storage_endpoint"`
	StorageAK       string `yaml:"storage_ak"`
	StorageSK       string `yaml:"storage_sk"`
	StorageBucket   string `yaml:"storage_bucket"`
	StorageProvider int8   `yaml:"storage_provider"`
}

// DockerBuildCtx ...
// Docker build参数
// WorkDir: docker build执行路径
// DockerFile: dockerfile名称, 默认为Dockerfile
// ImageBuild: build image镜像全称, e.g. xxx.com/spock-release-candidates/image:tag
type DockerBuildCtx struct {
	WorkDir         string `yaml:"work_dir" bson:"work_dir" json:"work_dir"`
	DockerFile      string `yaml:"docker_file" bson:"docker_file" json:"docker_file"`
	ImageName       string `yaml:"image_name" bson:"image_name" json:"image_name"`
	BuildArgs       string `yaml:"build_args" bson:"build_args" json:"build_args"`
	ImageReleaseTag string `yaml:"image_release_tag,omitempty" bson:"image_release_tag,omitempty" json:"image_release_tag"`
}

func (c *DockerBuildCtx) GetDockerFile() string {
	if c.DockerFile == "" {
		return "Dockerfile"
	}
	return c.DockerFile
}

type FileArchiveCtx struct {
	FileLocation string `yaml:"file_location" bson:"file_location" json:"file_location"`
	FileName     string `yaml:"file_name" bson:"file_name" json:"file_name"`
	//StorageUri   string `yaml:"storage_uri" bson:"storage_uri" json:"storage_uri"`
}

type SSH struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	UserName   string `json:"user_name"`
	IP         string `json:"ip"`
	IsProd     bool   `json:"is_prod"`
	Label      string `json:"label"`
	PrivateKey string `json:"private_key"`
}

// Proxy 翻墙配置信息
type Proxy struct {
	Type                   string `yaml:"type"`
	Address                string `yaml:"address"`
	Port                   int    `yaml:"port"`
	NeedPassword           bool   `yaml:"need_password"`
	Username               string `yaml:"username"`
	Password               string `yaml:"password"`
	EnableRepoProxy        bool   `yaml:"enable_repo_proxy"`
	EnableApplicationProxy bool   `yaml:"enable_application_proxy"`
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

// EnvVar ...
type EnvVar []string

// Environs 返回用户注入Env 格式为: "key=value".
// 支持 val 包含 $HOME
func (ev EnvVar) Environs() []string {
	resp := []string{}
	for _, val := range ev {
		if val == "" {
			continue
		}

		if len(strings.Split(val, "=")) != 2 {
			continue
		}

		replaced := strings.Replace(val, "$HOME", config.Home(), -1)
		resp = append(resp, replaced)
	}
	return resp
}

// Install ...
type Install struct {
	// 安装名称
	Name string `yaml:"name"`
	// 安装版本
	Version string `yaml:"version"`
	// 安装脚本
	Scripts []string `yaml:"scripts"`
	// 可执行文件目录
	BinPath string `yaml:"bin_path"`
	// Optional: 安装脚本环境变量
	Envs EnvVar `yaml:"envs"`
	// 安装包位置
	Download string `yaml:"download"`
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
}

// PRRef returns refs format
// It will check repo provider type, by default returns github refs format.
//
// e.g. github returns refs/pull/1/head
// e.g. gitlab returns merge-requests/1/head
func (r *Repo) PRRef() string {
	if strings.ToLower(r.Source) == ProviderGitlab || strings.ToLower(r.Source) == ProviderCodehub {
		return fmt.Sprintf("merge-requests/%d/head", r.PR)
	} else if strings.ToLower(r.Source) == ProviderGerrit {
		return r.CheckoutRef
	}
	return fmt.Sprintf("refs/pull/%d/head", r.PR)
}

// BranchRef returns branch refs format
// e.g. refs/heads/master
func (r *Repo) BranchRef() string {
	return fmt.Sprintf("refs/heads/%s", r.Branch)
}

// TagRef returns the tag ref of current repo
// e.g. refs/tags/v1.0.0
func (r *Repo) TagRef() string {
	return fmt.Sprintf("refs/tags/%s", r.Tag)
}

// Ref returns the changes ref of current repo in the following order:
// 1. tag ref
// 2. branch ref
// 3. pr ref
func (r *Repo) Ref() string {
	if len(r.Tag) > 0 {
		return r.TagRef()
	} else if len(r.Branch) > 0 {
		return r.BranchRef()
	} else if r.PR > 0 {
		return r.PRRef()
	}

	return ""
}

// Archive ...
type Archive struct {
	Dir            string `yaml:"dir"`
	File           string `yaml:"file"`
	TestReportFile string `yaml:"test_report_file"`
}

// GinkgoTest ...
type GinkgoTest struct {
	ResultPath     string   `yaml:"result_path"`
	TestReportPath string   `yaml:"test_report_path"`
	ArtifactPaths  []string `yaml:"artifact_paths"`
}

// DockerRegistry 推送镜像到 docker registry 配置
type DockerRegistry struct {
	Host      string `yaml:"host"`
	Namespace string `yaml:"namespace"`
	UserName  string `yaml:"username"`
	Password  string `yaml:"password"`
}

// Git ...
type Git struct {
	UserName string `yaml:"username"`

	Email string `yaml:"email"`

	GithubHost string `yaml:"github_host"`

	GithubSSHKey string `yaml:"github_ssh_key"`

	GitlabHost string `yaml:"gitlab_host"`
	// encoded in base64
	GitlabSSHKey string `yaml:"gitlab_ssh_key"`

	GitKnownHost string `yaml:"git_known_host"`
}

const (
	defaultGitlabHost = "gitlab.koderover.com"
	defaultGithubHost = "github.com"
)

// GetUserName ...
func (g *Git) GetUserName() string {
	if g.UserName == "" {
		return "koderover"
	}
	return g.UserName
}

// GetEmail ...
func (g *Git) GetEmail() string {
	if g.Email == "" {
		return "ci@koderover.com"
	}
	return g.Email
}

// GetGithubHost returns github host
// By default it returns github.com
func (g *Git) GetGithubHost() string {
	if g.GithubHost == "" {
		return defaultGithubHost
	}
	return g.GithubHost
}

// GetGitlabHost returns gitlab host
// By default it returns gitlab.koderover.io
func (g *Git) GetGitlabHost() string {
	if g.GitlabHost == "" {
		return defaultGitlabHost
	}
	return g.GitlabHost
}

// SSHCloneURL returns SSH clone url
func (g *Git) SSHCloneURL(source, owner, name string) string {
	if strings.ToLower(source) == ProviderGitlab {
		return fmt.Sprintf("git@%s:%s/%s.git", g.GetGitlabHost(), owner, name)
	}
	return fmt.Sprintf("git@%s:%s/%s.git", g.GetGithubHost(), owner, name)
}

// HTTPSCloneURL returns HTTPS clone url
func (g *Git) HTTPSCloneURL(source, token, owner, name string) string {
	if strings.ToLower(source) == ProviderGitlab {
		return fmt.Sprintf("https://%s/%s/%s.git", g.GetGitlabHost(), owner, name)
	}
	//return fmt.Sprintf("https://x-access-token:%s@%s/%s/%s.git", g.GetInstallationToken(owner), g.GetGithubHost(), owner, name)
	return fmt.Sprintf("https://x-access-token:%s@%s/%s/%s.git", token, g.GetGithubHost(), owner, name)
}

// SSHCloneURL returns Oauth clone url
// e.g.
//https://oauth2:ACCESS_TOKEN@somegitlab.com/owner/name.git
func (g *Git) OAuthCloneURL(source, token, address, owner, name, scheme string) string {
	if strings.ToLower(source) == ProviderGitlab {
		// address 需要传过来
		return fmt.Sprintf("%s://%s:%s@%s/%s/%s.git", scheme, OauthTokenPrefix, token, address, owner, name)
	}
	//	GITHUB
	return "github"
}

// WriteGithubSSHFile ...
func (g *Git) WriteGithubSSHFile() error {
	if g.GithubSSHKey == "" {
		return nil
	}

	dec, err := base64.StdEncoding.DecodeString(g.GithubSSHKey)
	if err != nil {
		return err
	}
	file := path.Join(config.Home(), "/.ssh/id_rsa.github")
	return ioutil.WriteFile(file, dec, 0400)
}

// WriteGitlabSSHFile ...
func (g *Git) WriteGitlabSSHFile() error {
	if g.GitlabSSHKey == "" {
		return nil
	}

	dec, err := base64.StdEncoding.DecodeString(g.GitlabSSHKey)
	if err != nil {
		return err
	}
	file := path.Join(config.Home(), "/.ssh/id_rsa.gitlab")
	return ioutil.WriteFile(file, dec, 0400)
}

// WriteKnownHostFile ...
func (g *Git) WriteKnownHostFile() error {
	if g.GitKnownHost == "" {
		return nil
	}

	file := path.Join(config.Home(), "/.ssh/known_hosts")
	return ioutil.WriteFile(file, []byte(g.GitKnownHost), 0600)
}

const (
	githubKeyCfg    = "Host %s\nIdentityFile ~/.ssh/id_rsa.github\n"
	gitlabKeyCfg    = "Host %s\nIdentityFile ~/.ssh/id_rsa.gitlab\n"
	hostKeyChecking = "HOST *\nStrictHostKeyChecking=no\nUserKnownHostsFile=/dev/null\n"
	proxyCmd        = "ProxyCommand nc -x %s %%h %%p\n"
)

// WriteSSHConfigFile ...
func (g *Git) WriteSSHConfigFile(proxy *Proxy) error {

	if proxy == nil {
		return nil
	}

	github := fmt.Sprintf(githubKeyCfg, g.GetGithubHost())

	if proxy.EnableRepoProxy && proxy.Type == "socks5" {
		github = github + fmt.Sprintf(proxyCmd, proxy.GetProxyURL())
	}

	gitlab := fmt.Sprintf(gitlabKeyCfg, g.GetGitlabHost())

	out := fmt.Sprintf("%s\n%s\n%s", hostKeyChecking, github, gitlab)

	file := path.Join(config.Home(), "/.ssh/config")
	return ioutil.WriteFile(file, []byte(out), 0600)
}
